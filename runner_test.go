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

	if runner == nil {
		t.Fatal("NewRunner reutrned nil.")
	}

	if runner.config != config {
		t.Errorf("Passed config is not set: %#v.", runner.config)
	}

	if runner.bots == nil {
		t.Error("Bot slice is nil.")
	}

	if runner.scheduledTasks == nil {
		t.Error("scheduledTasks are not set.")
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

func TestRunner_RegisterAlerter(t *testing.T) {
	runner := &Runner{}
	runner.alerters = []Alerter{}

	alerter := &DummyAlerter{}
	runner.RegisterAlerter(alerter)

	registeredAlerters := runner.alerters
	if len(registeredAlerters) != 1 {
		t.Fatalf("One and only one alerter should be registered, but actual number was %d.", len(registeredAlerters))
	}

	if registeredAlerters[0] != alerter {
		t.Fatalf("Passed alerter is not registered: %#v.", registeredAlerters[0])
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
	passedCommand := make(chan Command, 1)
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(cmd Command) {
			passedCommand <- cmd
		},
		RunFunc: func(_ context.Context, _ func(Input), _ func(error)) {
			return
		},
	}

	// Configure Runner
	runner := &Runner{
		config: NewConfig(),
		bots:   []Bot{bot},
		scheduledTasks: map[BotType][]ScheduledTask{
			bot.BotType(): []ScheduledTask{
				&DummyScheduledTask{},
				&DummyScheduledTask{ScheduleValue: "@every 1m"},
			},
		},
	}

	// Let it run
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	finished := make(chan bool)
	go func() {
		runner.Run(runnerCtx)
		finished <- true
	}()

	time.Sleep(1 * time.Second)
	cancelRunner()

	select {
	case cmd := <-passedCommand:
		if cmd == nil || cmd.Identifier() != commandBuilder.identifier {
			t.Errorf("Stashed CommandBuilder was not properly configured: %#v.", passedCommand)
		}
	case <-time.NewTicker(10 * time.Second).C:
		t.Fatal("CommandBuilder was not properly built.")
	}

	select {
	case <-finished:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Runner is not finished.")
	}
}

func TestRunner_RegisterScheduledTask(t *testing.T) {
	runner := &Runner{
		scheduledTasks: make(map[BotType][]ScheduledTask),
	}

	task := &DummyScheduledTask{
		IdentifierValue: "foo",
	}

	var botType BotType = "Buzz"
	runner.RegisterScheduledTask(botType, task)
	tasks, ok := runner.scheduledTasks[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(tasks) != 1 && tasks[0] != task {
		t.Errorf("Expected task is not stashed: %#v", tasks)
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

func Test_botSupervisor(t *testing.T) {
	rootCxt := context.Background()
	alerted := make(chan bool)
	alerters := []Alerter{
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, err error) error {
				alerted <- true
				return nil
			},
		},
	}
	botCtx, errSupervisor := botSupervisor(rootCxt, "DummyBotType", alerters)

	select {
	case <-botCtx.Done():
		t.Error("Bot context should not be canceled at this point.")
	default:
		// O.K.
	}

	errSupervisor(NewBotNonContinuableError("should stop"))

	select {
	case <-botCtx.Done():
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Bot context should be canceled at this point.")
	}
	if e := botCtx.Err(); e != context.Canceled {
		t.Errorf("botCtx.Err() must return context.Canceled, but was %#v", e)
	}
	select {
	case <-alerted:
	// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Alert should be sent at this point.")
	}

	nonBlocking := make(chan bool)
	go func() {
		errSupervisor(NewBotNonContinuableError("call after context cancellation should not block"))
		nonBlocking <- true
	}()
	select {
	case <-nonBlocking:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Call after context cancellation blocks.")
	}
}

func Test_setupInputReceiver(t *testing.T) {
	rootCxt := context.Background()
	botCtx, cancelBot := context.WithCancel(rootCxt)
	workerJob := make(chan func(), 1) // Receive the first one and block following inputs.

	responded := make(chan bool, 1)
	bot := &DummyBot{
		BotTypeValue: "DUMMY",
		RespondFunc: func(_ context.Context, input Input) error {
			responded <- true
			return errors.New("error is returned, but still doesn't block")
		},
	}

	receiveInput := setupInputReceiver(botCtx, bot, workerJob)
	time.Sleep(100 * time.Millisecond) // Why is this required...
	receiveInput(&DummyInput{})        // Should be received
	receiveInput(&DummyInput{})        // Channel is blocked, but function does not block

	select {
	case job := <-workerJob:
		job()
		select {
		case <-responded:
			// O.K.
		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Received input was not processed.")
		}

	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Job should be enqueued at this point.")
	}

	cancelBot()
	receiveInput(&DummyInput{}) // Receiving goroutine is canceled, but does not block
}
