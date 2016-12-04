package sarah

import (
	"github.com/robfig/cron"
	"golang.org/x/net/context"
	"regexp"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	config := NewConfig()
	runner := NewRunner(config)

	if runner.config != config {
		t.Errorf("Passed config is not set: %#v.", runner.config)
	}

	if runner.bots == nil {
		t.Error("Bot slice is nil.")
	}

	if runner.cron == nil {
		t.Error("Default cron instance is nil")
	}
}

func TestRunner_RegisterBot(t *testing.T) {
	runner := &Runner{}
	runner.bots = []Bot{}

	bot := &DummyBot{}
	runner.RegisterBot(bot)

	registeredBots := runner.bots
	if len(registeredBots) != 1 {
		t.Fatalf("One and only one bot should be registered, but actual number was %d.", len(registeredBots))
	}

	if registeredBots[0] != bot {
		t.Fatalf("Passed bot is not registered: %#v.", registeredBots[0])
	}
}

func TestRunner_RegisterAdapter(t *testing.T) {
	var botType BotType = "slack"
	adapter := &DummyAdapter{}
	adapter.BotTypeValue = botType

	runner := &Runner{}
	runner.bots = []Bot{}
	runner.config = NewConfig()
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
	var botType BotType = "myBot"

	// Prepare command to be configured on the fly
	commandBuilder := NewCommandBuilder().
		Identifier("dummy").
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		Func(func(_ context.Context, _ Input) (*CommandResponse, error) {
			return nil, nil
		}).
		InputExample(".echo foo")
	(*stashedCommandBuilders)[botType] = []*CommandBuilder{commandBuilder}

	// Prepare scheduled task to be configured on the fly
	dummyTaskConfig := &DummyScheduledTaskConfig{}
	dummyTaskConfig.ScheduleValue = "0 30 * * * *"
	taskBuilder := NewScheduledTaskBuilder().
		Identifier("dummy").
		ConfigStruct(dummyTaskConfig).
		Func(func(context.Context, ScheduledTaskConfig) (*CommandResponse, error) {
			return nil, nil
		})
	(*stashedScheduledTaskBuilders)[botType] = []*ScheduledTaskBuilder{taskBuilder}

	// Prepare Bot to be run
	bot := &DummyBot{}
	bot.BotTypeValue = botType
	var passedCommand Command
	bot.AppendCommandFunc = func(cmd Command) {
		passedCommand = cmd
	}
	bot.PluginConfigDirFunc = func() string {
		return "testdata/commandBuilder"
	}
	bot.RunFunc = func(_ context.Context, _ chan<- Input, _ chan<- error) {
		return
	}

	// Configure Runner
	runner := &Runner{
		config: NewConfig(),
		bots:   []Bot{},
		cron:   cron.New(),
	}
	runner.bots = []Bot{bot}

	// Let it run
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	defer func() {
		cancelRunner()
	}()
	runner.Run(runnerCtx)

	// Tests follow

	if passedCommand == nil || passedCommand.Identifier() != commandBuilder.identifier {
		t.Errorf("Stashed CommandBuilder was not properly configured: %#v.", passedCommand)
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
