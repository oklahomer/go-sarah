package watchers

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"path/filepath"
	"sync"
)

// ErrWatcherNotRunning is returned when method is called after Watcher context cancellation.
// This error can safely be ignored when this is returned by Watcher.Unsubscribe
// because the context and all subscriptions are already cancelled.
var ErrWatcherNotRunning = errors.New("context is already canceled")
var mutex sync.Mutex

// Watcher defines an interface that all file system watcher must satisfy.
type Watcher interface {
	Subscribe(string, string, func(string)) error
	Unsubscribe(string) error
}

// internalWatcher defines an interface for fsWatcher.Watcher to ease test.
// This is purely for internal use.
type internalWatcher interface {
	Add(string) error
	Remove(string) error
	Close() error
}

// Run initializes internal watcher and returns Watcher interface.
// When Run is completed, construction is all done and it is safe to call Watcher.Subscribe for additional subscription.
func Run(ctx context.Context) (Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &watcher{
		fsWatcher:        fsWatcher,
		subscribeDir:     make(chan *subscribeDir),
		unsubscribeGroup: make(chan string),
	}

	go w.supervise(ctx, fsWatcher.Events, fsWatcher.Errors)

	return w, nil
}

type watcher struct {
	fsWatcher        internalWatcher
	subscribeDir     chan *subscribeDir
	unsubscribeGroup chan string
}

// Subscribe starts new subscription to file/directory change.
func (w *watcher) Subscribe(group string, path string, callback func(string)) error {
	absDir, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	s := &subscribeDir{
		dir:      absDir,
		callback: callback,
		initErr:  make(chan error, 1),
	}
	w.subscribeDir <- s

	return <-s.initErr
}

// Unsubscribe cancels subscription.
func (w *watcher) Unsubscribe(group string) (err error) {
	defer func() {
		// Panics if and only if unsubscribeGroup channel is closed due to root context cancellation.
		if r := recover(); r != nil {
			err = ErrWatcherNotRunning
		}
	}()

	mutex.Lock()
	defer mutex.Unlock()
	w.unsubscribeGroup <- group

	return nil
}

func (w *watcher) supervise(ctx context.Context, events <-chan fsnotify.Event, errors <-chan error) {
	subscription := map[string][]*subscribeDir{}

	for {
		select {
		case <-ctx.Done():
			err := w.fsWatcher.Close()
			if err == nil {
				log.Info("Stop subscribing to file system event due to context cancel.")
			} else {
				log.Warnf("Error on subscription cancelation: %s.", err.Error())
			}

			// Explicitly close unsubscribeGroup to make sure enqueueing does not block forever, but panics instead.
			// watcher.Unsubscribe MUST recover and return ErrWatcherNotRunning error to caller.
			// BEWARE that group unsubscription and root context cancellation can occur simultaneously.
			mutex.Lock()
			close(w.unsubscribeGroup)
			mutex.Unlock()

			return

		case event := <-events:
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
				log.Infof("Received %s event for %s.", event.Op.String(), event.Name)

				dir, _ := filepath.Split(event.Name)
				absDir, _ := filepath.Abs(dir)

				watches, ok := subscription[absDir]
				if ok {
					for _, watch := range watches {
						watch.callback(event.Name)
					}
				}
			}

		case subscribe := <-w.subscribeDir:
			log.Infof("Start subscribing to %s", subscribe.dir)

			err := w.fsWatcher.Add(subscribe.dir)
			if err != nil {
				subscribe.initErr <- err
				break
			}

			watches, ok := subscription[subscribe.dir]
			if !ok {
				watches = []*subscribeDir{}
			}
			subscription[subscribe.dir] = append(watches, subscribe)
			subscribe.initErr <- nil

		case group := <-w.unsubscribeGroup:
			log.Info("Stop subscription for %s", group)

			for dir, subscribeDirs := range subscription {
				// Exclude all watches that are tied to given group, and stash those should be kept.
				remains := []*subscribeDir{}
				for _, subscribeDir := range subscribeDirs {
					if subscribeDir.group != group {
						remains = append(remains, subscribeDir)
					}
				}

				// If none should remain, stop subscribing to watch corresponding directory.
				if len(remains) == 0 {
					w.fsWatcher.Remove(dir)
					delete(subscription, dir)
					break
				}

				// If any remains, keep subscribing to the directory for remaining callbacks.
				subscription[dir] = remains
			}

		case err := <-errors:
			log.Errorf("Error on subscribing to directory change: %s.", err.Error())

		}
	}
}

type subscribeDir struct {
	group    string
	dir      string
	callback func(string)
	initErr  chan error
}
