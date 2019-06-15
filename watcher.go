package sarah

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
)

var ErrWatcherNotRunning = xerrors.New("context is already canceled")

var ErrAlreadySubscribing = xerrors.New("already subscribing")

type ConfigNotFoundError struct {
	BotType BotType
	ID      string
}

func (err *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("no configuration found for %s:%s", err.BotType, err.ID)
}

var _ error = (*ConfigNotFoundError)(nil)

type ConfigWatcher interface {
	Read(botCtx context.Context, botType BotType, id string, configPtr interface{}) error
	Subscribe(botCtx context.Context, botType BotType, id string, callback func()) error
	Unsubscribe(botType BotType) error
}

type nullConfigWatcher struct{}

var _ ConfigWatcher = (*nullConfigWatcher)(nil)

func (*nullConfigWatcher) Read(_ context.Context, _ BotType, _ string, _ interface{}) error {
	return nil
}

func (*nullConfigWatcher) Subscribe(_ context.Context, _ BotType, _ string, _ func()) error {
	return nil
}

func (*nullConfigWatcher) Unsubscribe(_ BotType) error {
	return nil
}
