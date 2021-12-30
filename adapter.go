package sarah

import "context"

// Adapter defines an interface that each bot adapter implementation must satisfy.
// An instance of its implementation and DefaultBotOption values can be passed to NewBot to set up a bot.
// The returned bot can be fed to Sarah via RegisterBot to have its life cycle managed.
type Adapter interface {
	// BotType tells what type of chat service this bot is integrating with. e.g. slack, gitter, cli, etc...
	// This can also be used as a unique ID to distinguish one bot from another.
	BotType() BotType

	// Run is called on Sarah's initiation, and the Adapter initiates its interaction with the corresponding chat service.
	// Sarah allocates a new goroutine for this task, so this execution can block until the given context is canceled.
	// When the chat service sends a message, this implementation receives the message, converts into Input, and sends it to the Input channel.
	// Sarah then receives the Input and sees if the input must be applied to the currently cached user context or if there is any matching Command.
	Run(context.Context, func(Input) error, func(error))

	// SendMessage sends the given message to the chat service.
	// Sarah calls this method when ScheduledTask.Execute or Command.Execute returns a non-nil response.
	// This must be capable of being called simultaneously by multiple workers.
	SendMessage(context.Context, Output)
}
