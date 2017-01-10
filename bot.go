package sarah

import (
	"golang.org/x/net/context"
	"strings"
)

// Bot provides interface for each bot implementation.
// Instance of concrete type can be fed to Runner.AppendBot to have its lifecycle managed by Runner.
// Multiple Bot implementation may be registered to single Runner instance.
type Bot interface {
	// BotType represents what this Bot implements. e.g. slack, gitter, cli, etc...
	// This can be used as a unique ID to distinguish one from another.
	BotType() BotType

	// Respond receives user input, look for corresponding command, execute it, and send result back to user if possible.
	Respond(context.Context, Input) error

	// SendMessage sends message to destination depending on the Bot implementation.
	// This is mainly used to send scheduled task's result.
	// Be advised: this method may be called simultaneously from multiple workers.
	SendMessage(context.Context, Output)

	// AppendCommand appends given Command implementation to Bot internal stash.
	// Stashed commands are checked against user input in Bot.Respond, and if Command.Match returns true, the
	// Command is considered as "corresponds" to the input, hence its Command.Execute is called and the result is
	// sent back to user.
	AppendCommand(Command)

	// Run is called on Runner.Run to let this Bot interact with corresponding service provider.
	// For example, this is where Bot or Bot's corresponding Adapter initiates connection with service provider.
	// This may run in a blocking manner til given context is canceled since a new goroutine is allocated for this task.
	// When the service provider sends message to us, convert that message payload to Input and send to Input channel.
	// Runner will receive the Input instance and proceed to find and execute corresponding command.
	Run(context.Context, chan<- Input, chan<- error)
}

type defaultBot struct {
	botType          BotType
	runFunc          func(context.Context, chan<- Input, chan<- error)
	sendMessageFunc  func(context.Context, Output)
	commands         *Commands
	userContextCache UserContexts
}

func NewBot(adapter Adapter, cacheConfig *CacheConfig) Bot {
	return &defaultBot{
		botType:          adapter.BotType(),
		runFunc:          adapter.Run,
		sendMessageFunc:  adapter.SendMessage,
		commands:         NewCommands(),
		userContextCache: NewCachedUserContexts(cacheConfig),
	}
}

func (bot *defaultBot) BotType() BotType {
	return bot.botType
}

func (bot *defaultBot) Respond(ctx context.Context, input Input) error {
	senderKey := input.SenderKey()
	userContext, cacheErr := bot.userContextCache.Get(senderKey)
	if cacheErr != nil {
		return cacheErr
	}

	var res *CommandResponse
	var err error
	if userContext == nil {
		res, err = bot.commands.ExecuteFirstMatched(ctx, input)
	} else {
		bot.userContextCache.Delete(senderKey)
		if strings.TrimSpace(input.Message()) == ".abort" {
			// abort
			return nil
		}
		res, err = (userContext.Next)(ctx, input)
	}

	if err != nil {
		return err
	}

	if res == nil {
		return nil
	}

	if res.Next != nil {
		bot.userContextCache.Set(senderKey, NewUserContext(res.Next))
	}

	message := NewOutputMessage(input.ReplyTo(), res.Content)
	bot.SendMessage(ctx, message)

	return nil
}

func (bot *defaultBot) SendMessage(ctx context.Context, output Output) {
	bot.sendMessageFunc(ctx, output)
}

func (bot *defaultBot) AppendCommand(command Command) {
	bot.commands.Append(command)
}

func (bot *defaultBot) Run(ctx context.Context, receivedInput chan<- Input, errCh chan<- error) {
	bot.runFunc(ctx, receivedInput, errCh)
}
