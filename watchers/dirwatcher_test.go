package watchers

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	"testing"
	"time"
)

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

func TestWatcher_supervise_Stop(t *testing.T) {
	closed := make(chan struct{})
	internalWatcher := dummyInternalWatcher{
		CloseFunc: func() error {
			closed <- struct{}{}
			return nil
		},
	}
	w := &watcher{
		fsWatcher:        internalWatcher,
		unsubscribeGroup: make(chan string),
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	eventCh := make(chan fsnotify.Event)
	errCh := make(chan error)
	go w.supervise(ctx, eventCh, errCh)

	cancel()

	select {
	case <-closed:
	// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Watcher.Close is not called.")
	}
}

func TestWatcher_supervise_subscription(t *testing.T) {
	added := make(chan struct{})
	removed := make(chan struct{})
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

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	eventCh := make(chan fsnotify.Event)
	errCh := make(chan error)
	go w.supervise(ctx, eventCh, errCh)

	group := "dummy"
	dir := "/"
	callbackCalled := make(chan struct{})
	subscribeErr := make(chan error)
	w.subscribeDir <- &subscribeDir{
		group: group,
		dir:   dir,
		callback: func(_ string) {
			callbackCalled <- struct{}{}
		},
		initErr: subscribeErr,
	}

	select {
	case <-added:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Watcher.Add is not called.")
	}

	select {
	case e := <-subscribeErr:
		if e != nil {
			t.Errorf("Unexpected error on subscription: %s.", e.Error())
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Subscription error is not returned.")
	}

	eventCh <- fsnotify.Event{
		Op:   fsnotify.Write,
		Name: dir,
	}

	select {
	case <-callbackCalled:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Expected callback function is not called.")
	}

	w.unsubscribeGroup <- group

	select {
	case <-removed:
	// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Watcher.Remove is not called.")
	}
}

func TestWatcher_Subscribe_Success(t *testing.T) {
	internalWatcher := &dummyInternalWatcher{
		AddFunc: func(_ string) error {
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

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	go w.supervise(ctx, nil, nil)

	err := w.Subscribe("dummy", "/", func(_ string) {})

	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}
}

func TestWatcher_Subscribe_Fail(t *testing.T) {
	expectedErr := errors.New("expected error")
	internalWatcher := &dummyInternalWatcher{
		AddFunc: func(_ string) error {
			return expectedErr
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

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	go w.supervise(ctx, nil, nil)

	err := w.Subscribe("dummy", "/", func(_ string) {})

	if err != expectedErr {
		t.Fatalf("Expected error is not returned: %s.", err.Error())
	}
}

func TestWatcher_Unsubscribe(t *testing.T) {
	removed := make(chan struct{})
	internalWatcher := &dummyInternalWatcher{
		AddFunc: func(_ string) error {
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

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel() // Is called manually later, but to make sure canceled is when panic arises beforehand.

	go w.supervise(ctx, nil, nil)

	// Multiple subscriptions exist for one directory.
	group := "group1"
	dir := "/"
	w.subscribeDir <- &subscribeDir{
		group:    group,
		dir:      dir,
		callback: func(_ string) {},
		initErr:  make(chan error, 1),
	}
	anotherGroup := "group2"
	w.subscribeDir <- &subscribeDir{
		group:    "another",
		dir:      dir,
		callback: func(_ string) {},
		initErr:  make(chan error, 1),
	}

	err := w.Unsubscribe(group)
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	err = w.Unsubscribe(anotherGroup)
	if err != ErrWatcherNotRunning {
		t.Errorf("Expected error is not returned: %s.", err.Error())
	}
}
