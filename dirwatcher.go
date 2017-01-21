package sarah

import (
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"path/filepath"
)

type watchingDir struct {
	botType  BotType
	dir      string
	callback func(string)
	initErr  chan error
}

type dirWatcher struct {
	fsWatcher *fsnotify.Watcher
	watchDir  chan *watchingDir
	cancel    chan BotType
}

func runConfigWatcher(ctx context.Context) (*dirWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dw := &dirWatcher{
		fsWatcher: fsWatcher,
		watchDir:  make(chan *watchingDir),
		cancel:    make(chan BotType),
	}

	go dw.receiveEvent(ctx)

	return dw, nil
}

func (dw *dirWatcher) receiveEvent(ctx context.Context) {
	subscription := map[string][]*watchingDir{}
	for {
		select {
		case <-ctx.Done():
			dw.fsWatcher.Close()
			log.Info("stop subscribing to file system event due to context cancel")
			return

		case event := <-dw.fsWatcher.Events:
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
				log.Infof("%s %s.", event.Op.String(), event.Name)
				dir, _ := filepath.Split(event.Name)
				absDir, _ := filepath.Abs(dir)

				watches, ok := subscription[absDir]
				if ok {
					for _, watch := range watches {
						watch.callback(event.Name)
					}
				}
			}

		case w := <-dw.watchDir:
			err := dw.fsWatcher.Add(w.dir)
			if err != nil {
				w.initErr <- err
				break
			}

			watches, ok := subscription[w.dir]
			if !ok {
				watches = []*watchingDir{}
			}
			subscription[w.dir] = append(watches, w)
			w.initErr <- nil

		case botType := <-dw.cancel:
			for dir, watches := range subscription {
				// Exclude all watches that are tied to given botType, and stash those should be kept.
				remains := []*watchingDir{}
				for _, watch := range watches {
					if watch.botType != botType {
						remains = append(remains, watch)
					}
				}

				// If none should remain, stop subscribing to watch corresponding directory.
				if len(remains) == 0 {
					dw.fsWatcher.Remove(dir)
					delete(subscription, dir)
					break
				}

				// If any remains, keep subscribing to the directory for remaining callbacks.
				subscription[dir] = remains
			}

		case err := <-dw.fsWatcher.Errors:
			log.Errorf("error on subscribing to directory change: %s.", err.Error())

		}
	}
}

func (dw *dirWatcher) watch(botCtx context.Context, botType BotType, path string, callback func(string)) error {
	absDir, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	watchingDir := &watchingDir{
		dir:      absDir,
		botType:  botType,
		callback: callback,
		initErr:  make(chan error, 1),
	}
	dw.watchDir <- watchingDir

	err = <-watchingDir.initErr
	if err != nil {
		return err
	}

	go func() {
		<-botCtx.Done()
		log.Infof("stop directory watch due to context cancel: %s.", botType)
		dw.cancel <- botType
	}()
	return nil
}
