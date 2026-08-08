// Package watchers provides a sarah.ConfigWatcher implementation that subscribes to changes on the filesystem.
package watchers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
	"gopkg.in/yaml.v2"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// abstractFsWatcher defines an interface to abstract fsnotify.Watcher.
// Its sole purpose is to ease the test by replacing fsnotify.Watcher with a dummy implementation.
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

// NewFileWatcher creates and a returns a new instance of sarah.ConfigWatcher implementation.
// This watcher subscribes to changes on the filesystem.
func NewFileWatcher(ctx context.Context, baseDir string) (sarah.ConfigWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to start file watcher: %w", err)
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

func (w *fileWatcher) Read(_ context.Context, botType sarah.BotType, id string, configPtr any) error {
	root, err := os.OpenRoot(w.baseDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &sarah.ConfigNotFoundError{
				BotType: botType,
				ID:      id,
			}
		}
		return fmt.Errorf("failed to open the base directory at %s: %w", w.baseDir, err)
	}
	defer root.Close()

	f, t, err := openPluginConfigFile(root, strings.ToLower(botType.String()), id)
	if err != nil {
		return err
	}
	if f == nil {
		return &sarah.ConfigNotFoundError{
			BotType: botType,
			ID:      id,
		}
	}
	defer f.Close()

	switch t {
	case yamlFile:
		return yaml.NewDecoder(f).Decode(configPtr)

	case jsonFile:
		return json.NewDecoder(f).Decode(configPtr)

	default:
		// Should never come. openPluginConfigFile guarantees that.
		return errUnsupportedConfigFileFormat

	}
}

// openPluginConfigFile looks for a configuration file of the given id in dir, which is relative to root.
// The returned *os.File is nil with no error when no candidate exists.
//
// Because every lookup goes through root, a dir or an id that points outside the watched base directory
// -- with "..", with an absolute path or through a symbolic link -- yields an error instead of an
// unintended file being read.
func openPluginConfigFile(root *os.Root, dir string, id string) (*os.File, fileType, error) {
	for _, c := range configFileCandidates {
		path := filepath.Join(dir, id+c.ext)

		f, err := root.Open(path)
		if err == nil {
			return f, c.fileType, nil
		}

		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		return nil, 0, fmt.Errorf("failed to read configuration file at %s: %w", filepath.Join(root.Name(), path), err)
	}

	return nil, 0, nil
}

func (w *fileWatcher) Watch(_ context.Context, botType sarah.BotType, id string, callback func()) error {
	configDir := filepath.Join(w.baseDir, botType.String())
	absDir, err := filepath.Abs(configDir)
	if err != nil {
		return fmt.Errorf("failed to construct absolute config absPath for %s: %w", botType, err)
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
		// Panics if and only if unsubscribe channel is closed due to the root context cancellation.
		if r := recover(); r != nil {
			err = sarah.ErrWatcherNotRunning
		}
	}()

	w.unsubscribe <- botType

	return
}

func (w *fileWatcher) run(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error) {
	subscriptions := map[string][]*subscription{}

	for {
		select {
		case <-ctx.Done():
			err := w.fsWatcher.Close()
			if err == nil {
				logger.Info("Stop subscribing to file system event due to context cancel.")
			} else {
				logger.Warnf("Error on subscription cancellation: %+v", err)
			}

			// Explicitly close unsubscribeGroup to make sure enqueueing does not block forever, but panics instead.
			// watcher.Unwatch MUST recover and return ErrWatcherNotRunning error to caller.
			// BEWARE that group unsubscription and root context cancellation can occur simultaneously.
			close(w.unsubscribe)

			return

		case event := <-events:
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
				logger.Infof("Received %s event for %s.", event.Op.String(), event.Name)

				doHandleEvent(event, subscriptions)

			default:
				// Do nothing
				logger.Debugf("Received %s event for %s.", event.Op.String(), event.Name)
			}

		case subscribe := <-w.subscribe:
			logger.Infof("Start subscribing to %s", subscribe.absDir)
			err := doSubscribe(w.fsWatcher, subscribe, subscriptions)
			subscribe.initErr <- err // Include nil error

		case botType := <-w.unsubscribe:
			logger.Infof("Stop subscribing config files for %s", botType)
			doUnsubscribe(w.fsWatcher, botType, subscriptions)

		case err := <-errs:
			logger.Errorf("Error on subscribing to directory change: %+v", err)
		}
	}
}

func doHandleEvent(event fsnotify.Event, subscriptions map[string][]*subscription) {
	configFile, err := plainPathToFile(event.Name)
	if errors.Is(err, errUnableToDetermineConfigFileFormat) || errors.Is(err, errUnsupportedConfigFileFormat) {
		// Irrelevant file is updated
		return
	} else if err != nil {
		logger.Warnf("Failed to locate %s: %+v", event.Name, err)
		return
	}

	watches, ok := subscriptions[configFile.absDir]
	if !ok {
		// No corresponding subscription is found for the directory
		return
	}

	// Notify all subscribers
	for _, watch := range watches {
		if watch.id == configFile.id {
			watch.callback()
		}
	}
}

func doSubscribe(a abstractFsWatcher, s *subscription, subscriptions map[string][]*subscription) error {
	watches, ok := subscriptions[s.absDir]
	if !ok {
		// Initial subscription for the given dir
		err := a.Add(s.absDir)
		if err != nil {
			return err
		}

		watches = []*subscription{}
	}
	for _, w := range watches {
		if w.id == s.id {
			return sarah.ErrAlreadySubscribing
		}
	}
	subscriptions[s.absDir] = append(watches, s)
	return nil
}

func doUnsubscribe(a abstractFsWatcher, botType sarah.BotType, subscriptions map[string][]*subscription) {
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
			_ = a.Remove(dir)
			delete(subscriptions, dir)
			return
		}

		// If any remains, keep subscribing to the directory for remaining callbacks.
		subscriptions[dir] = remains
	}
}

type fileType uint

const (
	_ fileType = iota
	yamlFile
	jsonFile
)

var (
	errUnableToDetermineConfigFileFormat = errors.New("can not determine file format")
	errUnsupportedConfigFileFormat       = errors.New("unsupported file format")
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
	absDir   string
	fileType fileType
}

func plainPathToFile(path string) (*pluginConfigFile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of %s: %w", path, err)
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
			absDir:   filepath.Dir(absDir), // Handle the trailing slash
			fileType: c.fileType,
		}, nil
	}

	return nil, errUnsupportedConfigFileFormat
}
