package watchers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/log"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// abstractFsWatcher defines an interface to abstract fsnotify.Watcher.
// Its sole purpose is to ease test by replacing fsnotify.Watcher with dummy implementation.
type abstractFsWatcher interface {
	Add(string) error
	Remove(string) error
	Close() error
}

type subscription struct {
	botType  sarah.BotType
	id       string
	absDir   string
	callback func()
	initErr  chan error
}

// NewFileWatcher creates and returns new instance of sarah.ConfigWatcher implementation.
// This subscribes to changes on filesystem.
func NewFileWatcher(ctx context.Context, baseDir string) (sarah.ConfigWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, xerrors.Errorf("failed to start file watcher: %w", err)
	}

	w := &fileWatcher{
		fsWatcher:   fsWatcher,
		subscribe:   make(chan *subscription),
		unsubscribe: make(chan sarah.BotType),
		baseDir:     baseDir,
	}
	go w.run(ctx, fsWatcher.Events, fsWatcher.Errors)

	return w, nil
}

type fileWatcher struct {
	fsWatcher   abstractFsWatcher
	subscribe   chan *subscription
	unsubscribe chan sarah.BotType
	baseDir     string
}

var _ sarah.ConfigWatcher = (*fileWatcher)(nil)

func (w *fileWatcher) Read(ctx context.Context, botType sarah.BotType, id string, configPtr interface{}) error {
	configDir := filepath.Join(w.baseDir, strings.ToLower(botType.String()))
	file := findPluginConfigFile(configDir, id)

	if file == nil {
		return &sarah.ConfigNotFoundError{
			BotType: botType,
			ID:      id,
		}
	}

	buf, err := ioutil.ReadFile(file.absPath)
	if err != nil {
		return xerrors.Errorf("failed to read configuration file at %s: %w", file.absPath, err)
	}

	switch file.fileType {
	case yamlFile:
		return yaml.Unmarshal(buf, configPtr)

	case jsonFile:
		return json.Unmarshal(buf, configPtr)

	default:
		// Should never come. findPluginConfigFile guarantees that.
		return xerrors.Errorf("unsupported file type: %s", file.absPath)

	}
}

func (w *fileWatcher) Watch(_ context.Context, botType sarah.BotType, id string, callback func()) error {
	configDir := filepath.Join(w.baseDir, botType.String())
	absDir, err := filepath.Abs(configDir)
	if err != nil {
		return xerrors.Errorf("failed to construct absolute config absPath for %s: %w", botType, err)
	}

	s := &subscription{
		botType:  botType,
		id:       id,
		absDir:   absDir,
		callback: callback,
		initErr:  make(chan error, 1),
	}
	w.subscribe <- s

	return <-s.initErr
}

func (w *fileWatcher) Unwatch(botType sarah.BotType) (err error) {
	defer func() {
		// Panics if and only if unsubscribeGroup channel is closed due to root context cancellation.
		if r := recover(); r != nil {
			err = sarah.ErrWatcherNotRunning
		}
	}()

	w.unsubscribe <- botType

	return
}

func (w *fileWatcher) run(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error) {
	subscriptions := map[string][]*subscription{}

OP:
	for {
		select {
		case <-ctx.Done():
			err := w.fsWatcher.Close()
			if err == nil {
				log.Info("Stop subscribing to file system event due to context cancel.")
			} else {
				log.Warnf("Error on subscription cancellation: %+v", err)
			}

			// Explicitly close unsubscribeGroup to make sure enqueueing does not block forever, but panics instead.
			// watcher.Unwatch MUST recover and return ErrWatcherNotRunning error to caller.
			// BEWARE that group unsubscription and root context cancellation can occur simultaneously.
			close(w.unsubscribe)

			return

		case event := <-events:
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
				log.Infof("Received %s event for %s.", event.Op.String(), event.Name)

				configFile, err := plainPathToFile(event.Name)
				if xerrors.Is(err, errUnableToDetermineConfigFileFormat) || xerrors.Is(err, errUnsupportedConfigFileFormat) {
					// Irrelevant file is updated
					continue OP
				} else if err != nil {
					log.Warnf("Failed to locate %s: %+v", event.Name, err)
					continue OP
				}

				watches, ok := subscriptions[configFile.absDir]
				if !ok {
					// No corresponding subscription is found for the directory
					continue OP
				}

				// Notify all subscribers
				for _, watch := range watches {
					if watch.id == configFile.id {
						watch.callback()
					}
				}

			default:
				// Do nothing
				log.Debugf("Received %s event for %s.", event.Op.String(), event.Name)

			}

		case subscribe := <-w.subscribe:
			log.Infof("Start subscribing to %s", subscribe.absDir)

			err := w.fsWatcher.Add(subscribe.absDir)
			if err != nil {
				subscribe.initErr <- err
				continue OP
			}

			watches, ok := subscriptions[subscribe.absDir]
			if !ok {
				watches = []*subscription{}
			}
			for _, w := range watches {
				if w.id == subscribe.id {
					subscribe.initErr <- sarah.ErrAlreadySubscribing
					continue OP
				}
			}
			subscriptions[subscribe.absDir] = append(watches, subscribe)
			subscribe.initErr <- nil

		case botType := <-w.unsubscribe:
			log.Infof("Stop subscribing config files for %s", botType)

			for dir, subscribeDirs := range subscriptions {
				// Exclude all watches that are tied to given group, and stash those should be kept.
				var remains []*subscription
				for _, subscribeDir := range subscribeDirs {
					if subscribeDir.botType != botType {
						remains = append(remains, subscribeDir)
					}
				}

				// If none should remain, stop subscribing to watch corresponding directory.
				if len(remains) == 0 {
					_ = w.fsWatcher.Remove(dir)
					delete(subscriptions, dir)
					continue OP
				}

				// If any remains, keep subscribing to the directory for remaining callbacks.
				subscriptions[dir] = remains
			}

		case err := <-errs:
			log.Errorf("Error on subscribing to directory change: %+v", err)

		}
	}
}

type fileType uint

const (
	_ fileType = iota
	yamlFile
	jsonFile
)

var (
	errUnableToDetermineConfigFileFormat = xerrors.New("can not determine file format")
	errUnsupportedConfigFileFormat       = xerrors.New("unsupported file format")
	configFileCandidates                 = []struct {
		ext      string
		fileType fileType
	}{
		{
			ext:      ".yaml",
			fileType: yamlFile,
		},
		{
			ext:      ".yml",
			fileType: yamlFile,
		},
		{
			ext:      ".json",
			fileType: jsonFile,
		},
	}
)

type pluginConfigFile struct {
	id       string
	absPath  string
	absDir   string
	fileType fileType
}

func findPluginConfigFile(configDir, id string) *pluginConfigFile {
	for _, c := range configFileCandidates {
		configPath := filepath.Join(configDir, fmt.Sprintf("%s%s", id, c.ext))
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			continue
		}

		_, err = os.Stat(absPath)
		if err == nil {
			// File exists.
			absDir, _ := filepath.Split(absPath)
			return &pluginConfigFile{
				id:       id,
				absPath:  absPath,
				absDir:   filepath.Dir(absDir), // Handle the trailing slash
				fileType: c.fileType,
			}
		}
	}

	return nil
}

func plainPathToFile(path string) (*pluginConfigFile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, xerrors.Errorf("failed to get absolute path of %s: %w", path, err)
	}

	ext := filepath.Ext(absPath)
	if ext == "" {
		return nil, errUnableToDetermineConfigFileFormat
	}

	absDir, filename := filepath.Split(absPath)
	id := strings.TrimSuffix(filename, ext) // buzz.yaml to buzz

	for _, c := range configFileCandidates {
		if ext != c.ext {
			continue
		}

		return &pluginConfigFile{
			id:       id,
			absPath:  absPath,
			absDir:   filepath.Dir(absDir), // Handle the trailing slash
			fileType: c.fileType,
		}, nil
	}

	return nil, errUnsupportedConfigFileFormat
}
