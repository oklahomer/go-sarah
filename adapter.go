package sarah

import "golang.org/x/net/context"

/*
Adapter defines interface that each Bot implementation has to satisfy.
Its instance can be fed to Bot to start bot interaction.
*/
type Adapter interface {
	BotType() BotType
	Run(context.Context, chan<- Input, chan<- error)
	SendMessage(context.Context, Output)
}
