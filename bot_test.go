package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

var (
	FOO BotType = "foo"
)

type NullAdapter struct {
	botType BotType
}

func (adapter *NullAdapter) BotType() BotType {
	return adapter.botType
}

func (adapter *NullAdapter) Run(_ context.Context, _ chan<- Input, _ chan<- error) {
}

func (adapter *NullAdapter) SendMessage(_ context.Context, _ Output) {
}

func NewNullAdapter() *NullAdapter {
	return NewNullAdapterWithBotType(FOO)
}

func NewNullAdapterWithBotType(botType BotType) *NullAdapter {
	return &NullAdapter{botType: botType}
}

func TestBotType_String(t *testing.T) {
	var BAR BotType = "myNewBotType"
	if BAR.String() != "myNewBotType" {
		t.Errorf("BotType does not return expected value. expected 'myNewBotType', but was '%s'.", BAR.String())
	}
}

func resetStashedBuilder() {
	stashedCommandBuilders = &commandBuilderStash{}
	stashedScheduledTaskBuilders = &scheduledTaskBuilderStash{}
}

type nullCommand struct {
}

func (c *nullCommand) Identifier() string {
	return "fooBarBuzz"
}

func (c *nullCommand) Execute(input Input) (*PluginResponse, error) {
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

func TestNewBotRunner(t *testing.T) {
	runner := NewRunner()

	if runner.bots == nil {
		t.Error("botProperties is nil")
	}

	if runner.worker == nil {
		t.Error("worker is nil")
	}
}

func TestBotRunner_AddAdapter(t *testing.T) {
	adapter := NewNullAdapter()

	runner := NewRunner()
	runner.AddAdapter(adapter, "")

	bot, ok := runner.bots[0].(*bot)
	if !ok {
		t.Fatal("registered bot is not type of default bot")
	}

	if bot.adapter != adapter {
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

	runner := NewRunner()
	runner.AddAdapter(firstAdapter, "")
	runner.AddAdapter(secondAdapter, "")
}

func TestBotRunner_Run_Stop(t *testing.T) {
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	runner := NewRunner()
	runner.Run(runnerCtx)

	time.Sleep(300 * time.Millisecond)
	if runner.worker.IsRunning() == false {
		t.Error("worker is not running")
	}

	cancelRunner()

	time.Sleep(300 * time.Millisecond)
	if runner.worker.IsRunning() == true {
		t.Error("worker is still running")
	}
}

func TestStopUnrecoverableAdapter(t *testing.T) {
	rootCtx := context.Background()
	adapterCtx, cancelAdapter := context.WithCancel(rootCtx)
	errCh := make(chan error)

	go stopUnrecoverableBot(errCh, cancelAdapter)
	if err := adapterCtx.Err(); err != nil {
		t.Error("ctx.Err() should be nil at this point")
		return
	}

	errCh <- NewBotNonContinuableError("")

	time.Sleep(100 * time.Millisecond)
	if err := adapterCtx.Err(); err == nil {
		t.Error("expecting an error at this point")
		return
	}
}
