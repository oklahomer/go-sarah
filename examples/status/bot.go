package main

import (
	"context"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
)

type nullBot struct {
}

func (*nullBot) BotType() sarah.BotType {
	return "nullBot"
}

func (*nullBot) Respond(context.Context, sarah.Input) error {
	panic("implement me")
}

func (*nullBot) SendMessage(context.Context, sarah.Output) {
	panic("implement me")
}

func (*nullBot) AppendCommand(sarah.Command) {
	panic("implement me")
}

func (*nullBot) Run(ctx context.Context, input func(sarah.Input) error, errNotifier func(error)) {
	<-ctx.Done()
	logger.Info("Stop bot")
}
