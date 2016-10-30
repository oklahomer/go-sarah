package sarah

import "golang.org/x/net/context"

// Adapter defines interface that each bot adapter implementation has to satisfy.
// Instance of its concrete struct can be fed to Runner via Runner.RegisterAdapter().
// Registered adapter is fed to default bot's constructor and the resulting bot instance is stored and managed by Runner.
//
//  runner := sarah.NewRunner(config)
//  runner.RegisterAdapter(myAdapter, "/plugin/config/dir")
type Adapter interface {
	// BotType represents what this Bot implements. e.g. slack, gitter, cli, etc...
	// This can be used as a unique ID to distinguish one from another.
	BotType() BotType

	// Run is called on Runner.Run by wrapping bot instance.
	// On this call, start interacting with corresponding service provider.
	// This may run in a blocking manner til given context is canceled since a new goroutine is allocated for this task.
	// When the service provider sends message to us, convert that message payload to Input and send to Input channel.
	// Runner will receive the Input instance and proceed to find and execute corresponding command.
	Run(context.Context, chan<- Input, chan<- error)

	// SendMessage sends message to corresponding service provider.
	// This can be called by scheduled task or in response to input from service provider.
	// Be advised: this method may be called simultaneously from multiple workers.
	SendMessage(context.Context, Output)
}
