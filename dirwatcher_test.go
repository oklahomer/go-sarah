package sarah

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type DummyWatcher struct {
	addFunc    func(string) error
	removeFunc func(string) error
	closeFunc  func() error
}

func (w *DummyWatcher) Add(dir string) error {
	return w.addFunc(dir)
}

func (w *DummyWatcher) Remove(dir string) error {
	return w.removeFunc(dir)
}

func (w *DummyWatcher) Close() error {
	return w.closeFunc()
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
	dir, err := filepath.Abs("dummy")
	if err != nil {
		t.Fatalf("Unexpected error on path string generation: %s.", err.Error())
	}

	watcher := &DummyWatcher{}
	target := make(chan *watchingDir, 1)
	cancelWatch := make(chan BotType, 1)
	dw := &dirWatcher{
		watcher:  watcher,
		watchDir: target,
		cancel:   cancelWatch,
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
				t.Error("Unexpected callback function is given")
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
	dir, err := filepath.Abs("dummy")
	if err != nil {
		t.Fatalf("Unexpected error on path string generation: %s.", err.Error())
	}

	watcher := &DummyWatcher{}
	target := make(chan *watchingDir, 1)
	cancelWatch := make(chan BotType, 1)
	dw := &dirWatcher{
		watcher:  watcher,
		watchDir: target,
		cancel:   cancelWatch,
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

func TestDirWatcher_receiveEvent_watchFailure(t *testing.T) {
	watchErr := errors.New("")
	watcher := &DummyWatcher{
		addFunc: func(_ string) error {
			return watchErr
		},
		closeFunc: func() error {
			return nil
		},
	}
	dw := &dirWatcher{
		watcher:  watcher,
		watchDir: make(chan *watchingDir, 1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dw.receiveEvent(ctx, make(chan fsnotify.Event, 1), make(chan error, 1))

	target := &watchingDir{
		dir:      "dummy",
		botType:  "DummyBot",
		callback: func(path string) {},
		initErr:  make(chan error, 1),
	}
	dw.watchDir <- target
	select {
	case initErr := <-target.initErr:
		if initErr != watchErr {
			t.Fatalf("Unexpected error is returned: %s.", initErr.Error())
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Directory addition did not complete in time.")
	}
}

func TestDirWatcher_receiveEvent_Events(t *testing.T) {
	watcher := &DummyWatcher{
		addFunc: func(_ string) error {
			return nil
		},
		closeFunc: func() error {
			return nil
		},
	}
	watchTarget := make(chan *watchingDir, 1)
	cancelWatch := make(chan BotType, 1)
	dw := &dirWatcher{
		watcher:  watcher,
		watchDir: watchTarget,
		cancel:   cancelWatch,
	}

	// Start receiving events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventChan := make(chan fsnotify.Event, 1)
	errorChan := make(chan error, 1)
	go dw.receiveEvent(ctx, eventChan, errorChan)

	// Let receiveEvent stash directory information internally.
	var botType BotType = "Foo"
	callbackPath := make(chan string, 1)
	configDir, err := filepath.Abs(filepath.Join("dummy", strings.ToLower(botType.String())))
	if err != nil {
		t.Fatalf("Unexpected error on path string generation: %s.", err.Error())
	}
	watch := &watchingDir{
		dir:     configDir,
		botType: botType,
		callback: func(path string) {
			callbackPath <- path
		},
		initErr: make(chan error, 1),
	}
	watchingDir := watch
	dw.watchDir <- watchingDir
	select {
	case initErr := <-watch.initErr:
		if initErr != nil {
			t.Fatalf("Unexpected error is returned: %s.", initErr.Error())
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Directory addition did not complete in time.")
	}

	// Event is sent for the stashed directory
	createdFile, err := filepath.Abs(filepath.Join(watch.dir, "newFile"))
	if err != nil {
		t.Fatalf("Unexpected error on path string generation: %s.", err.Error())
	}
	event := fsnotify.Event{
		Name: createdFile,
		Op:   fsnotify.Create,
	}
	eventChan <- event

	select {
	case path := <-callbackPath:
		if filepath.Dir(path) != watch.dir {
			t.Errorf("Expected %s, but was %s.", watch.dir, path)
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Callback function is not called.")
	}
}

func TestDirWatcher_receiveEvent_cancel(t *testing.T) {
	watcher := &DummyWatcher{
		addFunc: func(_ string) error {
			return nil
		},
		closeFunc: func() error {
			return nil
		},
		removeFunc: func(_ string) error {
			return nil
		},
	}

	// Start receiving events
	dw := &dirWatcher{
		watcher:  watcher,
		watchDir: make(chan *watchingDir, 1),
		cancel:   make(chan BotType, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventChan := make(chan fsnotify.Event, 1)
	errorChan := make(chan error, 1)
	go dw.receiveEvent(ctx, eventChan, errorChan)

	// Let receiveEvent stash directory information internally.
	var botType BotType = "Foo"
	callbackPath := make(chan string, 1)
	configDir, err := filepath.Abs(filepath.Join("dummy", strings.ToLower(botType.String())))
	if err != nil {
		t.Fatalf("Unexpected error on path string generation: %s.", err.Error())
	}
	watch := &watchingDir{
		dir:     configDir,
		botType: botType,
		callback: func(path string) {
			callbackPath <- path
		},
		initErr: make(chan error, 1),
	}
	dw.watchDir <- watch
	select {
	case <-watch.initErr:
		// no-opp
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("Directory addition did not complete in time.")
	}

	// Do the cancellation
	dw.cancel <- botType

	// Nothing bad happens
}
