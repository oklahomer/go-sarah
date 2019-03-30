package watchers

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"io/ioutil"
	stdLogger "log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := log.GetLogger()
	defer log.SetLogger(oldLogger)

	l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
	logger := log.NewWithStandardLogger(l)
	log.SetLogger(logger)

	code := m.Run()

	os.Exit(code)
}

type dummyInternalWatcher struct {
	AddFunc    func(string) error
	RemoveFunc func(string) error
	CloseFunc  func() error
}

func (w dummyInternalWatcher) Add(dir string) error {
	return w.AddFunc(dir)
}

func (w dummyInternalWatcher) Remove(dir string) error {
	return w.RemoveFunc(dir)
}

func (w dummyInternalWatcher) Close() error {
	return w.CloseFunc()
}

func TestRun(t *testing.T) {
	rootCtx := context.Background()
	watcherCtx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	w, err := Run(watcherCtx)
	if err != nil {
		t.Fatalf("Unexpected error on Run: %s", err.Error())
	}

	if w == nil {
		t.Error("Watcher is not returned")
	}
}

func TestWatcher_Subscribe(t *testing.T) {
	type testData struct {
		group string
		path  string
		fnc   func(string)
		err   error
	}
	tests := []testData{
		{
			group: "dummy",
			path:  "/",
			fnc:   func(_ string) {},
			err:   nil,
		},
		{
			group: "dummy",
			path:  "/",
			fnc:   func(_ string) {},
			err:   errors.New("dummy"),
		},
	}

	for i, test := range tests {
		testNum := i + 1
		w := &watcher{
			subscribeDir: make(chan *subscribeDir),
		}

		go func(d testData) {
			select {
			case subscribe := <-w.subscribeDir:
				if subscribe.initErr == nil {
					t.Fatalf("Channel to notify subscription error is not given on test #%d.", testNum)
				}
				subscribe.initErr <- d.err

				if subscribe.group != d.group {
					t.Errorf("Expected group id is not passed on test #%d: %s.", testNum, subscribe.group)
				}

				if subscribe.dir != d.path {
					t.Errorf("Expected path name is not passed on test #%d: %s.", testNum, subscribe.dir)
				}

				if reflect.ValueOf(subscribe.callback).Pointer() != reflect.ValueOf(d.fnc).Pointer() {
					t.Errorf("Expected function is not passed on test #%d.", testNum)
				}

			case <-time.NewTimer(10 * time.Second).C:
				t.Fatal("Expected subscription request is not passed.")

			}
		}(test)

		err := w.Subscribe(test.group, test.path, test.fnc)

		if test.err != nil {
			if err == nil {
				t.Fatal("Expected error is not returned.")
			}

			if err != test.err {
				t.Fatalf("Unexpected error is returned on test #%d: %s.", testNum, err.Error())
			}
		}
	}
}

func TestWatcher_Unsubscribe(t *testing.T) {
	w := &watcher{
		unsubscribeGroup: make(chan string),
	}

	group := "dummy"
	go func() {
		select {
		case unsubscribe := <-w.unsubscribeGroup:
			if unsubscribe != group {
				t.Errorf("Expected group information is not passed: %s.", unsubscribe)
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Fatal("Expected subscription request is not passed.")

		}
	}()

	err := w.Unsubscribe(group)
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	close(w.unsubscribeGroup)

	err = w.Unsubscribe("dummy")

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if err != ErrWatcherNotRunning {
		t.Errorf("Expected error is not returned after channel close: %s.", err.Error())
	}
}

func TestWatcher_supervise(t *testing.T) {
	added := make(chan struct{}, 1)
	removed := make(chan struct{}, 1)
	internalWatcher := &dummyInternalWatcher{
		AddFunc: func(_ string) error {
			added <- struct{}{}
			return nil
		},
		RemoveFunc: func(_ string) error {
			removed <- struct{}{}
			return nil
		},
		CloseFunc: func() error {
			return nil
		},
	}
	w := &watcher{
		fsWatcher:        internalWatcher,
		subscribeDir:     make(chan *subscribeDir),
		unsubscribeGroup: make(chan string),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventCh := make(chan fsnotify.Event)
	errCh := make(chan error)
	go w.supervise(ctx, eventCh, errCh)

	dirName, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	// Add first subscription
	firstGroup := "first"
	first := &subscribeDir{
		group:    firstGroup,
		dir:      dirName,
		callback: func(string) {},
		initErr:  make(chan error),
	}
	w.subscribeDir <- first
	select {
	case <-added:
		// O.K.

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Watcher.Add is not called.")

	}
	select {
	case err := <-first.initErr:
		if err != nil {
			t.Errorf("Unexpected error is returned: %s.", err.Error())
		}

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Subscription did not respond")

	}

	// Do nothing, but log. Just check this error does not block.
	errCh <- fsnotify.ErrEventOverflow

	// Add second subscription
	secondGroup := "second"
	callbackCalled := make(chan struct{}, 1)
	second := &subscribeDir{
		group: secondGroup,
		dir:   dirName,
		callback: func(_ string) {
			callbackCalled <- struct{}{}
		},
		initErr: make(chan error),
	}

	w.subscribeDir <- second
	select {
	case <-added:
		// O.K.

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Watcher.Add is not called.")

	}
	select {
	case err := <-second.initErr:
		if err != nil {
			t.Errorf("Unexpected error is returned: %s.", err.Error())
		}

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Subscription did not respond")

	}

	// Unsubscribe first group, but the same directory should still subscribed by second group
	w.unsubscribeGroup <- firstGroup

	// The target directory is updated
	eventCh <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: filepath.Join(dirName, "dummy.yml"),
	}
	select {
	case <-callbackCalled:
		// O.K.

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Callback function is not called")

	}

	// Unsubscribe second group, which means all subscriptions are canceled
	w.unsubscribeGroup <- secondGroup

	// The directory is updated, but no active subscription exists
	eventCh <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: filepath.Join(dirName, "dummy.yml"),
	}
	select {
	case <-callbackCalled:
		t.Error("Callback function is unintentionally called.")

	case <-time.NewTimer(100 * time.Millisecond).C:
		// O.K.

	}
}

func TestWatcher_supervise_subscribe_error(t *testing.T) {
	addErr := errors.New("dummy")
	internalWatcher := &dummyInternalWatcher{
		AddFunc: func(_ string) error {
			return addErr
		},
		CloseFunc: func() error {
			return nil
		},
	}
	w := &watcher{
		fsWatcher:        internalWatcher,
		subscribeDir:     make(chan *subscribeDir),
		unsubscribeGroup: make(chan string),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventCh := make(chan fsnotify.Event)
	errCh := make(chan error)
	go w.supervise(ctx, eventCh, errCh)

	dir := &subscribeDir{
		group:    "dummyGroup",
		dir:      "foo",
		callback: func(string) {},
		initErr:  make(chan error),
	}
	w.subscribeDir <- dir

	select {
	case err := <-dir.initErr:
		if err != addErr {
			t.Errorf("Expected error is not returned: %s.", err.Error())
		}

	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Subscription did not respond")

	}
}

func TestWatcher_supervise_stop(t *testing.T) {
	tests := []struct {
		err error
	}{
		{
			err: nil,
		},
		{
			err: errors.New("dummy"),
		},
	}

	for i, test := range tests {
		testNum := i + 1

		closed := make(chan struct{})
		internalWatcher := dummyInternalWatcher{
			CloseFunc: func() error {
				closed <- struct{}{}
				return test.err
			},
		}
		w := &watcher{
			fsWatcher:        internalWatcher,
			unsubscribeGroup: make(chan string),
		}

		ctx, cancel := context.WithCancel(context.Background())
		finished := make(chan struct{})
		go func() {
			w.supervise(ctx, make(chan fsnotify.Event), make(chan error))

			// Comes here when supervise finishes
			// Must finish even if Close returns error
			finished <- struct{}{}
		}()

		cancel()

		select {
		case <-closed:
			// O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Watcher.Close is not called.")

		}

		select {
		case <-finished:
			// O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Errorf("Watcher.supervise did not finish on test #%d.", testNum)

		}
	}
}
