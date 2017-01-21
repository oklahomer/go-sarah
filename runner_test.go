package sarah

import (
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"regexp"
	"testing"
	"time"
)

func TestNewConfig_UnmarshalNestedYaml(t *testing.T) {
	config := NewConfig()
	oldQueueSize := config.Worker.QueueSize
	oldWorkerNum := config.Worker.WorkerNum
	newWorkerNum := oldWorkerNum + 100

	yamlBytes := []byte(fmt.Sprintf("worker:\n  worker_num: %d", newWorkerNum))

	if err := yaml.Unmarshal(yamlBytes, config); err != nil {
		t.Fatalf("Error on parsing given YAML structure: %s. %s.", string(yamlBytes), err.Error())
	}

	if config.Worker.QueueSize != oldQueueSize {
		t.Errorf("QueueSize should stay when YAML value is not given: %d.", config.Worker.QueueSize)
	}

	if config.Worker.WorkerNum != newWorkerNum {
		t.Errorf("WorkerNum is not overridden with YAML value: %d.", config.Worker.WorkerNum)
	}
}

func TestNewRunner(t *testing.T) {
	config := NewConfig()
	runner := NewRunner(config)

	if runner.config != config {
		t.Errorf("Passed config is not set: %#v.", runner.config)
	}

	if runner.bots == nil {
		t.Error("Bot slice is nil.")
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
	dummySchedule := "@hourly"
	dummyTaskConfig := &DummyScheduledTaskConfig{ScheduleValue: dummySchedule}
	taskBuilder := NewScheduledTaskBuilder().
		Identifier("scheduled").
		ConfigurableFunc(dummyTaskConfig, func(context.Context, TaskConfig) ([]*ScheduledTaskResult, error) {
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
	bot.RunFunc = func(_ context.Context, _ chan<- Input, _ chan<- error) {
		return
	}

	// Configure Runner
	runner := &Runner{
		config: NewConfig(),
		bots:   []Bot{},
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

func Test_setupScheduledTask(t *testing.T) {
	called := false
	scheduler := &DummyScheduler{
		UpdateFunc: func(botType BotType, task ScheduledTask, fn func()) error {
			called = true
			return nil
		},
	}

	task := &scheduledTask{
		identifier: "dummyID",
		taskFunc: func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) {
			return nil, nil
		},
		config: &DummyScheduledTaskConfig{ScheduleValue: "@daily"},
	}
	updateScheduledTask(context.TODO(), &DummyBot{}, scheduler, task)

	if called == false {
		t.Fatal("function is not called")
	}
}

func Test_executeScheduledTask(t *testing.T) {
	dummyContent := "dummy content"
	dummyDestination := "#dummyDestination"
	defaultDestination := "#defaultDestination"
	type returnVal struct {
		results []*ScheduledTaskResult
		error   error
	}
	testSets := []struct {
		returnVal          *returnVal
		defaultDestination OutputDestination
	}{
		{returnVal: &returnVal{nil, nil}},
		{returnVal: &returnVal{nil, errors.New("dummy")}},
		// Destination is given by neither task result nor configuration, which ends up with early return
		{returnVal: &returnVal{[]*ScheduledTaskResult{{Content: dummyContent}}, nil}},
		// Destination is given by configuration
		{returnVal: &returnVal{[]*ScheduledTaskResult{{Content: dummyContent}}, nil}, defaultDestination: defaultDestination},
		// Destination is given by task result
		{returnVal: &returnVal{[]*ScheduledTaskResult{{Content: dummyContent, Destination: dummyDestination}}, nil}},
	}

	var sendingOutput []Output
	dummyBot := &DummyBot{SendMessageFunc: func(_ context.Context, output Output) {
		sendingOutput = append(sendingOutput, output)
	}}

	for _, testSet := range testSets {
		task := &scheduledTask{
			identifier: "dummy",
			taskFunc: func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) {
				val := testSet.returnVal
				return val.results, val.error
			},
			defaultDestination: testSet.defaultDestination,
			config:             &DummyScheduledTaskConfig{},
		}
		executeScheduledTask(context.TODO(), dummyBot, task)
	}

	if len(sendingOutput) != 2 {
		t.Fatalf("Expecting sending method to be called twice, but was called %d time(s).", len(sendingOutput))
	}
	if sendingOutput[0].Content() != dummyContent || sendingOutput[0].Destination() != defaultDestination {
		t.Errorf("Sending output differs from expecting one: %#v.", sendingOutput)
	}
	if sendingOutput[1].Content() != dummyContent || sendingOutput[1].Destination() != dummyDestination {
		t.Errorf("Sending output differs from expecting one: %#v.", sendingOutput)
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

func Test_respond(t *testing.T) {
	isCalled := false
	bot := &DummyBot{}
	bot.RespondFunc = func(_ context.Context, _ Input) error {
		isCalled = true
		return errors.New("just a dummy error instance to check if the method is actually called.")
	}

	inputReceiver := make(chan Input)
	workerJob := make(chan func())

	go respond(context.TODO(), bot, inputReceiver, workerJob)
	inputReceiver <- &DummyInput{}

	select {
	case <-time.NewTimer(1 * time.Second).C:
		t.Error("method did not finish within reasonable timeout.")
	case job := <-workerJob:
		job()
	}

	if isCalled == false {
		t.Error("respond method is not called with supplied input.")
	}
}
