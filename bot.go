package sarah

import (
	"golang.org/x/net/context"
	"strings"
	"time"
)

type Bot interface {
	BotType() BotType
	Respond(context.Context, Input) (*PluginResponse, error)
	SendMessage(context.Context, Output)
	AppendCommand(Command)
	Run(context.Context, chan<- Input, chan<- error)
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

func (bot *bot) Respond(ctx context.Context, botInput Input) (*PluginResponse, error) {
	senderKey := botInput.SenderKey()
	userContext := bot.userContextCache.Get(senderKey)

	if userContext == nil {
		return bot.commands.ExecuteFirstMatched(ctx, botInput)
	}

	bot.userContextCache.Delete(senderKey)
	if strings.TrimSpace(botInput.Message()) == ".abort" {
		// abort
		return nil, nil
	}
	fn := userContext.Next
	res, err := fn(ctx, botInput)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Next != nil {
		bot.userContextCache.Set(senderKey, NewUserContext(res.Next))
	}

	return res, err
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
