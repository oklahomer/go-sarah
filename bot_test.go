package sarah

import (
	"testing"
	"time"
)

var (
	FOO BotType = "foo"
)

type NullAdapter struct {
	botType BotType
}

func (adapter *NullAdapter) GetBotType() BotType {
	return adapter.botType
}

func (adapter *NullAdapter) Run(_ chan<- BotInput) {
}

func (adapter *NullAdapter) SendMessage(_ BotOutput) {
}

func (adapter *NullAdapter) Stop() {
}

func NewNullAdapter() *NullAdapter {
	return NewNullAdapterWithBotType(FOO)
}

func NewNullAdapterWithBotType(botType BotType) *NullAdapter {
	return &NullAdapter{botType: botType}
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

func (c *nullCommand) Execute(input BotInput) (*PluginResponse, error) {
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
			Func(func(strippedMessage string, input BotInput, _ CommandConfig) (*PluginResponse, error) {
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

func TestNewBotRunner(t *testing.T) {
	runner := NewBotRunner()

	if runner.botProperties == nil {
		t.Error("botProperties is nil")
	}

	if runner.workerPool == nil {
		t.Error("workerPool is nil")
	}

	if runner.stopAll == nil {
		t.Error("stopAll is nil")
	}
}

func TestBotRunner_AddAdapter(t *testing.T) {
	adapter := NewNullAdapter()

	runner := NewBotRunner()
	runner.AddAdapter(adapter, "")

	stashedBotProperties := runner.botProperties[0]

	if stashedBotProperties.adapter != adapter {
		t.Error("wrong adapter is stashed")
	}
}

func TestBotRunner_AddAdapter_DuplicationPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic did not occur")
		}
	}()
	firstAdapter := NewNullAdapter()

	var BAR BotType = "foo" // Same value as default FOO.
	secondAdapter := NewNullAdapterWithBotType(BAR)

	runner := NewBotRunner()
	runner.AddAdapter(firstAdapter, "")
	runner.AddAdapter(secondAdapter, "")
}

func TestBotRunner_Run_Stop(t *testing.T) {
	runner := NewBotRunner()
	runner.Run()

	time.Sleep(300 * time.Millisecond)
	if runner.workerPool.IsRunning() == false {
		t.Error("worker is not running")
	}

	runner.Stop()

	time.Sleep(300 * time.Millisecond)
	if runner.workerPool.IsRunning() == true {
		t.Error("worker is still running")
	}
}
