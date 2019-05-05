package sarah

import (
	"errors"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"io/ioutil"
	stdLogger "log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := log.GetLogger()
	defer log.SetLogger(oldLogger)

	// Suppress log output in test by default
	l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
	logger := log.NewWithStandardLogger(l)
	log.SetLogger(logger)

	code := m.Run()

	os.Exit(code)
}

func SetupAndRun(fnc func()) {
	// Initialize package variables
	runnerStatus = &status{}
	options = &optionHolder{}

	fnc()
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

func Test_optionHolder_register(t *testing.T) {
	opt := func(_ *runner) error {
		return nil
	}
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
	expectedErr := errors.New("option application error")
	holder := &optionHolder{}
	holder.stashed = []func(*runner) error{
		func(_ *runner) error {
			called++
			return nil
		},
		func(_ *runner) error {
			called++
			return expectedErr
		},
	}
	r := &runner{}

	err := holder.apply(r)

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}
	if err != expectedErr {
		t.Errorf("Unexpected error is not returned: %s.", err.Error())
	}

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
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
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
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
		}

		if len(r.bots) != 1 {
			t.Fatalf("Expected number of bot is not registered: %d.", len(r.bots))
		}

		if r.bots[0] != bot {
			t.Error("Given bot is not registered.")
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
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
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
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
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
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
		}

		if len(r.scheduledTaskProps[botType]) != 1 {
			t.Fatalf("Expected number of ScheduledTaskProps is not registered: %d.", len(r.scheduledTaskProps[botType]))
		}

		if r.scheduledTaskProps[botType][0] != props {
			t.Error("Given ScheduledTaskProps is not registered.")
		}
	})
}

func TestRegisterWatcher(t *testing.T) {
	SetupAndRun(func() {
		watcher := &DummyWatcher{}
		RegisterWatcher(watcher)
		r := &runner{}

		for _, v := range options.stashed {
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
		}

		if r.watcher == nil {
			t.Fatal("Watcher is not set")
		}

		if r.watcher != watcher {
			t.Error("Given Watcher is not set.")
		}
	})
}

func TestRegisterWorker(t *testing.T) {
	SetupAndRun(func() {
		worker := &DummyWorker{}
		RegisterWorker(worker)
		r := &runner{}

		for _, v := range options.stashed {
			err := v(r)
			if err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}
		}

		if r.worker == nil {
			t.Fatal("Worker is not set")
		}
		if r.worker != worker {
			t.Error("Given Worker is not set.")
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
			TimeZone:         time.UTC.String(),
			PluginConfigRoot: "/not/empty",
		}

		r, e := newRunner(context.Background(), config)
		if e != nil {
			t.Fatalf("Unexpected error is returned: %s.", e.Error())
		}

		if r == nil {
			t.Fatal("runner instance is not returned.")
		}

		if r.watcher == nil {
			t.Error("Default Watcher should be set when PluginConfigRoot is not empty.")
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
			TimeZone:         "DUMMY",
			PluginConfigRoot: "/not/empty",
		}

		_, e := newRunner(context.Background(), config)
		if e == nil {
			t.Fatal("Expected error is not returned.")
		}
	})
}

func Test_newRunner_WithOptionError(t *testing.T) {
	SetupAndRun(func() {
		config := &Config{
			TimeZone:         time.UTC.String(),
			PluginConfigRoot: "/not/empty",
		}
		options.stashed = []func(*runner) error{
			func(_ *runner) error {
				return errors.New("dummy")
			},
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
			PluginConfigRoot: "",
			TimeZone:         time.Now().Location().String(),
		}

		r := &runner{
			config: config,
			bots: []Bot{
				bot,
			},
		}

		rootCtx := context.Background()
		ctx, cancel := context.WithCancel(rootCtx)
		go r.run(ctx)

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
		time.Sleep(1 * time.Second)

		if CurrentStatus().Bots[0].Running {
			t.Error("BotStatus.Running should not be true at this point.")
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
			PluginConfigRoot: "dummy/config",
			TimeZone:         time.Now().Location().String(),
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
			scheduler: &DummyScheduler{
				UpdateFunc: func(_ BotType, _ ScheduledTask, _ func()) error {
					return nil
				},
				RemoveFunc: func(_ BotType, _ string) error {
					return nil
				},
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
			PluginConfigRoot: "",
			TimeZone:         time.Now().Location().String(),
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
		runnerCtx, _ := context.WithCancel(rootCtx)
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

func Test_runner_subscribeConfigDir(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"
		subscribed := make(chan struct {
			botTypeStr string
			dir        string
		})
		unsubscribed := make(chan string)
		watcher := &DummyWatcher{
			SubscribeFunc: func(botTypeStr string, dir string, _ func(string)) error {
				subscribed <- struct {
					botTypeStr string
					dir        string
				}{
					botTypeStr: botTypeStr,
					dir:        dir,
				}
				return nil
			},
			UnsubscribeFunc: func(botTypeStr string) error {
				unsubscribed <- botTypeStr
				return nil
			},
		}

		r := &runner{
			watcher: watcher,
		}

		rootCtx := context.Background()
		ctx, cancel := context.WithCancel(rootCtx)

		bot := &DummyBot{
			BotTypeValue: botType,
		}

		configDir := "/path/to/config/dir"

		go r.subscribeConfigDir(ctx, bot, configDir)

		select {
		case s := <-subscribed:
			if s.botTypeStr != botType.String() {
				t.Errorf("Unexpected BotType is passed: %s.", s.botTypeStr)
			}
			if s.dir != configDir {
				t.Errorf("Unexpected directory string is passed: %s.", s.dir)
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Fatal("Subscribing directory data should be passed.")

		}

		cancel()

		select {
		case u := <-unsubscribed:
			if u != botType.String() {
				t.Fatalf("Unexpected BotType string is passed: %s.", u)
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Fatal("Unsubscribing directory data should be passed.")

		}
	})
}

func Test_runner_subscribeConfigDir_WithSubscriptionError(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"
		subscribed := make(chan struct{}, 1)
		watcher := &DummyWatcher{
			SubscribeFunc: func(botTypeStr string, dir string, _ func(string)) error {
				subscribed <- struct{}{}
				return errors.New("subscription error")
			},
		}

		r := &runner{
			watcher: watcher,
		}

		rootCtx := context.Background()
		ctx, cancel := context.WithCancel(rootCtx)
		defer cancel()

		bot := &DummyBot{
			BotTypeValue: botType,
		}

		configDir := "/path/to/config/dir"

		// Should not block this time
		r.subscribeConfigDir(ctx, bot, configDir)

		select {
		case <-subscribed:
			// O.K.

		default:
			t.Fatal("Watcher.Subscribe should be called.")

		}
	})
}

func Test_runner_subscribeConfigDir_WithUnsubscriptionError(t *testing.T) {
	SetupAndRun(func() {
		var botType BotType = "myBot"
		unsubscribed := make(chan struct{}, 1)
		watcher := &DummyWatcher{
			SubscribeFunc: func(botTypeStr string, dir string, _ func(string)) error {
				return nil
			},
			UnsubscribeFunc: func(_ string) error {
				unsubscribed <- struct{}{}
				return errors.New("unsubscription error")

			},
		}

		r := &runner{
			watcher: watcher,
		}

		rootCtx := context.Background()
		ctx, cancel := context.WithCancel(rootCtx)

		bot := &DummyBot{
			BotTypeValue: botType,
		}

		configDir := "/path/to/config/dir"
		go r.subscribeConfigDir(ctx, bot, configDir)
		time.Sleep(500 * time.Millisecond)
		cancel()

		select {
		case <-unsubscribed:
			// O.K.

		case <-time.NewTimer(500 * time.Millisecond).C:
			t.Fatal("Watcher.Unsubscribe should be called.")

		}
	})
}

func Test_runner_subscribeConfigDir_WithCallback(t *testing.T) {
	tests := []struct {
		isErr bool
		path  string
	}{
		{
			isErr: true,
			path:  "/invalid/file/extension",
		},
		{
			isErr: true,
			path:  "/unsupported/file/format.toml",
		},
		{
			isErr: false,
			path:  filepath.Join("testdata", "command", "dummy.json"),
		},
		{
			isErr: false,
			path:  filepath.Join("testdata", "command", "dummy.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			SetupAndRun(func() {
				var botType BotType = "myBot"
				callbackCalled := make(chan struct{}, 1)
				watcher := &DummyWatcher{
					SubscribeFunc: func(_ string, _ string, callback func(string)) error {
						callback(tt.path)
						callbackCalled <- struct{}{}
						return nil
					},
					UnsubscribeFunc: func(_ string) error {
						return nil
					},
				}
				r := &runner{
					watcher: watcher,
				}
				bot := &DummyBot{
					BotTypeValue: botType,
				}

				rootCtx := context.Background()
				ctx, cancel := context.WithCancel(rootCtx)

				go r.subscribeConfigDir(ctx, bot, "dummy")
				<-callbackCalled
				cancel()
			})
		})
	}
}

func Test_registerCommand(t *testing.T) {
	SetupAndRun(func() {
		command := &DummyCommand{}
		var appendedCommand Command
		bot := &DummyBot{AppendCommandFunc: func(cmd Command) { appendedCommand = cmd }}

		bot.AppendCommand(command)

		if appendedCommand != command {
			t.Error("Given Command is not appended.")
		}
	})
}

func Test_registerScheduledTask(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_updateCommandConfig(t *testing.T) {
	SetupAndRun(func() {
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
				instructionFunc: func(_ *HelpInput) string {
					return "instruction text"
				},
			},
			{
				identifier:  "dummy",
				botType:     botType,
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
				matchFunc:   func(_ Input) bool { return true },
				config:      c,
				instructionFunc: func(_ *HelpInput) string {
					return "instruction text"
				},
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
	})
}

func Test_updateCommandConfig_WithBrokenYaml(t *testing.T) {
	SetupAndRun(func() {
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
				instructionFunc: func(_ *HelpInput) string {
					return "instruction text"
				},
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
	})
}

func Test_updateCommandConfig_WithConfigValue(t *testing.T) {
	SetupAndRun(func() {
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
				instructionFunc: func(_ *HelpInput) string {
					return "instruction text"
				},
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
	})
}

func Test_updateScheduledTaskConfig(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_updateScheduledTaskConfig_WithBrokenYaml(t *testing.T) {
	SetupAndRun(func() {
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
	})
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

func Test_superviseBot(t *testing.T) {
	SetupAndRun(func() {
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
		botCtx, errSupervisor := superviseBot(rootCxt, "DummyBotType", alerters)

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

func Test_plainPathToFile(t *testing.T) {
	SetupAndRun(func() {
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
	})
}

func Test_findPluginConfigFile(t *testing.T) {
	SetupAndRun(func() {
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
	})
}
