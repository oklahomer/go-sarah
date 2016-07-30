package sarah

import (
	"testing"
	"time"
)

var (
	FOO BotType = "foo"
)

type NullAdapter struct {
}

func (adapter *NullAdapter) GetPluginConfigDir() string {
	return ""
}

func (adapter *NullAdapter) GetBotType() BotType {
	return FOO
}

func (adapter *NullAdapter) Run(receiver chan<- BotInput) {
}

func (adapter *NullAdapter) SendResponse(response *CommandResponse) {
}

func (adapter *NullAdapter) Stop() {
}

func NewNullAdapter() *NullAdapter {
	return &NullAdapter{}
}

func TestBotType(t *testing.T) {
	if FOO.String() != "foo" {
		t.Errorf("BotTYpe does not return expected value. expected 'foo', but was '%s'.", FOO.String())
	}
}

func resetStashedBuilder() {
	stashedCommandBuilder = map[BotType][]*commandBuilder{}
}

type nullCommand struct {
}

func (c *nullCommand) Identifier() string {
	return "fooBarBuzz"
}

func (c *nullCommand) Execute(input BotInput) (*CommandResponse, error) {
	return nil, nil
}

func (c *nullCommand) Example() string {
	return "dummy"
}

func (c *nullCommand) Match(input string) bool {
	return true
}

func (c *nullCommand) StripCommand(input string) string {
	return input
}

func TestAppendCommandBuilder(t *testing.T) {
	resetStashedBuilder()
	commandBuilder :=
		NewCommandBuilder().
			ConfigStruct(NullConfig).
			Identifier("fooCommand").
			Example("example text").
			Func(func(strippedMessage string, input BotInput, _ CommandConfig) (*CommandResponse, error) {
				return nil, nil
			})
	AppendCommandBuilder(FOO, commandBuilder)

	stashedBuilders := stashedCommandBuilder[FOO]
	if size := len(stashedBuilders); size != 1 {
		t.Errorf("1 commandBuilder was expected to be stashed, but was %d", size)
	}
	if builder := stashedBuilders[0]; builder != commandBuilder {
		t.Errorf("stashed commandBuilder is somewhat different. %#v", builder)
	}
}

func TestNewBot(t *testing.T) {
	bot := NewBot()

	if bot.adapters == nil {
		t.Error("adapters is nil")
	}

	if bot.commands == nil {
		t.Error("commands is nil")
	}

	if bot.workerPool == nil {
		t.Error("workerPool is nil")
	}

	if bot.stopAll == nil {
		t.Error("stopAll is nil")
	}
}

func TestBot_AddAdapter(t *testing.T) {
	adapter := NewNullAdapter()

	bot := NewBot()
	bot.AddAdapter(adapter)

	stashedAdapter, ok := bot.adapters[FOO]
	if !ok {
		t.Error("given adapter is not stashed")
		return
	}

	if stashedAdapter != adapter {
		t.Error("wrong adapter is stashed")
	}
}

func TestBot_RunStop(t *testing.T) {
	bot := NewBot()
	bot.Run()

	time.Sleep(300 * time.Millisecond)
	if bot.workerPool.IsRunning() == false {
		t.Error("worker is not running")
	}

	bot.Stop()

	time.Sleep(300 * time.Millisecond)
	if bot.workerPool.IsRunning() == true {
		t.Error("worker is still running")
	}
}
