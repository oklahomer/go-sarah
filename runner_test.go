package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

type nullCommand struct {
}

func (c *nullCommand) Identifier() string {
	return "fooBarBuzz"
}

func (c *nullCommand) Execute(input Input) (*PluginResponse, error) {
	return nil, nil
}

func (c *nullCommand) InputExample() string {
	return "dummy"
}

func (c *nullCommand) Match(input string) bool {
	return true
}

func (c *nullCommand) StripCommand(input string) string {
	return input
}

func TestNewBotRunner(t *testing.T) {
	runner := NewRunner(NewConfig())

	if runner.bots == nil {
		t.Error("botProperties is nil")
	}

	if runner.worker == nil {
		t.Error("worker is nil")
	}
}

func TestBotRunner_RegisterAdapter(t *testing.T) {
	adapter := &nullAdapter{}

	runner := NewRunner(NewConfig())
	runner.RegisterAdapter(adapter, "")

	bot, ok := runner.bots[0].(*bot)
	if !ok {
		t.Fatal("registered bot is not type of default bot")
	}

	if bot.adapter != adapter {
		t.Error("wrong adapter is stashed")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic did not occur")
		}
	}()
	runner.RegisterAdapter(&nullAdapter{}, "")
}

func TestBotRunner_Run_Stop(t *testing.T) {
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	runner := NewRunner(NewConfig())
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
		t.Fatal("ctx.Err() should be nil at this point")
	}

	errCh <- NewBotNonContinuableError("")

	time.Sleep(100 * time.Millisecond)
	if err := adapterCtx.Err(); err == nil {
		t.Fatal("expecting an error at this point")
	}
}
