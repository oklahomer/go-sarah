package sarah

import (
	"context"
	"golang.org/x/xerrors"
	"strconv"
	"testing"
)

type DummyScheduledTask struct {
	IdentifierValue         string
	ExecuteFunc             func(context.Context) ([]*ScheduledTaskResult, error)
	DefaultDestinationValue OutputDestination
	ScheduleValue           string
}

func (s *DummyScheduledTask) Identifier() string {
	return s.IdentifierValue
}

func (s *DummyScheduledTask) Execute(ctx context.Context) ([]*ScheduledTaskResult, error) {
	return s.ExecuteFunc(ctx)
}

func (s *DummyScheduledTask) DefaultDestination() OutputDestination {
	return s.DefaultDestinationValue
}

func (s *DummyScheduledTask) Schedule() string {
	return s.ScheduleValue
}

type DummyScheduledTaskConfig struct {
	ScheduleValue    string `yaml:"schedule"`
	DestinationValue OutputDestination
}

func (config DummyScheduledTaskConfig) Schedule() string {
	return config.ScheduleValue
}

func (config DummyScheduledTaskConfig) DefaultDestination() OutputDestination {
	return config.DestinationValue
}

func TestNewScheduledTaskPropsBuilder(t *testing.T) {
	builder := NewScheduledTaskPropsBuilder()

	if builder == nil {
		t.Fatal("Returned builder instance is nil.")
	}
}

func TestScheduledTaskPropsBuilder_BotType(t *testing.T) {
	var botType BotType = "dummy"
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.BotType(botType)

	if builder.props.botType != botType {
		t.Error("Provided BotType was not set.")
	}
}

func TestScheduledTaskPropsBuilder_Identifier(t *testing.T) {
	id := "overWhelmedWithTasks"
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.Identifier(id)

	if builder.props.identifier != id {
		t.Fatal("Supplied id is not set.")
	}
}

func TestScheduledTaskPropsBuilder_Func(t *testing.T) {
	res := "dummyResponse"
	taskFunc := func(_ context.Context) ([]*ScheduledTaskResult, error) {
		return []*ScheduledTaskResult{{Content: res}}, nil
	}
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.Func(taskFunc)

	actualRes, err := builder.props.taskFunc(context.TODO())
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if actualRes[0].Content != res {
		t.Fatal("Supplied func is not set.")
	}
}

func TestScheduledTaskPropsBuilder_Schedule(t *testing.T) {
	schedule := "@daily"
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.Schedule(schedule)

	if builder.props.schedule != schedule {
		t.Fatal("Supplied schedule is not set.")
	}
}

func TestScheduledTaskPropsBuilder_DefaultDestination(t *testing.T) {
	destination := "dest"
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.DefaultDestination(destination)

	if builder.props.defaultDestination != destination {
		t.Fatal("Supplied destination is not set.")
	}
}

func TestScheduledTaskPropsBuilder_ConfigurableFunc(t *testing.T) {
	config := &DummyScheduledTaskConfig{}
	taskFunc := func(_ context.Context, c TaskConfig) ([]*ScheduledTaskResult, error) {
		if _, ok := c.(*DummyScheduledTaskConfig); !ok {
			t.Errorf("Unexpected config type is given %#v.", c)
		}
		return []*ScheduledTaskResult{
			{
				Content: "foo",
			},
		}, nil
	}
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.ConfigurableFunc(config, taskFunc)

	if builder.props.config != config {
		t.Fatal("Supplied config is not set.")
	}
	if builder.props.taskFunc == nil {
		t.Fatal("Supplied function is not set.")
	}

	_, err := builder.props.taskFunc(context.TODO(), config)
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}
}

func TestScheduledTaskPropsBuilder_Build(t *testing.T) {
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	_, err := builder.Build()
	if err != ErrTaskInsufficientArgument {
		t.Fatalf("Expected error is not returned: %#v.", err)
	}

	var botType BotType = "dummyBot"
	builder.BotType(botType)
	id := "scheduled"
	builder.Identifier(id)
	builder.Func(func(_ context.Context) ([]*ScheduledTaskResult, error) {
		return nil, nil
	})
	builder.Schedule("@hourly")

	props, err := builder.Build()

	if err != nil {
		t.Fatalf("Error returned on task build: %#v.", err)
	}

	if props.botType != botType {
		t.Errorf("Supplied BotType is not returned: %s", props.botType)
	}

	if props.identifier != id {
		t.Errorf("Supplied id is not returned: %s", props.identifier)
	}

	if props.taskFunc == nil {
		t.Fatal("Supplied func is not set.")
	}
}

func TestScheduledTaskPropsBuilder_Build_WithUnconfigurableSchedule(t *testing.T) {
	nonScheduledConfig := &struct {
		Token string `yaml:"token"`
	}{
		Token: "",
	}
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.BotType("dummyBot").
		Identifier("dummyId").
		ConfigurableFunc(nonScheduledConfig, func(_ context.Context, _ TaskConfig) ([]*ScheduledTaskResult, error) {
			return nil, nil
		})

	props, err := builder.Build()

	if props != nil {
		t.Error("Built ScheduledTaskProps should not be returned.")
	}

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if err != ErrTaskScheduleNotGiven {
		t.Fatalf("Exected error is not returned: %s.", err.Error())
	}
}

func TestScheduledTaskPropsBuilder_MustBuild(t *testing.T) {
	builder := &ScheduledTaskPropsBuilder{props: &ScheduledTaskProps{}}
	builder.BotType("dummyBot").
		Identifier("dummy").
		Func(func(_ context.Context) ([]*ScheduledTaskResult, error) {
			return nil, nil
		})

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic did not occur.")
			}
		}()
		builder.MustBuild()
	}()

	builder.Schedule("@daily")
	props := builder.MustBuild()
	if props.identifier != builder.props.identifier {
		t.Error("Provided identifier is not set.")
	}
}

func TestScheduledTask_Identifier(t *testing.T) {
	id := "myTask"
	task := &scheduledTask{identifier: id}

	if task.Identifier() != id {
		t.Fatalf("ID is not properly returned: %s.", task.Identifier())
	}
}

func TestScheduledTask_Execute(t *testing.T) {
	returningContent := "abc"
	taskFunc := func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) {
		return []*ScheduledTaskResult{
			{Content: returningContent},
		}, nil
	}
	task := &scheduledTask{taskFunc: taskFunc}
	results, err := task.Execute(context.TODO())

	if err != nil {
		t.Fatalf("Unexpected error is returned: %#v.", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result to return, but was: %d.", len(results))

	}
	if results[0].Content != returningContent {
		t.Errorf("Unexpected content is returned: %s", results[0].Content)
	}
}

func TestScheduledTask_DefaultDestination(t *testing.T) {
	destination := "dest"
	task := &scheduledTask{defaultDestination: destination}

	if task.DefaultDestination() != destination {
		t.Fatalf("Returned destination differs: %s.", task.DefaultDestination())
	}
}

func TestScheduledTask_Schedule(t *testing.T) {
	schedule := "@daily"
	task := &scheduledTask{schedule: schedule}

	if task.Schedule() != schedule {
		t.Fatalf("Returned schedule differs: %s.", task.Schedule())
	}
}

func Test_buildScheduledTask(t *testing.T) {
	tests := []struct {
		props          *ScheduledTaskProps
		watcher        ConfigWatcher
		validateConfig func(cfg interface{}) error
		hasErr         bool
	}{
		{
			// No config or pre-defined schedule
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "",
				defaultDestination: "dummy",
				config:             nil,
			},
			watcher: nil,
			hasErr:  true,
		},
		{
			// No config, but schedule
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "@daily",
				defaultDestination: "dummy",
				config:             nil,
			},
			watcher: nil,
			hasErr:  false,
		},
		{
			// No schedule can be fetched from pre-defined value or config
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "",
				defaultDestination: "dummy",
				config:             &DummyScheduledTaskConfig{},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
					return nil
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(*DummyScheduledTaskConfig)
				if !ok {
					return xerrors.Errorf("Unexpected type is passed: %T.", cfg)
				}

				if config.ScheduleValue != "" {
					return xerrors.Errorf("Unexpected value is set: %s", config.ScheduleValue)
				}

				return nil
			},
			hasErr: true,
		},
		{
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "@daily",
				defaultDestination: "dummy",
				config: &DummyScheduledTaskConfig{
					ScheduleValue:    "",
					DestinationValue: "",
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, cfg interface{}) error {
					config, ok := cfg.(*DummyScheduledTaskConfig)
					if !ok {
						t.Errorf("Unexpected type is passed: %T.", cfg)
						return nil
					}

					config.ScheduleValue = "@every 1m"
					return nil
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(*DummyScheduledTaskConfig)
				if !ok {
					return xerrors.Errorf("Unexpected type is passed: %T.", cfg)
				}

				if config.ScheduleValue != "@every 1m" {
					return xerrors.Errorf("Unexpected value is set: %s", config.ScheduleValue)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "@daily",
				defaultDestination: "dummy",
				config: &DummyScheduledTaskConfig{
					ScheduleValue:    "",
					DestinationValue: "",
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, botType BotType, id string, cfg interface{}) error {
					return &ConfigNotFoundError{
						BotType: botType,
						ID:      id,
					}
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(*DummyScheduledTaskConfig)
				if !ok {
					return xerrors.Errorf("Unexpected type is passed: %T.", cfg)
				}

				if config.ScheduleValue != "" {
					return xerrors.Errorf("Unexpected value is set: %s", config.ScheduleValue)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "",
				defaultDestination: "dummy",
				// Not a pointer to the config value, but is well handled
				config: DummyScheduledTaskConfig{
					ScheduleValue:    "",
					DestinationValue: "",
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, botType BotType, id string, cfg interface{}) error {
					config, ok := cfg.(*DummyScheduledTaskConfig) // Pointer is passed
					if !ok {
						t.Errorf("Unexpected type is passed: %T.", cfg)
						return nil
					}

					config.ScheduleValue = "@every 1m"
					return nil
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(DummyScheduledTaskConfig) // Value is passed
				if !ok {
					return xerrors.Errorf("Unexpected type is passed: %T.", cfg)
				}

				if config.ScheduleValue != "@every 1m" {
					return xerrors.Errorf("Unexpected value is set: %s", config.ScheduleValue)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &ScheduledTaskProps{
				botType:            "botType",
				identifier:         "fileNotFound",
				taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
				schedule:           "",
				defaultDestination: "dummy",
				config: &DummyScheduledTaskConfig{
					ScheduleValue:    "",
					DestinationValue: "",
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
					return xerrors.New("unacceptable error")
				},
			},
			hasErr: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			task, err := buildScheduledTask(context.TODO(), tt.props, tt.watcher)
			if tt.hasErr {
				if err == nil {
					t.Error("Expected error is not returned.")
				}
				return
			}

			if task == nil {
				t.Fatal("Built task is not returned.")
			}

			if tt.props.config != nil {
				typed := task.(*scheduledTask)
				err = tt.validateConfig(typed.configWrapper.value)
				if err != nil {
					t.Error(err.Error())
				}
			}
		})
	}
}

//// Test_race_commandRebuild is an integration test to detect race condition on Command (re-)build.
//func Test_race_taskRebuild(t *testing.T) {
//	// Prepare TaskConfig
//	type config struct {
//		Token string
//	}
//	props, err := NewScheduledTaskPropsBuilder().
//		Identifier("dummy").
//		BotType("dummyBot").
//		ConfigurableFunc(&config{Token: "default"}, func(_ context.Context, givenConfig TaskConfig) ([]*ScheduledTaskResult, error) {
//			_, _ = ioutil.Discard.Write([]byte(givenConfig.(*config).Token)) // Read access to config struct
//			return nil, nil
//		}).
//		Schedule("@every 1m").
//		DefaultDestination("").
//		Build()
//	if err != nil {
//		t.Fatalf("Error on ScheduledTaskProps preparation: %s.", err.Error())
//	}
//	file := &pluginConfigFile{
//		id:       props.identifier,
//		path:     filepath.Join("testdata", "command", "dummy.yaml"),
//		fileType: yamlFile,
//	}
//
//	rootCtx := context.Background()
//	ctx, cancel := context.WithCancel(rootCtx)
//
//	task, err := buildScheduledTask(props, file)
//	if err != nil {
//		t.Fatalf("Error on ScheduledTask build: %s.", err.Error())
//	}
//
//	// Continuously read configuration file and re-build Command
//	go func(c context.Context, p *ScheduledTaskProps) {
//		for {
//			select {
//			case <-c.Done():
//				return
//
//			default:
//				// Write
//				_, err := buildScheduledTask(p, file)
//				if err != nil {
//					t.Errorf("Error on command build: %s.", err.Error())
//				}
//			}
//		}
//	}(ctx, props)
//
//	// Continuously read config struct's field value by calling ScheduledTask.Execute
//	go func(c context.Context) {
//		for {
//			select {
//			case <-c.Done():
//				return
//
//			default:
//				_, _ = task.Execute(ctx)
//
//			}
//		}
//	}(ctx)
//
//	// Wait till race condition occurs
//	time.Sleep(1 * time.Second)
//	cancel()
//}
