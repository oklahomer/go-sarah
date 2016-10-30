package sarah

import (
	"golang.org/x/net/context"
	"strings"
	"time"
)

// Bot provides interface for each bot implementation.
// Instance of concrete type can be fed to Runner.AppendBot to have its lifecycle managed by Runner.
// Multiple Bot implementation may be registered to single Runner instance.
type Bot interface {
	// BotType represents what this Bot implements. e.g. slack, gitter, cli, etc...
	// This can be used as a unique ID to distinguish one from another.
	BotType() BotType

	// Respond receives user input, look for corresponding command, execute it, and return response if possible.
	// The returned *PluginResponse can be converted to Output and then passed to SendMessage.
	Respond(context.Context, Input) (*PluginResponse, error)

	// SendMessage sends message to destination depending on the Bot implementation.
	// This is mainly used to send Bot.Respond's response or scheduled task's result.
	// Be advised: this method may be called simultaneously from multiple workers.
	SendMessage(context.Context, Output)

	// AppendCommand appends given Command implementation to Bot internal stash.
	// Stashed commands are checked against user input in Bot.Respond, and if Command.Match returns true, the
	// Command is considered to "corresponds" to the input, hence its Command.Execute is called and the result is
	// returned by Command.Respond to be fed to Bot.SendMessage.
	AppendCommand(Command)

	// Run is called on Runner.Run to let this Bot interact with corresponding service provider.
	// For example, this is where Bot or Bot's corresponding Adapter initiates connection with service provider.
	// This may run in a blocking manner til given context is canceled since a new goroutine is allocated for this task.
	// When the service provider sends message to us, convert that message payload to Input and send to Input channel.
	// Runner will receive the Input instance and proceed to find and execute corresponding command.
	Run(context.Context, chan<- Input, chan<- error)

	// PluginConfigDir returns where each Command's configuration file is located.
	// Files must be YAML formatted with .yaml postfix.
	PluginConfigDir() string
}

type bot struct {
	adapter          Adapter
	commands         *Commands
	userContextCache *CachedUserContexts
	pluginConfigDir  string
}

func newBot(adapter Adapter, configDir string) Bot {
	return &bot{
		adapter:          adapter,
		commands:         NewCommands(),
		userContextCache: NewCachedUserContexts(3*time.Minute, 10*time.Minute),
		pluginConfigDir:  configDir,
	}
}

func (bot *bot) BotType() BotType {
	return bot.adapter.BotType()
}

func (bot *bot) Respond(ctx context.Context, input Input) (res *PluginResponse, err error) {
	senderKey := input.SenderKey()
	userContext := bot.userContextCache.Get(senderKey)

	if userContext == nil {
		res, err = bot.commands.ExecuteFirstMatched(ctx, input)
	} else {
		bot.userContextCache.Delete(senderKey)
		if strings.TrimSpace(input.Message()) == ".abort" {
			// abort
			return
		}
		res, err = (userContext.Next)(ctx, input)
	}

	if err != nil {
		return
	}

	if res != nil && res.Next != nil {
		bot.userContextCache.Set(senderKey, NewUserContext(res.Next))
	}

	return
}

func (bot *bot) SendMessage(ctx context.Context, output Output) {
	bot.adapter.SendMessage(ctx, output)
}

func (bot *bot) AppendCommand(command Command) {
	bot.commands.Append(command)
}

func (bot *bot) Run(ctx context.Context, receivedInput chan<- Input, errCh chan<- error) {
	bot.adapter.Run(ctx, receivedInput, errCh)
}

func (bot *bot) PluginConfigDir() string {
	return bot.pluginConfigDir
}
