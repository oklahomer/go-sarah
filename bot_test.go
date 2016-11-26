package sarah

import (
	"golang.org/x/net/context"
	"testing"
)

type DummyBot struct {
	BotTypeValue BotType

	RespondFunc func(context.Context, Input) error

	SendMessageFunc func(context.Context, Output)

	AppendCommandFunc func(Command)

	RunFunc func(context.Context, chan<- Input, chan<- error)

	PluginConfigDirFunc func() string
}

func (bot *DummyBot) BotType() BotType {
	return bot.BotTypeValue
}

func (bot *DummyBot) Respond(ctx context.Context, input Input) error {
	return bot.RespondFunc(ctx, input)
}

func (bot *DummyBot) SendMessage(ctx context.Context, output Output) {
	bot.SendMessageFunc(ctx, output)
}

func (bot *DummyBot) AppendCommand(command Command) {
	bot.AppendCommandFunc(command)
}

func (bot *DummyBot) Run(ctx context.Context, input chan<- Input, errCh chan<- error) {
	bot.RunFunc(ctx, input, errCh)
}

func (bot *DummyBot) PluginConfigDir() string {
	return bot.PluginConfigDirFunc()
}

// TODO switch to use DummyAdapter on following commit
const nullBotType BotType = "nullType"

type nullAdapter struct {
}

func (a *nullAdapter) BotType() BotType {
	return nullBotType
}

func (a *nullAdapter) Run(_ context.Context, input chan<- Input, errChan chan<- error) {}

func (a *nullAdapter) SendMessage(_ context.Context, _ Output) {}

func TestNewBot(t *testing.T) {
	adapter := &nullAdapter{}
	myBot := newBot(adapter, "")
	if _, ok := myBot.(*bot); !ok {
		t.Errorf("newBot did not return bot instance. %#v", myBot)
	}
}

func TestBot_BotType(t *testing.T) {
	adapter := &nullAdapter{}
	bot := newBot(adapter, "")

	if bot.BotType() != nullBotType {
		t.Errorf("bot type is wrong %s", bot.BotType())
	}
}

func TestBot_PluginConfigDir(t *testing.T) {
	dummyPluginDir := "/dummy/path/to/config"
	adapter := &nullAdapter{}
	bot := newBot(adapter, dummyPluginDir)

	if bot.PluginConfigDir() != dummyPluginDir {
		t.Errorf("plugin configuration file's location is wrong: %s", bot.PluginConfigDir())
	}
}

func TestBot_AppendCommand(t *testing.T) {
	adapter := &nullAdapter{}
	myBot := newBot(adapter, "")

	command := &abandonedCommand{}
	myBot.AppendCommand(command)

	registeredCommands := myBot.(*bot).commands
	if len(registeredCommands.cmd) != 1 {
		t.Errorf("1 registered command should exists. %#v", registeredCommands)
	}
}

func TestBot_Respond(t *testing.T) {
	adapter := &nullAdapter{}
	bot := newBot(adapter, "")

	command := &echoCommand{}
	bot.AppendCommand(command)

	input := &testInput{}
	input.message = "echo"

	err := bot.Respond(context.Background(), input)

	if err != nil {
		t.Errorf("error on Bot#Respond. %s", err.Error())
	}
}
