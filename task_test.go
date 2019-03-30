package sarah

import (
	"golang.org/x/net/context"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
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

func (config *DummyScheduledTaskConfig) Schedule() string {
	return config.ScheduleValue
}

func (config *DummyScheduledTaskConfig) DefaultDestination() OutputDestination {
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

func Test_buildScheduledTask_WithOutConfig(t *testing.T) {
	schedule := "@every 1m"
	props := &ScheduledTaskProps{
		botType:            "botType",
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           schedule,
		defaultDestination: "dummy",
		config:             nil,
	}

	task, err := buildScheduledTask(props, nil)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if task.Schedule() != schedule {
		t.Errorf("Schedule is not properly set: %s.", task.Schedule())
	}
}

func Test_buildScheduledTask_WithOutConfigOrSchedule(t *testing.T) {
	props := &ScheduledTaskProps{
		botType:            "botType",
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           "",
		defaultDestination: "dummy",
		config:             nil,
	}

	_, err := buildScheduledTask(props, nil)

	if err == nil {
		t.Fatalf("Expected error is not returned.")
	}
}

func Test_buildScheduledTask_WithOutConfigFile(t *testing.T) {
	config := &struct {
		Token string
	}{
		Token: "presetToken",
	}
	props := &ScheduledTaskProps{
		botType:            "botType",
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           "@every 1m",
		defaultDestination: "dummy",
		config:             config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "fileNotFound.yaml"),
		fileType: yamlFile,
	}

	_, err := buildScheduledTask(props, file)

	if err == nil {
		t.Fatalf("Error should be returned when expecting config file is not located")
	}
}

func Test_buildScheduledTask_WithBrokenYaml(t *testing.T) {
	config := &struct {
		Token string `yaml:"token"`
	}{
		Token: "",
	}
	props := &ScheduledTaskProps{
		identifier:         "broken",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           "@every 1m",
		defaultDestination: "dummy",
		config:             config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "broken.yaml"),
		fileType: yamlFile,
	}

	_, err := buildScheduledTask(props, file)

	if err == nil {
		t.Fatal("Error must be returned.")
	}
}

func Test_buildScheduledTask_WithOutSchedule(t *testing.T) {
	emptyScheduleConfig := &DummyScheduledTaskConfig{}
	emptyScheduleProps := &ScheduledTaskProps{
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		defaultDestination: "dummy",
		config:             emptyScheduleConfig,
	}

	_, err := buildScheduledTask(emptyScheduleProps, nil)

	if err == nil {
		t.Fatal("Epected error is not returned.")
	}

	if err != ErrTaskScheduleNotGiven {
		t.Fatalf("Expected error is not returned: %#v.", err.Error())
	}
}

func Test_buildScheduledTask_WithDefaultDestinationConfig(t *testing.T) {
	config := &DummyScheduledTaskConfig{
		ScheduleValue:    "@every 1m",
		DestinationValue: "dummy",
	}
	props := &ScheduledTaskProps{
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           "",
		defaultDestination: "",
		config:             config,
	}

	task, err := buildScheduledTask(props, nil)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if task.DefaultDestination() != config.DestinationValue {
		t.Fatalf("Expected default destination is returned: %s.", task.DefaultDestination())
	}
}

func Test_buildScheduledTask_WithConfigValue(t *testing.T) {
	type config struct {
		Token string
	}
	value := config{
		Token: "presetToken",
	}
	props := &ScheduledTaskProps{
		botType:            "botType",
		identifier:         "fileNotFound",
		taskFunc:           func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) { return nil, nil },
		schedule:           "@every 1m",
		defaultDestination: "dummy",
		config:             value,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	task, err := buildScheduledTask(props, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s", err.Error())
	}

	v := task.(*scheduledTask).configWrapper.value.(config).Token
	if v != "foobar" {
		t.Errorf("Newly set config does not reflect value from file: %s.", v)
	}
}

// Test_race_commandRebuild is an integration test to detect race condition on Command (re-)build.
func Test_race_taskRebuild(t *testing.T) {
	// Prepare TaskConfig
	type config struct {
		Token string
	}
	props, err := NewScheduledTaskPropsBuilder().
		Identifier("dummy").
		BotType("dummyBot").
		ConfigurableFunc(&config{Token: "default"}, func(_ context.Context, givenConfig TaskConfig) ([]*ScheduledTaskResult, error) {
			_, _ = ioutil.Discard.Write([]byte(givenConfig.(*config).Token)) // Read access to config struct
			return nil, nil
		}).
		Schedule("@every 1m").
		DefaultDestination("").
		Build()
	if err != nil {
		t.Fatalf("Error on ScheduledTaskProps preparation: %s.", err.Error())
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	task, err := buildScheduledTask(props, file)
	if err != nil {
		t.Fatalf("Error on ScheduledTask build: %s.", err.Error())
	}

	// Continuously read configuration file and re-build Command
	go func(c context.Context, p *ScheduledTaskProps) {
		for {
			select {
			case <-c.Done():
				return

			default:
				// Write
				_, err := buildScheduledTask(p, file)
				if err != nil {
					t.Errorf("Error on command build: %s.", err.Error())
				}
			}
		}
	}(ctx, props)

	// Continuously read config struct's field value by calling ScheduledTask.Execute
	go func(c context.Context) {
		for {
			select {
			case <-c.Done():
				return

			default:
				_, _ = task.Execute(ctx)

			}
		}
	}(ctx)

	// Wait till race condition occurs
	time.Sleep(1 * time.Second)
	cancel()
}
