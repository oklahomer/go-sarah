package sarah

import (
	"errors"
	"golang.org/x/net/context"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

type DummyWorker struct {
	EnqueueFunc func(func()) error
}

func (w *DummyWorker) Enqueue(fnc func()) error {
	return w.EnqueueFunc(fnc)
}

type DummyWatcher struct {
	SubscribeFunc   func(string, string, func(string)) error
	UnsubscribeFunc func(string) error
}

func (w *DummyWatcher) Subscribe(group string, path string, callback func(string)) error {
	return w.SubscribeFunc(group, path, callback)
}

func (w *DummyWatcher) Unsubscribe(group string) error {
	return w.UnsubscribeFunc(group)
}

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config == nil {
		t.Fatal("Expected *Config is not returned.")
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

func TestRunnerOptions_Arg_WithError(t *testing.T) {
	calledCnt := 0
	options := &RunnerOptions{
		func(_ *Runner) error {
			calledCnt++
			return errors.New("something is wrong")
		},
		func(_ *Runner) error {
			calledCnt++
			return nil
		},
	}

	err := options.Arg()(&Runner{})

	if err == nil {
		t.Fatal("Error should be returned.")
	}

	if calledCnt != 1 {
		t.Error("Construction should abort right after first error is returned, but seems like it continued.")
	}
}

func TestNewRunner(t *testing.T) {
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
		commandProps: make(map[BotType][]*CommandProps),
	}

	WithCommandProps(props)(runner)

	botCmdProps, ok := runner.commandProps[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(botCmdProps) != 1 && botCmdProps[0] != props {
		t.Error("Expected CommandProps is not stashed.")
	}
}

func TestWithWorker(t *testing.T) {
	worker := &DummyWorker{}
	runner := &Runner{}
	WithWorker(worker)(runner)

	if runner.worker != worker {
		t.Fatal("Given worker is not set.")
	}
}

func TestWithWatcher(t *testing.T) {
	watcher := &DummyWatcher{}
	runner := &Runner{}
	WithWatcher(watcher)(runner)

	if runner.watcher != watcher {
		t.Fatal("Given watcher is not set.")
	}
}

func TestWithScheduledTaskProps(t *testing.T) {
	var botType BotType = "dummy"
	props := &ScheduledTaskProps{
		botType: botType,
	}
	runner := &Runner{
		scheduledTaskPrps: make(map[BotType][]*ScheduledTaskProps),
	}

	WithScheduledTaskProps(props)(runner)

	taskProps, ok := runner.scheduledTaskPrps[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(taskProps) != 1 && taskProps[0] != props {
		t.Error("Expected ScheduledTaskProps is not stashed.")
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
		botType:    botType,
		identifier: "dummy",
		matchFunc: func(input Input) bool {
			return regexp.MustCompile(`^\.echo`).MatchString(input.Message())
		},
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
		commandProps: map[BotType][]*CommandProps{
			bot.BotType(): {
				commandProps,
			},
		},
		scheduledTaskPrps: map[BotType][]*ScheduledTaskProps{
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
		watcher: &DummyWatcher{
			SubscribeFunc: func(_ string, _ string, _ func(string)) error {
				return nil
			},
			UnsubscribeFunc: func(_ string) error {
				return nil
			},
		},
		worker: &DummyWorker{
			EnqueueFunc: func(fnc func()) error {
				return nil
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
			t.Errorf("Stashed CommandPropsBuilder was not properly configured: %#v.", passedCommand)
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("CommandPropsBuilder was not properly built.")
	}

	select {
	case <-finished:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Runner is not finished.")
	}
}

func TestRunner_Run_Minimal(t *testing.T) {
	config := NewConfig()
	config.PluginConfigRoot = "/"
	runner := &Runner{
		config:            config,
		bots:              []Bot{},
		commandProps:      map[BotType][]*CommandProps{},
		scheduledTaskPrps: map[BotType][]*ScheduledTaskProps{},
		scheduledTasks:    map[BotType][]ScheduledTask{},
		watcher:           nil,
		worker:            nil,
	}

	// Let it run
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	defer cancelRunner()

	runner.Run(runnerCtx)

	if runner.watcher == nil {
		t.Error("Default watcher is not set.")
	}

	if runner.worker == nil {
		t.Error("Default worker is not set.")
	}
}

func Test_runBot(t *testing.T) {
	var givenErr error
	bot := &DummyBot{
		RunFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {
			panic("panic!!!")
		},
	}
	runBot(
		context.TODO(),
		bot,
		func(_ Input) error {
			return nil
		},
		func(err error) {
			givenErr = err
		},
	)

	if givenErr == nil {
		t.Fatal("Expected error is not returned.")
	}

	if _, ok := givenErr.(*BotNonContinuableError); !ok {
		t.Errorf("Expected error type is not given: %#v.", givenErr)
	}
}

func Test_registerCommand(t *testing.T) {
	command := &DummyCommand{}
	var appendedCommand Command
	bot := &DummyBot{AppendCommandFunc: func(cmd Command) { appendedCommand = cmd }}

	bot.AppendCommand(command)

	if appendedCommand != command {
		t.Error("Given Command is not appended.")
	}
}

func Test_registerScheduledTask(t *testing.T) {
	called := false
	callbackCalled := false
	bot := &DummyBot{}
	task := &DummyScheduledTask{
		ExecuteFunc: func(_ context.Context) ([]*ScheduledTaskResult, error) {
			callbackCalled = true
			return nil, nil
		},
	}
	scheduler := &DummyScheduler{
		UpdateFunc: func(_ BotType, _ ScheduledTask, callback func()) error {
			called = true
			callback()
			return nil
		},
	}

	registerScheduledTask(context.TODO(), bot, task, scheduler)

	if called == false {
		t.Error("Scheduler's update func is not called.")
	}

	if callbackCalled == false {
		t.Error("Callback function is not called.")
	}
}

func Test_commandUpdaterFunc(t *testing.T) {
	var botType BotType = "dummy"
	registeredCommand := 0
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(_ Command) {
			registeredCommand++
		},
	}
	props := []*CommandProps{
		{
			identifier:  "irrelevant",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      nil,
			example:     "exampleInput",
		},
		{
			identifier:  "matching",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      struct{ foo string }{foo: "broken"},
			example:     "exampleInput",
		},
	}

	updater := commandUpdaterFunc(bot, props)
	updater(filepath.Join("testdata", "command", "matching.yaml"))

	if registeredCommand != 1 {
		t.Errorf("Only one comamnd is expected to be registered: %d.", registeredCommand)
	}
}

func Test_commandUpdaterFunc_WithBrokenYaml(t *testing.T) {
	var botType BotType = "dummy"
	registeredCommand := 0
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(_ Command) {
			registeredCommand++
		},
	}
	props := []*CommandProps{
		{
			identifier:  "broken",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      struct{ foo string }{foo: "broken"},
			example:     "exampleInput",
		},
	}

	updater := commandUpdaterFunc(bot, props)
	updater(filepath.Join("testdata", "command", "broken.yaml"))

	if registeredCommand != 0 {
		t.Errorf("No comamnd is expected to be registered: %d.", registeredCommand)
	}
}

func Test_scheduledTaskUpdaterFunc(t *testing.T) {
	var botType BotType = "dummy"
	registeredCommand := 0
	bot := &DummyBot{
		BotTypeValue: botType,
	}
	props := []*ScheduledTaskProps{
		{
			botType:            botType,
			identifier:         "irrelevant",
			taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
			schedule:           "@every 1m",
			defaultDestination: "boo",
			config:             nil,
		},
		{
			botType:            botType,
			identifier:         "dummy",
			taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
			schedule:           "@every 1m",
			defaultDestination: "dummy",
			config:             nil,
		},
	}
	scheduler := &DummyScheduler{
		UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
			registeredCommand++
			return nil
		},
	}

	updater := scheduledTaskUpdaterFunc(context.TODO(), bot, props, scheduler)
	updater(filepath.Join("testdata", "command", "dummy.yaml"))

	if registeredCommand != 1 {
		t.Errorf("Only one comamnd is expected to be registered: %d.", registeredCommand)
	}
}

func Test_scheduledTaskUpdaterFunc_WithBrokenYaml(t *testing.T) {
	var botType BotType = "dummy"
	registeredCommand := 0
	bot := &DummyBot{
		BotTypeValue: botType,
	}
	props := []*ScheduledTaskProps{
		{
			botType:            botType,
			identifier:         "broken",
			taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
			schedule:           "@every 1m",
			defaultDestination: "boo",
			config:             &struct{ token string }{},
		},
	}
	scheduler := &DummyScheduler{
		UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
			registeredCommand++
			return nil
		},
	}

	updater := scheduledTaskUpdaterFunc(context.TODO(), bot, props, scheduler)
	updater(filepath.Join("testdata", "command", "broken.yaml"))

	if registeredCommand != 0 {
		t.Errorf("No comamnd is expected to be registered: %d.", registeredCommand)
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
	responded := make(chan bool, 1)
	worker := &DummyWorker{
		EnqueueFunc: func(fnc func()) error {
			fnc()
			return nil
		},
	}

	bot := &DummyBot{
		BotTypeValue: "DUMMY",
		RespondFunc: func(_ context.Context, input Input) error {
			responded <- true
			return errors.New("error is returned, but still doesn't block")
		},
	}

	receiveInput := setupInputReceiver(context.TODO(), bot, worker)
	if err := receiveInput(&DummyInput{}); err != nil {
		t.Errorf("Error should not be returned at this point: %s.", err.Error())
	}

	select {
	case <-responded:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Received input was not processed.")
	}
}

func Test_setupInputReceiver_BlockedInputError(t *testing.T) {
	bot := &DummyBot{}
	worker := &DummyWorker{
		EnqueueFunc: func(fnc func()) error {
			return errors.New("any error should result in BlockedInputError")
		},
	}

	receiveInput := setupInputReceiver(context.TODO(), bot, worker)
	err := receiveInput(&DummyInput{})
	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if _, ok := err.(*BlockedInputError); !ok {
		t.Fatalf("Expected error type is not returned: %T.", err)
	}
}
