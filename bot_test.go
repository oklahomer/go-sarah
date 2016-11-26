package sarah

import (
	"golang.org/x/net/context"
	"regexp"
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

func Test_newBot(t *testing.T) {
	adapter := &DummyAdapter{}
	myBot := newBot(adapter, "")
	if _, ok := myBot.(*bot); !ok {
		t.Errorf("newBot did not return bot instance: %#v.", myBot)
	}
}

func TestBot_BotType(t *testing.T) {
	var botType BotType = "slack"
	adapter := &DummyAdapter{}
	adapter.BotTypeValue = botType
	bot := newBot(adapter, "")

	if bot.BotType() != botType {
		t.Errorf("Bot type is wrong: %s.", bot.BotType())
	}
}

func TestBot_PluginConfigDir(t *testing.T) {
	dummyPluginDir := "/dummy/path/to/config"
	adapter := &DummyAdapter{}
	bot := newBot(adapter, dummyPluginDir)

	if bot.PluginConfigDir() != dummyPluginDir {
		t.Errorf("Plugin configuration file's location is wrong: %s.", bot.PluginConfigDir())
	}
}

func TestBot_AppendCommand(t *testing.T) {
	adapter := &DummyAdapter{}
	myBot := newBot(adapter, "")

	command := &DummyCommand{}
	myBot.AppendCommand(command)

	registeredCommands := myBot.(*bot).commands
	if len(registeredCommands.cmd) != 1 {
		t.Errorf("1 registered command should exists: %#v.", registeredCommands)
	}
}

func TestBot_Respond(t *testing.T) {
	adapterProcessed := false
	adapter := &DummyAdapter{}
	adapter.SendMessageFunc = func(_ context.Context, _ Output) {
		adapterProcessed = true
	}
	bot := newBot(adapter, "")

	command := &DummyCommand{}
	command.MatchFunc = func(str string) bool {
		return true
	}
	command.ExecuteFunc = func(_ context.Context, input Input) (*CommandResponse, error) {
		return &CommandResponse{Content: regexp.MustCompile(`^\.echo`).ReplaceAllString(input.Message(), "")}, nil
	}
	bot.AppendCommand(command)

	input := &testInput{}
	input.message = ".echo foo"

	err := bot.Respond(context.Background(), input)

	if err != nil {
		t.Errorf("Error on Bot#Respond. %s", err.Error())
	}

	if adapterProcessed == false {
		t.Error("Adapter.SendMessage is not called.")
	}
}

func TestBot_Run(t *testing.T) {
	adapterProcessed := false
	adapter := &DummyAdapter{}
	adapter.RunFunc = func(_ context.Context, _ chan<- Input, _ chan<- error) {
		adapterProcessed = true
	}
	bot := newBot(adapter, "")

	inputReceiver := make(chan Input)
	errCh := make(chan error)
	rootCtx := context.Background()
	botCtx, cancelBot := context.WithCancel(rootCtx)
	defer cancelBot()
	bot.Run(botCtx, inputReceiver, errCh)

	if adapterProcessed == false {
		t.Error("Adapter.Run is not called.")
	}
}

func TestBot_SendMessage(t *testing.T) {
	adapterProcessed := false
	adapter := &DummyAdapter{}
	adapter.SendMessageFunc = func(_ context.Context, _ Output) {
		adapterProcessed = true
	}
	bot := newBot(adapter, "")

	output := NewOutputMessage(struct{}{}, struct{}{})
	ctx := context.Background()
	bot.SendMessage(ctx, output)

	if adapterProcessed == false {
		t.Error("Adapter.SendMessage is not called.")
	}
}
