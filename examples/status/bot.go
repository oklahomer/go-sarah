package main

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
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
	log.Info("Stop bot")
}
