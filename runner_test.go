package sarah

import (
	"errors"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"io/ioutil"
	stdLogger "log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := log.GetLogger()
	defer log.SetLogger(oldLogger)

	l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
	logger := log.NewWithStandardLogger(l)
	log.SetLogger(logger)

	code := m.Run()

	os.Exit(code)
}

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
	options.Append(func(_ *runner) error { return nil })

	if len(*options) != 1 {
		t.Errorf("Size of stashed options should be 0 at first, but was %d.", len(*options))
	}
}

func TestRunnerOptions_Arg(t *testing.T) {
	options := &RunnerOptions{}
	calledCnt := 0
	*options = append(
		*options,
		func(_ *runner) error {
			calledCnt++
			return nil
		},
		func(_ *runner) error {
			calledCnt++
			return nil
		},
	)

	options.Arg()(&runner{})

	if calledCnt != 2 {
		t.Fatalf("Options are not properly called. Count: %d.", calledCnt)
	}
}

func TestRunnerOptions_Arg_WithError(t *testing.T) {
	calledCnt := 0
	options := &RunnerOptions{
		func(_ *runner) error {
			calledCnt++
			return errors.New("something is wrong")
		},
		func(_ *runner) error {
			calledCnt++
			return nil
		},
	}

	err := options.Arg()(&runner{})

	if err == nil {
		t.Fatal("Error should be returned.")
	}

	if calledCnt != 1 {
		t.Error("Construction should abort right after first error is returned, but seems like it continued.")
	}
}

func TestNewRunner(t *testing.T) {
	config := NewConfig()
	r, err := NewRunner(config)

	if err != nil {
		t.Fatalf("Unexpected error: %#v.", err)
	}

	if r == nil {
		t.Fatal("NewRunner reutrned nil.")
	}

	impl, ok := r.(*runner)
	if !ok {
		t.Fatalf("Returned instance is not runner instance: %T", r)
	}

	if impl.config != config {
		t.Errorf("Passed config is not set: %#v.", impl.config)
	}

	if impl.bots == nil {
		t.Error("Bot slice is nil.")
	}

	if impl.scheduledTasks == nil {
		t.Error("scheduledTasks are not set.")
	}
}

func TestNewRunner_WithRunnerOption(t *testing.T) {
	called := false
	config := NewConfig()
	r, err := NewRunner(
		config,
		func(_ *runner) error {
			called = true
			return nil
		},
	)

	if r == nil {
		t.Error("runner instance should be returned.")
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
	r, err := NewRunner(
		config,
		func(_ *runner) error {
			called = true
			return nil
		},
		func(_ *runner) error {
			return optErr
		},
	)

	if r != nil {
		t.Error("runner instance should not be returned on error.")
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
	r := &runner{
		bots: []Bot{},
	}

	WithBot(bot)(r)

	registeredBots := r.bots
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
	r := &runner{
		commandProps: make(map[BotType][]*CommandProps),
	}

	WithCommandProps(props)(r)

	botCmdProps, ok := r.commandProps[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(botCmdProps) != 1 && botCmdProps[0] != props {
		t.Error("Expected CommandProps is not stashed.")
	}
}

func TestWithWorker(t *testing.T) {
	worker := &DummyWorker{}
	r := &runner{}
	WithWorker(worker)(r)

	if r.worker != worker {
		t.Fatal("Given worker is not set.")
	}
}

func TestWithWatcher(t *testing.T) {
	watcher := &DummyWatcher{}
	r := &runner{}
	WithWatcher(watcher)(r)

	if r.watcher != watcher {
		t.Fatal("Given watcher is not set.")
	}
}

func TestWithScheduledTaskProps(t *testing.T) {
	var botType BotType = "dummy"
	props := &ScheduledTaskProps{
		botType: botType,
	}
	r := &runner{
		scheduledTaskPrps: make(map[BotType][]*ScheduledTaskProps),
	}

	WithScheduledTaskProps(props)(r)

	taskProps, ok := r.scheduledTaskPrps[botType]
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
	r := &runner{
		scheduledTasks: make(map[BotType][]ScheduledTask),
	}

	WithScheduledTask(botType, task)(r)

	tasks, ok := r.scheduledTasks[botType]
	if !ok {
		t.Fatal("Expected BotType is not stashed as key.")
	}
	if len(tasks) != 1 && tasks[0] != task {
		t.Errorf("Expected task is not stashed: %#v", tasks)
	}
}

func TestWithAlerter(t *testing.T) {
	alerter := &DummyAlerter{}
	r := &runner{
		alerters: &alerters{},
	}

	WithAlerter(alerter)(r)

	registeredAlerters := r.alerters
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

	// Configure runner
	r := &runner{
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
		r.Run(runnerCtx)
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

func TestRunner_Run_WithPluginConfigRoot(t *testing.T) {
	config := &Config{
		PluginConfigRoot: "dummy/config",
		TimeZone:         time.Now().Location().String(),
	}

	var botType BotType = "bot"
	bot := &DummyBot{
		BotTypeValue: botType,
		RunFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {
			return
		},
	}

	subscribeCh := make(chan struct{}, 2)
	r := &runner{
		config:            config,
		bots:              []Bot{bot},
		commandProps:      map[BotType][]*CommandProps{},
		scheduledTaskPrps: map[BotType][]*ScheduledTaskProps{},
		scheduledTasks:    map[BotType][]ScheduledTask{},
		watcher: &DummyWatcher{
			SubscribeFunc: func(_ string, _ string, _ func(string)) error {
				subscribeCh <- struct{}{}
				return errors.New("this error should not cause fatal state")
			},
			UnsubscribeFunc: func(_ string) error {
				return errors.New("this error also should not cause fatal state")
			},
		},
		worker: &DummyWorker{},
	}

	// Let it run
	rootCtx := context.Background()
	runnerCtx, cancelRunner := context.WithCancel(rootCtx)
	go r.Run(runnerCtx)

	// Wait till all setup is done.
	time.Sleep(100 * time.Millisecond)
	cancelRunner()

	select {
	case <-subscribeCh:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Watcher.Subscribe is not called.")
	}
}

func TestRunner_Run_Minimal(t *testing.T) {
	config := &Config{
		PluginConfigRoot: "/",
		TimeZone:         time.Now().Location().String(),
	}
	r := &runner{
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

	r.Run(runnerCtx)

	if r.watcher == nil {
		t.Error("Default watcher is not set.")
	}

	if r.worker == nil {
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

func Test_updateCommandConfig(t *testing.T) {
	type config struct {
		Token string
	}
	c := &config{
		Token: "default",
	}

	var botType BotType = "dummy"
	var appendCalled bool
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(_ Command) {
			appendCalled = true
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
			identifier:  "dummy",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      c,
			example:     "exampleInput",
		},
	}

	file := &pluginConfigFile{
		id:       "dummy",
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}
	err := updateCommandConfig(bot, props, file)

	if err != nil {
		t.Errorf("Unexpected error returned: %s.", err.Error())
	}

	if appendCalled {
		t.Error("Bot.AppendCommand should not be called when pointer to CommandConfig is given.")
	}

	if c.Token != "foobar" {
		t.Errorf("Expected configuration value is not set: %s.", c.Token)
	}
}

func Test_updateCommandConfig_WithBrokenYaml(t *testing.T) {
	type config struct {
		Token string
	}
	c := &config{
		Token: "default",
	}

	var botType BotType = "dummy"
	bot := &DummyBot{
		BotTypeValue: botType,
	}
	props := []*CommandProps{
		{
			identifier:  "broken",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      c,
			example:     "exampleInput",
		},
	}

	file := &pluginConfigFile{
		id:       "broken",
		path:     filepath.Join("testdata", "command", "broken.yaml"),
		fileType: yamlFile,
	}
	err := updateCommandConfig(bot, props, file)

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if err == errUnableToDetermineConfigFileFormat || err == errUnsupportedConfigFileFormat {
		t.Errorf("Unexpected error type was returned: %T", err)
	}
}

func Test_updateCommandConfig_WithConfigValue(t *testing.T) {
	type config struct {
		Token string
	}
	c := config{
		Token: "default",
	}

	var botType BotType = "dummy"
	var newCmd Command
	bot := &DummyBot{
		BotTypeValue: botType,
		AppendCommandFunc: func(cmd Command) {
			newCmd = cmd
		},
	}
	props := []*CommandProps{
		{
			identifier:  "dummy",
			botType:     botType,
			commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
			matchFunc:   func(_ Input) bool { return true },
			config:      c,
			example:     "exampleInput",
		},
	}

	file := &pluginConfigFile{
		id:       "dummy",
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}
	err := updateCommandConfig(bot, props, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if err == errUnableToDetermineConfigFileFormat || err == errUnsupportedConfigFileFormat {
		t.Errorf("Unexpected error type was returned: %T.", err)
	}

	if newCmd == nil {
		t.Error("Bot.AppendCommand must be called to replace old Command when config value is set instead of pointer.")
	}

	v := newCmd.(*defaultCommand).configWrapper.value.(config).Token
	if v != "foobar" {
		t.Errorf("Newly set config does not reflect value from file: %s.", v)
	}
}

func Test_updateScheduledTaskConfig(t *testing.T) {
	var botType BotType = "dummy"
	registeredScheduledTask := 0
	bot := &DummyBot{
		BotTypeValue: botType,
	}

	type config struct {
		Token string
	}
	c := &config{
		Token: "default",
	}

	props := []*ScheduledTaskProps{
		{
			botType:            botType,
			identifier:         "irrelevant",
			taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
			schedule:           "@every 1m",
			defaultDestination: "boo",
			config:             c,
		},
		{
			botType:            botType,
			identifier:         "dummy",
			taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
			schedule:           "@every 1m",
			defaultDestination: "dummy",
			config:             c,
		},
	}
	scheduler := &DummyScheduler{
		UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
			registeredScheduledTask++
			return nil
		},
	}

	file := &pluginConfigFile{
		id:       "dummy",
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}
	err := updateScheduledTaskConfig(context.TODO(), bot, props, scheduler, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if registeredScheduledTask != 1 {
		t.Errorf("Only one comamnd is expected to be registered: %d.", registeredScheduledTask)
	}
}

func Test_updateScheduledTaskConfig_WithBrokenYaml(t *testing.T) {
	var botType BotType = "dummy"
	registeredScheduledTask := 0
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
			config:             &struct{ Token string }{},
		},
	}

	var removeCalled bool
	scheduler := &DummyScheduler{
		UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
			registeredScheduledTask++
			return nil
		},
		RemoveFunc: func(_ BotType, _ string) error {
			removeCalled = true
			return nil
		},
	}

	file := &pluginConfigFile{
		id:       "broken",
		path:     filepath.Join("testdata", "command", "broken.yaml"),
		fileType: yamlFile,
	}
	err := updateScheduledTaskConfig(context.TODO(), bot, props, scheduler, file)

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if registeredScheduledTask != 0 {
		t.Errorf("No comamnd is expected to be registered: %d.", registeredScheduledTask)
	}

	if !removeCalled {
		t.Error("scheduler.remove should be removed when config update fails.")
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
			configWrapper: &taskConfigWrapper{
				value: &DummyScheduledTaskConfig{},
				mutex: &sync.RWMutex{},
			},
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

func Test_plainPathToFile(t *testing.T) {
	tests := []struct {
		input    string
		id       string
		path     string
		fileType fileType
		err      error
	}{
		{
			input:    "./foo/bar.yaml",
			id:       "bar",
			path:     func() string { p, _ := filepath.Abs("./foo/bar.yaml"); return p }(),
			fileType: yamlFile,
		},
		{
			input:    "/abs/foo/bar.yml",
			id:       "bar",
			path:     func() string { p, _ := filepath.Abs("/abs/foo/bar.yml"); return p }(),
			fileType: yamlFile,
		},
		{
			input:    "foo/bar.json",
			id:       "bar",
			path:     func() string { p, _ := filepath.Abs("foo/bar.json"); return p }(),
			fileType: jsonFile,
		},
		{
			input: "/abs/foo/undetermined",
			err:   errUnableToDetermineConfigFileFormat,
		},
		{
			input: "/abs/foo/unsupported.csv",
			err:   errUnsupportedConfigFileFormat,
		},
	}

	for i, test := range tests {
		testNo := i + 1
		file, err := plainPathToFile(test.input)

		if test.err == nil {
			if err != nil {
				t.Errorf("Unexpected error is retuend on test %d: %s.", testNo, err.Error())
				continue
			}

			if file.id != test.id {
				t.Errorf("Unexpected id is set on test %d: %s.", testNo, file.id)
			}

			if file.path != test.path {
				t.Errorf("Unexpected path is set on test %d: %s.", testNo, file.path)
			}

			if file.fileType != test.fileType {
				t.Errorf("Unexpected fileType is set on test %d: %d.", testNo, file.fileType)
			}

			continue
		}

		if err != test.err {
			t.Errorf("Unexpected error is returned: %#v.", err)
		}
	}
}

func Test_findPluginConfigFile(t *testing.T) {
	tests := []struct {
		configDir string
		id        string
		found     bool
	}{
		{
			configDir: filepath.Join("testdata", "command"),
			id:        "dummy",
			found:     true,
		},
		{
			configDir: filepath.Join("testdata", "nonExistingDir"),
			id:        "notFound",
			found:     false,
		},
	}

	for i, test := range tests {
		testNo := i + 1
		file := findPluginConfigFile(test.configDir, test.id)
		if test.found && file == nil {
			t.Error("Expected *pluginConfigFile is not returned.")
		} else if !test.found && file != nil {
			t.Errorf("Unexpected returned value on test %d: %#v.", testNo, file)
		}
	}
}
