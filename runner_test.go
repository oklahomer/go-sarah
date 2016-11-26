package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	runner := NewRunner(NewConfig())

	if runner.bots == nil {
		t.Error("BotProperties is nil.")
	}

	if runner.worker == nil {
		t.Error("Worker is nil.")
	}
}

func TestRunner_RegisterBot(t *testing.T) {
	bot := &DummyBot{}

	runner := NewRunner(NewConfig())
	runner.RegisterBot(bot)

	registeredBots := runner.bots
	if len(registeredBots) != 1 {
		t.Fatalf("One and only one bot should be registered, but actual number was %d.", len(registeredBots))
	}

	bot, ok := registeredBots[0].(*DummyBot)
	if !ok {
		t.Fatalf("Registered bot is not type of DummyBot: %#v.", registeredBots[0])
	}
}

func TestRunner_RegisterAdapter(t *testing.T) {
	var botType BotType = "slack"
	adapter := &DummyAdapter{}
	adapter.BotTypeValue = botType

	runner := NewRunner(NewConfig())
	runner.RegisterAdapter(adapter, "")

	bot, ok := runner.bots[0].(*bot)
	if !ok {
		t.Fatal("Registered bot is not type of default bot.")
	}

	if bot.adapter != adapter {
		t.Error("Wrong adapter is stashed.")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic did not occur.")
		}
	}()

	duplicatedAdapter := &DummyAdapter{}
	duplicatedAdapter.BotTypeValue = botType
	runner.RegisterAdapter(duplicatedAdapter, "")
}

func TestRunner_Run(t *testing.T) {
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	runner := NewRunner(NewConfig())
	runner.Run(runnerCtx)

	time.Sleep(300 * time.Millisecond)
	if runner.worker.IsRunning() == false {
		t.Error("Worker is not running.")
	}

	cancelRunner()

	time.Sleep(300 * time.Millisecond)
	if runner.worker.IsRunning() == true {
		t.Error("Worker is still running.")
	}
}

func Test_stopUnrecoverableBot(t *testing.T) {
	rootCtx := context.Background()
	botCtx, cancelBot := context.WithCancel(rootCtx)
	errCh := make(chan error)

	go stopUnrecoverableBot(errCh, cancelBot)
	if err := botCtx.Err(); err != nil {
		t.Fatal("Context.Err() should be nil before error is given.")
	}

	errCh <- NewBotNonContinuableError("")

	time.Sleep(100 * time.Millisecond)
	if err := botCtx.Err(); err == nil {
		t.Fatal("Expecting an error at this point.")
	}
}
