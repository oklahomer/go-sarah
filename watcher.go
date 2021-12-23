package sarah

import (
	"context"
	"errors"
	"fmt"
)

// ErrWatcherNotRunning is returned when ConfigWatcher.Unwatch is called but the context is already canceled.
var ErrWatcherNotRunning = errors.New("context is already canceled")

// ErrAlreadySubscribing is returned when duplicated calls to ConfigWatcher.Watch occur.
var ErrAlreadySubscribing = errors.New("already subscribing")

// ConfigNotFoundError is returned when a corresponding configuration is not found.
// This is typically returned when the caller tries to see if there is any configuration available via ConfigWatcher.Read.
type ConfigNotFoundError struct {
	BotType BotType
	ID      string
}

// Error returns stringified representation of the error.
func (err *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("no configuration found for %s:%s", err.BotType, err.ID)
}

var _ error = (*ConfigNotFoundError)(nil)

// ConfigWatcher defines an interface that all "watcher" implementations must satisfy.
// A watcher subscribes to any change on the configuration setting of Command or ScheduledTask.
// When a change is detected, ConfigWatcher calls the callback function to apply the change to the configuration values Command or ScheduledTask is referring to.
// One example could be watchers.fileWatcher that subscribes to configuration file changes;
// while another reference implementation -- https://github.com/oklahomer/go-sarah-githubconfig -- subscribes to changes on a given GitHub repository.
type ConfigWatcher interface {
	// Read reads the latest configuration value and apply that value to configPtr.
	Read(botCtx context.Context, botType BotType, id string, configPtr interface{}) error
	// Watch subscribes to given id's configuration.
	// When a change to the corresponding configuration value occurs, callback is called.
	// A call to callback function triggers go-sarah's core to call Read() to reflect the latest configuration value.
	Watch(botCtx context.Context, botType BotType, id string, callback func()) error
	// Unwatch is called when Bot is stopped and subscription is no longer required.
	Unwatch(botType BotType) error
}

type nullConfigWatcher struct{}

var _ ConfigWatcher = (*nullConfigWatcher)(nil)

func (*nullConfigWatcher) Read(_ context.Context, _ BotType, _ string, _ interface{}) error {
	return nil
}

func (*nullConfigWatcher) Watch(_ context.Context, _ BotType, _ string, _ func()) error {
	return nil
}

func (*nullConfigWatcher) Unwatch(_ BotType) error {
	return nil
}
