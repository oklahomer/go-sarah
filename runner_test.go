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

func TestNewRunnerOptions(t *testing.T) {
	options := NewRunnerOptions()

	if len(*options) != 0 {
		t.Errorf("Size of stashed options should be 0 at first, but was %d.", len(*options))
	}
}

func TestRunnerOptions_Append(t *testing.T) {
	options := &RunnerOptions{}
	options.Append(func(_ *Runner) error { return nil })

	if len(*options) != 1 {
		t.Errorf("Size of stashed options should be 0 at first, but was %d.", len(*options))
	}
}

func TestRunnerOptions_Arg(t *testing.T) {
	options := &RunnerOptions{}
	calledCnt := 0
	*options = append(
		*options,
		func(_ *Runner) error {
			calledCnt++
			return nil
		},
		func(_ *Runner) error {
			calledCnt++
			return nil
		},
	)

	options.Arg()(&Runner{})

	if calledCnt != 2 {
		t.Fatalf("Options are not properly called. Count: %d.", calledCnt)
	}
}

func TestNewRunner_WithoutRunnerOption(t *testing.T) {
	config := NewConfig()
	runner, err := NewRunner(config)

	if err != nil {
		t.Fatalf("Unexpected error: %#v.", err)
	}

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

func TestNewRunner_WithRunnerOption(t *testing.T) {
	called := false
	config := NewConfig()
	runner, err := NewRunner(
		config,
		func(_ *Runner) error {
			called = true
			return nil
		},
	)

	if runner == nil {
		t.Error("Runner instance should be returned.")
	}

	if called == false {
		t.Error("RunnerOption is not called.")
	}

	if err != nil {
		t.Errorf("Unexpected error is returned: %#v.", err)
	}
}

func TestNewRunner_WithRunnerFatalOptions(t *testing.T) {
	called := false
	optErr := errors.New("Second RunnerOption returns this error.")
	config := NewConfig()
	runner, err := NewRunner(
		config,
		func(_ *Runner) error {
			called = true
			return nil
		},
		func(_ *Runner) error {
			return optErr
		},
	)

	if runner != nil {
		t.Error("Runner instance should not be returned on error.")
	}

	if called == false {
		t.Error("RunnerOption is not called.")
	}

	if err == nil {
		t.Error("Error should be returned.")
	}

	if err != optErr {
		t.Errorf("Expected error is not returned: %#v.", err)
	}
}

func TestWithBot(t *testing.T) {
	bot := &DummyBot{}
	runner := &Runner{
		bots: []Bot{},
	}

	WithBot(bot)(runner)

	registeredBots := runner.bots
	if len(registeredBots) != 1 {
		t.Fatalf("One and only one bot should be registered, but actual number was %d.", len(registeredBots))
	}

	if registeredBots[0] != bot {
		t.Fatalf("Passed bot is not registered: %#v.", registeredBots[0])
	}
}

func TestWithCommandProps(t *testing.T) {
	var botType BotType = "dummy"
	props := &CommandProps{
		botType: botType,
	}
	runner := &Runner{
		cmdProps: make(map[BotType][]*CommandProps),
	}

	WithCommandProps(props)(runner)

	botCmdProps, ok := runner.cmdProps[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(botCmdProps) != 1 && botCmdProps[0] != props {
		t.Error("Expected CommandProps is not stashed.")
	}
}

func TestWithScheduledTask(t *testing.T) {
	var botType BotType = "dummy"
	task := &DummyScheduledTask{}
	runner := &Runner{
		scheduledTasks: make(map[BotType][]ScheduledTask),
	}

	WithScheduledTask(botType, task)(runner)

	tasks, ok := runner.scheduledTasks[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(tasks) != 1 && tasks[0] != task {
		t.Errorf("Expected task is not stashed: %#v", tasks)
	}
}

func TestWithAlerter(t *testing.T) {
	alerter := &DummyAlerter{}
	runner := &Runner{
		alerters: &alerters{},
	}

	WithAlerter(alerter)(runner)

	registeredAlerters := runner.alerters
	if len(*registeredAlerters) != 1 {
		t.Fatalf("One and only one alerter should be registered, but actual number was %d.", len(*registeredAlerters))
	}

	if (*registeredAlerters)[0] != alerter {
		t.Fatalf("Passed alerter is not registered: %#v.", (*registeredAlerters)[0])
	}
}
func TestRunner_Run(t *testing.T) {
	var botType BotType = "myBot"

	// Prepare Bot to be run
	passedCommand := make(chan Command, 1)
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(cmd Command) {
			passedCommand <- cmd
		},
		RunFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {
			return
		},
	}

	// Prepare command to be configured on the fly
	commandProps := &CommandProps{
		botType:      botType,
		identifier:   "dummy",
		matchPattern: regexp.MustCompile(`^\.echo`),
		commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
			return nil, nil
		},
		example: ".echo foo",
	}

	// Prepare scheduled task to be configured on the fly
	dummySchedule := "@hourly"
	dummyTaskConfig := &DummyScheduledTaskConfig{ScheduleValue: dummySchedule}
	scheduledTaskProps := &ScheduledTaskProps{
		botType:    botType,
		identifier: "dummyTask",
		taskFunc: func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) {
			return nil, nil
		},
		schedule:           dummySchedule,
		config:             dummyTaskConfig,
		defaultDestination: "",
	}

	// Configure Runner
	runner := &Runner{
		config: NewConfig(),
		bots:   []Bot{bot},
		cmdProps: map[BotType][]*CommandProps{
			bot.BotType(): {
				commandProps,
			},
		},
		taskProps: map[BotType][]*ScheduledTaskProps{
			bot.BotType(): {
				scheduledTaskProps,
			},
		},
		scheduledTasks: map[BotType][]ScheduledTask{
			bot.BotType(): {
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
		if cmd == nil || cmd.Identifier() != commandProps.identifier {
			t.Errorf("Stashed CommandBuilder was not properly configured: %#v.", passedCommand)
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("CommandBuilder was not properly built.")
	}

	select {
	case <-finished:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Runner is not finished.")
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
	alerters := &alerters{
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, err error) error {
				panic("Panic should not affect other alerters' behavior.")
			},
		},
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
	if err := receiveInput(&DummyInput{}); err != nil {
		// Should be received
		t.Errorf("Error should not be returned at this point: %s.", err.Error())
	}

	if err := receiveInput(&DummyInput{}); err == nil {
		// Channel is blocked, but function does not block
		t.Error("Error should be returned on this call.")
	}

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
	if err := receiveInput(&DummyInput{}); err == nil {
		// Receiving goroutine is canceled, but does not block
		t.Error("Error should be returned on this call.")
	}
}
