package sarah

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"io"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := logger.GetLogger()
	defer logger.SetLogger(oldLogger)

	// Suppress log output in test by default
	l := log.New(io.Discard, "dummyLog", 0)
	logger.SetLogger(logger.NewWithStandardLogger(l))

	code := m.Run()

	os.Exit(code)
}

func SetupAndRun(fnc func()) {
	// Initialize package variables
	runnerStatus = &status{}
	options = &optionHolder{}

	fnc()
}

type DummyConfigWatcher struct {
	ReadFunc    func(context.Context, BotType, string, interface{}) error
	WatchFunc   func(context.Context, BotType, string, func()) error
	UnwatchFunc func(BotType) error
}

func (w *DummyConfigWatcher) Read(botCtx context.Context, botType BotType, id string, configPtr interface{}) error {
	return w.ReadFunc(botCtx, botType, id, configPtr)
}

func (w *DummyConfigWatcher) Watch(ctx context.Context, botType BotType, id string, callback func()) error {
	return w.WatchFunc(ctx, botType, id, callback)
}

func (w *DummyConfigWatcher) Unwatch(botType BotType) error {
	return w.UnwatchFunc(botType)
}

type DummyWorker struct {
	EnqueueFunc func(func()) error
}

func (w *DummyWorker) Enqueue(fnc func()) error {
	return w.EnqueueFunc(fnc)
}

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config == nil {
		t.Fatal("Expected *Config is not returned.")
	}
}

func Test_optionHolder_register(t *testing.T) {
	opt := func(_ *runner) {}
	holder := &optionHolder{}
	holder.register(opt)

	if len(holder.stashed) != 1 {
		t.Fatalf("Expected number of options are not stashed: %d.", len(holder.stashed))
	}

	if reflect.ValueOf(holder.stashed[0]).Pointer() != reflect.ValueOf(opt).Pointer() {
		t.Error("Given option is not stashed.")
	}
}

func Test_optionHandler_apply(t *testing.T) {
	called := 0
	holder := &optionHolder{}
	holder.stashed = []func(*runner){
		func(_ *runner) {
			called++
		},
		func(_ *runner) {
			called++
		},
	}
	r := &runner{}

	holder.apply(r)

	if called != 2 {
		t.Errorf("Unexpected number of options are applied: %d.", called)
	}
}

func TestRegisterAlerter(t *testing.T) {
	SetupAndRun(func() {
		alerter := &DummyAlerter{}
		RegisterAlerter(alerter)
		r := &runner{
			alerters: &alerters{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(*r.alerters) != 1 {
			t.Fatalf("Expected number of alerter is not registered: %d.", len(*r.alerters))
		}

		if (*r.alerters)[0] != alerter {
			t.Error("Given alerter is not registered.")
		}
	})
}

func TestRegisterBot(t *testing.T) {
	SetupAndRun(func() {
		bot := &DummyBot{}
		RegisterBot(bot)
		r := &runner{
			alerters: &alerters{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(r.bots) != 1 {
			t.Fatalf("Expected number of bot is not registered: %d.", len(r.bots))
		}

		if r.bots[0] != bot {
			t.Error("Given bot is not registered.")
		}
	})
}

func TestRegisterCommand(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "dummy"
		command := &DummyCommand{}
		RegisterCommand(botType, command)
		r := &runner{
			commands: map[BotType][]Command{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(r.commands[botType]) != 1 {
			t.Fatalf("Expected number of Command is not registered: %d.", len(r.commandProps[botType]))
		}

		if r.commands[botType][0] != command {
			t.Error("Given Command is not registered.")
		}
	})
}

func TestRegisterCommandProps(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "dummy"
		props := &CommandProps{
			botType: botType,
		}
		RegisterCommandProps(props)
		r := &runner{
			commandProps: map[BotType][]*CommandProps{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(r.commandProps[botType]) != 1 {
			t.Fatalf("Expected number of CommandProps is not registered: %d.", len(r.commandProps[botType]))
		}

		if r.commandProps[botType][0] != props {
			t.Error("Given CommandProps is not registered.")
		}
	})
}

func TestRegisterScheduledTask(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "dummy"
		task := &DummyScheduledTask{}
		RegisterScheduledTask(botType, task)
		r := &runner{
			scheduledTasks: map[BotType][]ScheduledTask{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(r.scheduledTasks[botType]) != 1 {
			t.Fatalf("Expected number of ScheduledTask is not registered: %d.", len(r.scheduledTasks[botType]))
		}

		if r.scheduledTasks[botType][0] != task {
			t.Error("Given ScheduledTask is not registered.")
		}
	})
}

func TestRegisterScheduledTaskProps(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "dummy"
		props := &ScheduledTaskProps{
			botType: botType,
		}
		RegisterScheduledTaskProps(props)
		r := &runner{
			scheduledTaskProps: map[BotType][]*ScheduledTaskProps{},
		}

		for _, v := range options.stashed {
			v(r)
		}

		if len(r.scheduledTaskProps[botType]) != 1 {
			t.Fatalf("Expected number of ScheduledTaskProps is not registered: %d.", len(r.scheduledTaskProps[botType]))
		}

		if r.scheduledTaskProps[botType][0] != props {
			t.Error("Given ScheduledTaskProps is not registered.")
		}
	})
}

func TestRegisterConfigWatcher(t *testing.T) {
	SetupAndRun(func() {
		watcher := &DummyConfigWatcher{}
		RegisterConfigWatcher(watcher)
		r := &runner{}

		for _, v := range options.stashed {
			v(r)
		}

		if r.configWatcher == nil {
			t.Fatal("ConfigWatcher is not set")
		}

		if r.configWatcher != watcher {
			t.Error("Given ConfigWatcher is not set.")
		}
	})
}

func TestRegisterWorker(t *testing.T) {
	SetupAndRun(func() {
		worker := &DummyWorker{}
		RegisterWorker(worker)
		r := &runner{}

		for _, v := range options.stashed {
			v(r)
		}

		if r.worker == nil {
			t.Fatal("Worker is not set")
		}
		if r.worker != worker {
			t.Error("Given Worker is not set.")
		}
	})
}

func TestRegisterBotErrorSupervisor(t *testing.T) {
	SetupAndRun(func() {
		supervisor := func(_ BotType, _ error) *SupervisionDirective {
			return nil
		}
		RegisterBotErrorSupervisor(supervisor)
		r := &runner{}

		for _, v := range options.stashed {
			v(r)
		}

		if r.superviseError == nil {
			t.Fatal("superviseError is not set.")
		}

		if reflect.ValueOf(r.superviseError).Pointer() != reflect.ValueOf(supervisor).Pointer() {
			t.Error("Passed function is not set.")
		}
	})
}

func TestRun(t *testing.T) {
	SetupAndRun(func() {
		config := &Config{
			TimeZone: time.UTC.String(),
		}

		// Initial call with valid setting should work.
		err := Run(context.Background(), config)
		if err != nil {
			t.Fatalf("Unexpected error is returned: %s.", err.Error())
		}

		err = Run(context.Background(), config)
		if err == nil {
			t.Fatal("Expected error is not returned.")
		}

		// Wait for the runner goroutine spawned by Run() to finish.
		// No bots are registered, so runner.run() returns almost immediately.
		select {
		case <-runnerStatus.finished:
			// runner.run() has completed and called runnerStatus.stop().

		case <-time.NewTimer(1 * time.Second).C:
			t.Fatal("Runner goroutine did not finish in time.")
		}
	})
}

func TestRun_WithInvalidConfig(t *testing.T) {
	SetupAndRun(func() {
		config := &Config{
			TimeZone: "INVALID",
		}

		err := Run(context.Background(), config)

		if err == nil {
			t.Error("Expected error is not returned.")
		}
	})
}

func Test_newRunner(t *testing.T) {
	SetupAndRun(func() {
		config := &Config{
			TimeZone: time.UTC.String(),
		}

		r, e := newRunner(context.Background(), config)
		if e != nil {
			t.Fatalf("Unexpected error is returned: %s.", e.Error())
		}

		if r == nil {
			t.Fatal("runner instance is not returned.")
		}

		if r.configWatcher == nil {
			t.Error("Default ConfigWatcher should be set when PluginConfigRoot is not empty.")
		}

		if r.scheduler == nil {
			t.Error("Scheduler must run at this point.")
		}

		if r.worker == nil {
			t.Error("Default Worker should be set.")
		}
	})
}

func Test_newRunner_WithTimeZoneError(t *testing.T) {
	SetupAndRun(func() {
		config := &Config{
			TimeZone: "DUMMY",
		}

		_, e := newRunner(context.Background(), config)
		if e == nil {
			t.Fatal("Expected error is not returned.")
		}
	})
}

func Test_runner_run(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"

		bot := &DummyBot{
			BotTypeValue: botType,
			RunFunc: func(ctx context.Context, _ func(Input) error, _ func(error)) {
				<-ctx.Done()
			},
		}

		config := &Config{
			TimeZone: time.Now().Location().String(),
		}

		r := &runner{
			config: config,
			bots: []Bot{
				bot,
			},
		}

		rootCtx := context.Background()
		ctx, cancel := context.WithCancel(rootCtx)
		finished := make(chan struct{})
		go func() {
			r.run(ctx)
			close(finished)
		}()

		time.Sleep(1 * time.Second)

		status := CurrentStatus()

		if len(status.Bots) != 1 {
			t.Fatalf("Expected number of Bot is not registered.")
		}

		if status.Bots[0].Type != botType {
			t.Errorf("Unexpected BotStatus.Type is returned: %s.", status.Bots[0].Type)
		}

		if !status.Bots[0].Running {
			t.Error("BotStatus.Running should be true at this point.")
		}

		cancel()

		select {
		case <-finished:
			// r.run() has fully returned, including runnerStatus.stop()

		case <-time.NewTimer(1 * time.Second).C:
			t.Fatal("runner.run did not return in time.")
		}

		currentStatus := CurrentStatus()

		if currentStatus.Bots[0].Running {
			t.Error("BotStatus.Running should not be true at this point.")
		}
		if currentStatus.Running {
			t.Error("Status.Running should be false after all bots stop.")
		}
	})

}

func Test_runner_runBot(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"

		// Prepare Bot to be run
		passedCommand := make(chan Command, 1)
		bot := &DummyBot{
			BotTypeValue: botType,
			AppendCommandFunc: func(cmd Command) {
				passedCommand <- cmd
			},
			RunFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {},
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
			instructionFunc: func(_ *HelpInput) string {
				return ".echo foo"
			},
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
		config := &Config{
			TimeZone: time.Now().Location().String(),
		}
		alerted := make(chan struct{}, 1)
		r := &runner{
			config: config,
			bots:   []Bot{bot},
			commandProps: map[BotType][]*CommandProps{
				bot.BotType(): {
					commandProps,
				},
			},
			scheduledTaskProps: map[BotType][]*ScheduledTaskProps{
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
			configWatcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
					return nil
				},
				WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
					return nil
				},
				UnwatchFunc: func(_ BotType) error {
					return nil
				},
			},
			worker: &DummyWorker{
				EnqueueFunc: func(fnc func()) error {
					return nil
				},
			},
			scheduler: &DummyScheduler{
				UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
					return nil
				},
				RemoveFunc: func(_ BotType, _ string) {},
			},
			alerters: &alerters{
				&DummyAlerter{
					AlertFunc: func(_ context.Context, _ BotType, err error) error {
						alerted <- struct{}{}
						return nil
					},
				},
			},
		}

		// Let it run
		rootCtx := context.Background()
		runnerCtx, cancelRunner := context.WithCancel(rootCtx)
		finished := make(chan bool)
		go func() {
			r.runBot(runnerCtx, bot)
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

		if CurrentStatus().Running {
			t.Error("Status.Running should be false at this point.")
		}

		select {
		case <-alerted:
			// O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Alert should be sent no matter how runner is canceled.")

		}
	})
}

func Test_runner_runBot_WithPanic(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"

		// Prepare Bot to be run
		bot := &DummyBot{
			BotTypeValue: botType,
			AppendCommandFunc: func(cmd Command) {
			},
			RunFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {
				panic("panic on runner.Run")
			},
		}

		// Configure runner
		config := &Config{
			TimeZone: time.Now().Location().String(),
		}
		alerted := make(chan struct{}, 1)
		r := &runner{
			config: config,
			bots:   []Bot{bot},
			alerters: &alerters{
				&DummyAlerter{
					AlertFunc: func(_ context.Context, _ BotType, err error) error {
						alerted <- struct{}{}
						return nil
					},
				},
			},
		}

		if CurrentStatus().Running {
			t.Error("Status.Running should be false at this point.")
		}

		// Let it run
		rootCtx := context.Background()
		runnerCtx, cancel := context.WithCancel(rootCtx)
		defer cancel()
		finished := make(chan bool)
		go func() {
			r.runBot(runnerCtx, bot)
			finished <- true
		}()

		time.Sleep(1 * time.Second)

		select {
		case <-finished:
			// O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Runner is not finished.")

		}

		if CurrentStatus().Running {
			t.Error("Status.Running should be false at this point.")
		}

		select {
		case <-alerted:
			// O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Alert should be sent no matter how runner is canceled.")

		}
	})
}

func Test_runner_superviseBot(t *testing.T) {
	tests := []struct {
		escalated error
		directive *SupervisionDirective
		shutdown  bool
	}{
		{
			escalated: NewBotNonContinuableError("this should stop Bot"),
			shutdown:  true,
		},
		{
			escalated: errors.New("plain error"),
			directive: nil,
			shutdown:  false,
		},
		{
			escalated: errors.New("plain error"),
			directive: &SupervisionDirective{
				AlertingErr: errors.New("this is sent via alerter"),
				StopBot:     true,
			},
			shutdown: true,
		},
		{
			escalated: errors.New("plain error"),
			directive: &SupervisionDirective{
				AlertingErr: nil,
				StopBot:     true,
			},
			shutdown: true,
		},
		{
			escalated: errors.New("plain error"),
			directive: &SupervisionDirective{
				AlertingErr: errors.New("this is sent via alerter"),
				StopBot:     false,
			},
			shutdown: false,
		},
		{
			escalated: errors.New("plain error"),
			directive: &SupervisionDirective{
				AlertingErr: nil,
				StopBot:     false,
			},
			shutdown: false,
		},
	}
	alerted := make(chan error, 1)

	for i, tt := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := &runner{
				alerters: &alerters{
					&DummyAlerter{
						AlertFunc: func(_ context.Context, _ BotType, err error) error {
							panic("Panic should not affect other alerters' behavior.")
						},
					},
					&DummyAlerter{
						AlertFunc: func(_ context.Context, _ BotType, err error) error {
							alerted <- err
							return nil
						},
					},
				},
				superviseError: func(_ BotType, _ error) *SupervisionDirective {
					return tt.directive
				},
			}
			rootCxt := context.Background()
			botCtx, errSupervisor := r.superviseBot(rootCxt, "DummyBotType")

			// Make sure the Bot state is currently active
			select {
			case <-botCtx.Done():
				t.Error("Bot context should not be canceled at this point.")

			default:
				// O.K.

			}

			// Escalate an error
			errSupervisor(tt.escalated)

			if tt.shutdown {
				// Bot should be canceled
				select {
				case <-botCtx.Done():
					// O.K.

				case <-time.NewTimer(1 * time.Second).C:
					t.Error("Bot context should be canceled at this point.")

				}
				if e := botCtx.Err(); e != context.Canceled {
					t.Errorf("botCtx.Err() must return context.Canceled, but was %#v", e)
				}
			}

			if _, ok := tt.escalated.(*BotNonContinuableError); ok {
				// When Bot escalate an non-continuable error, then alerter should be called.
				select {
				case e := <-alerted:
					if e != tt.escalated {
						t.Errorf("Unexpected error value is passed: %#v", e)
					}

				case <-time.NewTimer(1 * time.Second).C:
					t.Error("Alerter is not called.")

				}
			} else if tt.directive != nil && tt.directive.AlertingErr != nil {
				select {
				case e := <-alerted:
					if e != tt.directive.AlertingErr {
						t.Errorf("Unexpected error value is passed: %#v", e)
					}

				case <-time.NewTimer(1 * time.Second).C:
					t.Error("Alerter is not called.")

				}
			}

			// See if a succeeding call block
			nonBlocking := make(chan bool)
			go func() {
				errSupervisor(errors.New("succeeding calls should never block"))
				nonBlocking <- true
			}()
			select {
			case <-nonBlocking:
				// O.K.

			case <-time.NewTimer(10 * time.Second).C:
				t.Error("Succeeding error escalation blocks.")

			}
		})
	}
}

func Test_executeScheduledTask(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_setupInputReceiver(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_setupInputReceiver_BlockedInputError(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_registerCommands(t *testing.T) {
	SetupAndRun(func() {
		tests := []struct {
			configWatcher ConfigWatcher
			props         []*CommandProps
			commands      []Command
			callback      bool
			regNum        int
		}{
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						return nil
					},
				},
				props: []*CommandProps{
					{},
				},
				callback: false,
				regNum:   1,
			},
			{
				configWatcher: &DummyConfigWatcher{
					ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
						return errors.New("configuration error")
					},
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						return errors.New("subscription error")
					},
				},
				props: []*CommandProps{
					{
						config: struct{}{},
					},
				},
				callback: false,
				regNum:   0,
			},
			{
				configWatcher: &DummyConfigWatcher{
					ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
						return nil
					},
					WatchFunc: func(_ context.Context, _ BotType, id string, callback func()) error {
						callback()
						return nil
					},
				},
				props: []*CommandProps{
					{
						config: struct{}{},
					},
				},
				callback: false,
				regNum:   2,
			},
			{
				configWatcher: &DummyConfigWatcher{
					ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
						t.Error("ConfigWatcher should not be called when pre-built Command is given.")
						return nil
					},
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						t.Error("ConfigWatcher should not be called when pre-built Command is given.")
						return nil
					},
				},
				commands: []Command{
					&DummyCommand{},
				},
				regNum: 1,
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				regNum := 0
				botType := BotType(fmt.Sprintf("bot%d", i))
				bot := &DummyBot{
					BotTypeValue: botType,
					AppendCommandFunc: func(command Command) {
						regNum++
					},
				}
				r := &runner{
					configWatcher: tt.configWatcher,
					commands: map[BotType][]Command{
						botType: tt.commands,
					},
					commandProps: map[BotType][]*CommandProps{
						botType: tt.props,
					},
				}

				r.registerCommands(context.TODO(), bot)

				if tt.regNum != regNum {
					t.Errorf("Unexpected number of command registration call: %d.", regNum)
				}
			})
		}
	})
}

func Test_registerScheduledTasks(t *testing.T) {
	SetupAndRun(func() {
		tests := []struct {
			configWatcher ConfigWatcher
			props         []*ScheduledTaskProps
			tasks         []ScheduledTask
			updateError   bool
			regNum        int
		}{
			{
				tasks: []ScheduledTask{
					&scheduledTask{
						schedule: "",
					},
				},
				updateError: false,
				regNum:      0,
			},
			{
				tasks: []ScheduledTask{
					&scheduledTask{
						schedule: "@daily",
					},
				},
				updateError: false,
				regNum:      1,
			},
			{
				tasks: []ScheduledTask{
					&scheduledTask{
						schedule: "@daily",
					},
				},
				updateError: true,
				regNum:      0,
			},
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						return nil
					},
				},
				props: []*ScheduledTaskProps{
					{
						schedule: "@daily",
					},
				},
				updateError: false,
				regNum:      1,
			},
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, id string, callback func()) error {
						callback()
						return nil
					},
				},
				props: []*ScheduledTaskProps{
					{
						schedule: "@daily",
					},
				},
				updateError: false,
				regNum:      2,
			},
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, id string, callback func()) error {
						callback()
						return nil
					},
				},
				props: []*ScheduledTaskProps{
					{
						schedule: "@daily",
					},
				},
				updateError: true,
				regNum:      0,
			},
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						return nil
					},
				},
				props: []*ScheduledTaskProps{
					{
						schedule: "",
					},
				},
				updateError: false,
				regNum:      0,
			},
			{
				configWatcher: &DummyConfigWatcher{
					WatchFunc: func(_ context.Context, _ BotType, _ string, _ func()) error {
						return errors.New("subscription error")
					},
				},
				props: []*ScheduledTaskProps{
					{
						schedule: "@daily",
					},
				},
				updateError: false,
				regNum:      1,
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				regNum := 0
				botType := BotType(fmt.Sprintf("bot%d", i))
				bot := &DummyBot{
					BotTypeValue: botType,
				}
				r := &runner{
					configWatcher: tt.configWatcher,
					scheduledTaskProps: map[BotType][]*ScheduledTaskProps{
						botType: tt.props,
					},
					scheduledTasks: map[BotType][]ScheduledTask{
						botType: tt.tasks,
					},
					scheduler: &DummyScheduler{
						UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
							if tt.updateError {
								return errors.New("update error")
							}
							regNum++
							return nil
						},
						RemoveFunc: func(_ BotType, _ string) {},
					},
				}

				r.registerScheduledTasks(context.TODO(), bot)

				if tt.regNum != regNum {
					t.Errorf("Unexpected number of task registration call: %d.", regNum)
				}
			})
		}
	})
}
