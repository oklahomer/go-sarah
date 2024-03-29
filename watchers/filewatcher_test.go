package watchers

import (
	"context"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := logger.GetLogger()
	defer logger.SetLogger(oldLogger)

	l := log.New(io.Discard, "dummyLog", 0)
	logger.SetLogger(logger.NewWithStandardLogger(l))

	code := m.Run()

	os.Exit(code)
}

type dummyFsWatcher struct {
	AddFunc    func(string) error
	RemoveFunc func(string) error
	CloseFunc  func() error
}

func (w *dummyFsWatcher) Add(dir string) error {
	return w.AddFunc(dir)
}

func (w *dummyFsWatcher) Remove(dir string) error {
	return w.RemoveFunc(dir)
}

func (w *dummyFsWatcher) Close() error {
	return w.CloseFunc()
}

func TestNewFileWatcher(t *testing.T) {
	rootCtx := context.Background()
	watcherCtx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	w, err := NewFileWatcher(watcherCtx, "testdata")

	if err != nil {
		t.Fatalf("Unexpected error on Run: %s", err.Error())
	}

	if w == nil {
		t.Error("ConfigWatcher is not returned")
	}
}

func TestFileWatcher_Read(t *testing.T) {
	tests := []struct {
		id     string
		hasErr bool
	}{
		{
			id: "jsonHello",
		},
		{
			id: "yamlHello",
		},
		{
			id:     "invalid",
			hasErr: true,
		},
	}

	dirName, err := filepath.Abs(filepath.Join("..", "testdata", "config"))
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	var botType sarah.BotType = "dummy"
	type helloConfig struct {
		Text string `json:"text" yaml:"text"`
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			w := &fileWatcher{
				baseDir: dirName,
			}
			configPtr := &helloConfig{}

			err := w.Read(context.TODO(), botType, tt.id, configPtr)

			if tt.hasErr {
				if err == nil {
					t.Error("Expected error is not returned.")
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to read config file: %s.", err.Error())
			}

			if configPtr.Text != "HELLO" {
				t.Error("Configuration file content is not reflected to the struct.")
			}
		})
	}
}

func TestFileWatcher_Watch(t *testing.T) {
	tests := []struct {
		err error
	}{
		{
			err: errors.New("err"),
		},
		{
			err: nil,
		},
	}

	var botType sarah.BotType = "dummy"
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			subscr := make(chan *subscription, 1)
			w := &fileWatcher{
				baseDir:   filepath.Join("path", "to", "dummy", "dir"),
				subscribe: subscr,
			}

			go func(e error) {
				select {
				case dir := <-subscr:
					dir.initErr <- e

				case <-time.NewTimer(100 * time.Millisecond).C:
					t.Error("Expected error value is not passed")

				}
			}(tt.err)

			callback := func() {}
			err := w.Watch(context.TODO(), botType, "hello", callback)

			if tt.err == nil && err != nil {
				t.Errorf("Unexpected error is returned: %s", err.Error())
				return
			}

			if tt.err != err {
				t.Errorf("Expected error is not returned: %s", err)
			}
		})
	}
}

func TestFileWatcher_Unwatch(t *testing.T) {
	tests := []struct {
		panic bool
	}{
		{
			panic: false,
		},
		{
			panic: true,
		},
	}

	var botType sarah.BotType = "dummy"
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			unsubscr := make(chan sarah.BotType, 1)
			w := &fileWatcher{
				unsubscribe: unsubscr,
			}

			if tt.panic {
				// Should be recovered and translated to an error
				close(w.unsubscribe)
			}

			err := w.Unwatch(botType)
			if tt.panic {
				if err != sarah.ErrWatcherNotRunning {
					t.Errorf("Expected error is not returned: %s", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error is returned: %s", err.Error())
			}
		})
	}
}

func Test_doSubscribe(t *testing.T) {
	t.Run("Successful scenario", func(t *testing.T) {
		added := 0
		d := &dummyFsWatcher{
			AddFunc: func(s string) error {
				added++
				return nil
			},
		}

		subscriptions := map[string][]*subscription{}

		t.Run("Initial subscription", func(t *testing.T) {
			dir := filepath.Join("path", "to", "dummy", "dir", "A")
			s := &subscription{
				absDir: dir,
				id:     "id1",
			}

			err := doSubscribe(d, s, subscriptions)

			if err != nil {
				t.Errorf("Unexpected error is returned: %s", err.Error())
			}

			if len(subscriptions) != 1 {
				t.Fatal("Subscription is not started.")
			}

			v, ok := subscriptions[dir]
			if !ok {
				t.Fatal("Subscription is not started.")
			}

			if len(v) != 1 {
				t.Errorf("Unexpected number of subscription is started: %d", len(v))
			}

			if v[0].absDir != dir {
				t.Errorf("Unexpected dir is subscribed: %s", v[0].absDir)
			}

			if added != 1 {
				t.Errorf("Unexpected number of watcher calls: %d", added)
			}
		})

		t.Run("Second subscription", func(t *testing.T) {
			dir := filepath.Join("path", "to", "dummy", "dir", "B")
			s := &subscription{
				absDir: dir,
				id:     "id2",
			}

			err := doSubscribe(d, s, subscriptions)

			if err != nil {
				t.Errorf("Unexpected error is returned: %s", err.Error())
			}

			if len(subscriptions) != 2 {
				t.Fatal("Subscription is not started.")
			}

			v, ok := subscriptions[dir]
			if !ok {
				t.Fatal("Subscription is not started.")
			}

			if len(v) != 1 {
				t.Errorf("Unexpected number of subscription is started: %d", len(v))
			}

			if v[0].absDir != dir {
				t.Errorf("Unexpected dir is subscribed: %s", v[0].absDir)
			}

			if added != 2 {
				t.Errorf("Unexpected number of watcher calls: %d", added)
			}
		})

		t.Run("Third subscription with the same dir as the second one", func(t *testing.T) {
			dir := filepath.Join("path", "to", "dummy", "dir", "B")
			s := &subscription{
				absDir: dir,
				id:     "id3",
			}

			err := doSubscribe(d, s, subscriptions)

			if err != nil {
				t.Errorf("Unexpected error is returned: %s", err.Error())
			}

			if len(subscriptions) != 2 {
				t.Fatal("Subscription is not started.")
			}

			v, ok := subscriptions[dir]
			if !ok {
				t.Fatal("Subscription is not started.")
			}

			// Multiple ids for a single dir
			if len(v) != 2 {
				t.Errorf("Unexpected number of subscription is started: %d", len(v))
			}

			// watcher.Add is not called for the duplicated dir
			if added != 2 {
				t.Errorf("Unexpected number of watcher calls: %d", added)
			}
		})
	})

}

func Test_doUnsubscribe(t *testing.T) {
	var botTypeA sarah.BotType = "dummyBotTypeA"
	var botTypeB sarah.BotType = "dummyBotTypeB"
	dir := filepath.Join("path", "to", "dummy", "dir")
	subscriptions := map[string][]*subscription{
		dir: {
			&subscription{botType: botTypeA, id: "commandA"},
			&subscription{botType: botTypeB, id: "commandB"},
		},
	}

	var removeDir []string
	d := &dummyFsWatcher{
		RemoveFunc: func(s string) error {
			removeDir = append(removeDir, s)
			return nil
		},
	}

	t.Run("Initial unsubscription", func(t *testing.T) {
		doUnsubscribe(d, botTypeA, subscriptions)

		v, ok := subscriptions[dir]
		if !ok {
			t.Fatal("Subscription to the basedir should stay at this point.")
		}

		if len(v) != 1 {
			t.Fatalf("Subscription count should be decreased to 1: %d", len(v))
		}

		if v[0].botType != botTypeB {
			t.Error("The subscription to dummyBotTypeB should stay.")
		}

		if len(removeDir) > 0 {
			t.Errorf("Any directory should not be remved at this point: %d", len(removeDir))
		}
	})

	t.Run("Second unsubscription", func(t *testing.T) {
		doUnsubscribe(d, botTypeB, subscriptions)

		_, ok := subscriptions[dir]
		if ok {
			t.Fatal("All subscription must be gone at this point.")
		}

		if len(removeDir) != 1 {
			t.Errorf("Basedir must be removed at this point.")
		}
	})
}

func TestFileWatcher_run(t *testing.T) {
	dir, _ := filepath.Abs(filepath.Join("..", "testdata", "config", "dummy"))
	invalidDir, _ := filepath.Abs(filepath.Join("..", "testdata", "config", "invalid"))
	validId := "hello"
	subscriptions := []struct {
		absdir     string
		id         string
		watcherErr error
		err        error
		notify     chan struct{}
	}{
		{
			absdir: dir,
			id:     validId,
			notify: make(chan struct{}, 1),
		},
		{
			absdir: dir,
			id:     validId,
			err:    sarah.ErrAlreadySubscribing,
			notify: nil,
		},
		{
			absdir: dir,
			id:     "invalid",
			notify: make(chan struct{}, 1),
		},
		{
			absdir:     invalidDir,
			id:         "error",
			watcherErr: errors.New("error"),
			notify:     nil,
		},
	}
	testCnt := 0
	remove := make(chan string, 2)
	fsWatcher := &dummyFsWatcher{
		AddFunc: func(absDir string) error {
			s := subscriptions[testCnt-1]
			return s.watcherErr
		},
		RemoveFunc: func(absDir string) error {
			remove <- absDir
			return nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	subscr := make(chan *subscription, 1)
	w := &fileWatcher{
		fsWatcher:   fsWatcher,
		subscribe:   subscr,
		unsubscribe: make(chan sarah.BotType, 1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan fsnotify.Event, 1)
	errs := make(chan error, 1)
	go w.run(ctx, events, errs)
	go func() {
		e := errors.New("watcher error")
		for {
			select {
			case <-ctx.Done():
				return

			default:
				// Error should not affect main operation
				select {
				case errs <- e:
					// O.K.

				case <-time.NewTimer(100 * time.Millisecond).C:
					if ctx.Err() == context.Canceled {
						// This is acceptable. errs simply stack because watcher stopped.
						return
					}
					t.Error("Error sending should not block")

				}
			}
		}

	}()

	// Two subscriptions are added for a directory
	var botType sarah.BotType = "dummyBotType"
	for i, s := range subscriptions {
		testCnt++
		copied := s // Not to refer the last element in the loop
		err := make(chan error, 1)
		w.subscribe <- &subscription{
			botType: botType,
			id:      copied.id,
			initErr: err,
			absDir:  copied.absdir,
			callback: func() {
				copied.notify <- struct{}{}
			},
		}

		select {
		case e := <-err:
			if copied.watcherErr != nil {
				if copied.watcherErr != e {
					t.Errorf("Unexpected error state on trial %d: %s", i, e)
				}
			}

			if copied.err != nil {
				if copied.err != e {
					t.Errorf("Unexpected error state on trial %d: %s", i, e)
				}
			}

		case <-time.NewTimer(100 * time.Millisecond).C:
			t.Errorf("Result is not returned.")

		}
	}

	// Valid write event occurs
	events <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: filepath.Join(dir, fmt.Sprintf("%s.json", validId)),
	}
	for _, s := range subscriptions {
		if s.notify == nil {
			continue
		}

		if s.id == validId {
			select {
			case <-s.notify:
				// O.K.

			case <-time.NewTimer(100 * time.Millisecond).C:
				t.Errorf("Event is not notified for %s.", s.id)

			}
		} else {
			select {
			case id := <-s.notify:
				t.Errorf("Unexpected notification is sent: %s.", id)

			case <-time.NewTimer(100 * time.Millisecond).C:
				// O.K.

			}
		}
	}

	// Invalid valid write events occur
	events <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: filepath.Join(dir, "noExtension"),
	}
	events <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: filepath.Join(dir, "noSubscribingDir", "invalid.json"),
	}
	events <- fsnotify.Event{
		Op:   fsnotify.Remove,
		Name: filepath.Join(dir, "noSubscribingDir", "invalid.json"),
	}

	// Unwatch
	w.unsubscribe <- "invalidBotType"
	w.unsubscribe <- botType
	select {
	case d := <-remove:
		if d != dir {
			t.Errorf("Unexpected directory name is given.")
		}

	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Error("Subscription is not removed.")

	}
}

func TestFileWatcher_run_cancel(t *testing.T) {
	tests := []struct {
		err error
	}{
		{
			err: errors.New(""),
		},
		{
			err: nil,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			dummyWatcher := &dummyFsWatcher{
				CloseFunc: func() error {
					return tt.err
				},
			}
			w := &fileWatcher{
				fsWatcher:   dummyWatcher,
				unsubscribe: make(chan sarah.BotType),
			}

			finished := make(chan struct{})
			go func() {
				w.run(ctx, make(chan fsnotify.Event), make(chan error))
				finished <- struct{}{}
			}()

			// Should not cause panic
			cancel()

			select {
			case <-finished:
				// O.K.

			case <-time.NewTimer(100 * time.Millisecond).C:
				t.Error("ConfigWatcher is not stopped.")

			}
		})
	}
}

func Test_plainPathToFile(t *testing.T) {
	tests := []struct {
		path     string
		hasErr   bool
		fileType fileType
	}{
		{
			path:   "/extension/is/empty",
			hasErr: true,
		},
		{
			path:     "/path/to/json/file.json",
			hasErr:   false,
			fileType: jsonFile,
		},
		{
			path:     "/path/to/yaml/file.yml",
			hasErr:   false,
			fileType: yamlFile,
		},
		{
			path:     "/path/to/yaml/file.yaml",
			hasErr:   false,
			fileType: yamlFile,
		},
		{
			path:   "/path/to/yaml/file.html",
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			configFile, err := plainPathToFile(tt.path)
			if tt.hasErr {
				if err == nil {
					t.Fatal("Expected error is not returned.")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error is returned: %s.", err.Error())
			}

			if configFile == nil {
				t.Fatalf("Expected object is not returned.")
			}

			if configFile.fileType != tt.fileType {
				t.Error("Expected fileType is not returned.")
			}
		})
	}

}
