package sarah

import (
	"golang.org/x/net/context"
	"testing"
)

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

	response, err := bot.Respond(context.Background(), input)

	if err != nil {
		t.Errorf("error on Bot#Respond. %s", err.Error())
	}

	if response.Content != input.Message() {
		t.Errorf("unexpected response content. %#v", response.Content)
	}
}
