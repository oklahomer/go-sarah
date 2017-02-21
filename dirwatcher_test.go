package sarah

import (
	"context"
	"errors"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

func setupTmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "dirwatcher")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %s.", err.Error())
	}

	return dir
}

func Test_runConfigWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dw, err := runConfigWatcher(ctx)
	if err != nil {
		t.Fatalf("Error is returned: %s.", err.Error())
	}
	if dw == nil {
		t.Fatal("Expected dirWatcher instance is not returned.")
	}
}

func TestDirWatcher_watch(t *testing.T) {
	dir := setupTmpDir(t)
	defer func() {
		os.RemoveAll(dir)
	}()

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	target := make(chan *watchingDir, 1)
	cancelWatch := make(chan BotType, 1)
	dw := &dirWatcher{
		fsWatcher: fsWatcher,
		watchDir:  target,
		cancel:    cancelWatch,
	}

	var botType BotType = "Foo"
	callback := func(_ string) {}
	go func() {
		select {
		case d := <-dw.watchDir:
			if d.botType != botType {
				t.Errorf("Unexpected BotType is given: %s.", d.botType.String())
			}
			if d.dir != dir {
				t.Errorf("Unexpected directory is given: %s.", d.dir)
			}
			if reflect.ValueOf(d.callback).Pointer() != reflect.ValueOf(callback).Pointer() {
				t.Errorf("Unexpected callback function is given: %#v.", d.callback)
			}
			d.initErr <- nil
			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	err = dw.watch(ctx, botType, dir, callback)
	if err != nil {
		cancel()
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	cancel()
	select {
	case canceledBotType := <-cancelWatch:
		if canceledBotType != botType {
			t.Errorf("Unexpected BotType is passed: %s.", canceledBotType.String())
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Context cancellation is not propagated.")
	}
}

func TestDirWatcher_watch_InitError(t *testing.T) {
	dir := setupTmpDir(t)
	defer func() {
		os.RemoveAll(dir)
	}()

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	target := make(chan *watchingDir, 1)
	dw := &dirWatcher{
		fsWatcher: fsWatcher,
		watchDir:  target,
	}

	initErr := errors.New("")
	go func() {
		select {
		case d := <-dw.watchDir:
			d.initErr <- initErr
			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = dw.watch(ctx, "Foo", dir, func(_ string) {})
	if err == nil {
		t.Fatal("Expected error is not returned.")
	}
}

func TestDirWatcher_receiveEvent_Events(t *testing.T) {
	dir := setupTmpDir(t)
	defer func() {
		os.RemoveAll(dir)
	}()

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	target := make(chan *watchingDir, 1)
	cancelWatch := make(chan BotType, 1)
	dw := &dirWatcher{
		fsWatcher: fsWatcher,
		watchDir:  target,
		cancel:    cancelWatch,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dw.receiveEvent(ctx)

	var botType BotType = "Foo"
	callbackPath := make(chan string, 1)
	watchingDir := &watchingDir{
		dir:     dir,
		botType: botType,
		callback: func(path string) {
			callbackPath <- path
		},
		initErr: make(chan error, 1),
	}
	select {
	case dw.watchDir <- watchingDir:
		// ok
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Can not enqueue watching directory.")
	}

	file, err := ioutil.TempFile(dir, "Foo")
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
	file.WriteString("Dummy")
	file.Sync()
	file.Close()
	ioutil.WriteFile(file.Name(), []byte(""), 0600) // Hmmm...
	select {
	case d := <-callbackPath:
		if d != file.Name() {
			t.Errorf("Expected %s, but was %s.", file.Name(), d)
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Callback function is not called.")
	}
}
