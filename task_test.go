package sarah

import (
	"golang.org/x/net/context"
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

func (config *DummyScheduledTaskConfig) Schedule() string {
	return config.ScheduleValue
}

func (config *DummyScheduledTaskConfig) DefaultDestination() OutputDestination {
	return config.DestinationValue
}

func TestNewScheduledTaskBuilder(t *testing.T) {
	builder := NewScheduledTaskBuilder()

	if builder == nil {
		t.Fatal("Returned builder instance is nil.")
	}
}

func TestScheduledTaskBuilder_Identifier(t *testing.T) {
	id := "overWhelmedWithTasks"
	builder := &ScheduledTaskBuilder{}
	builder.Identifier(id)

	if builder.identifier != id {
		t.Fatal("Supplied id is not set.")
	}
}

func TestScheduledTaskBuilder_Func(t *testing.T) {
	res := "dummyResponse"
	taskFunc := func(_ context.Context) ([]*ScheduledTaskResult, error) {
		return []*ScheduledTaskResult{{Content: res}}, nil
	}
	builder := &ScheduledTaskBuilder{}
	builder.Func(taskFunc)

	actualRes, err := builder.taskFunc(context.TODO())
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if actualRes[0].Content != res {
		t.Fatal("Supplied func is not set.")
	}
}

func TestScheduledTaskBuilder_Schedule(t *testing.T) {
	schedule := "@daily"
	builder := &ScheduledTaskBuilder{}
	builder.Schedule(schedule)

	if builder.schedule != schedule {
		t.Fatal("Supplied schedule is not set.")
	}
}

func TestScheduledTaskBuilder_DefaultDestination(t *testing.T) {
	destination := "dest"
	builder := &ScheduledTaskBuilder{}
	builder.DefaultDestination(destination)

	if builder.defaultDestination != destination {
		t.Fatal("Supplied destination is not set.")
	}
}

func TestScheduledTaskBuilder_ConfigurableFunc(t *testing.T) {
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
	builder := &ScheduledTaskBuilder{}
	builder.ConfigurableFunc(config, taskFunc)

	if builder.config != config {
		t.Fatal("Supplied config is not set.")
	}
	if builder.taskFunc == nil {
		t.Fatal("Supplied function is not set.")
	}

	_, err := builder.taskFunc(context.TODO(), config)
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}
}

func TestScheduledTaskBuilder_Build(t *testing.T) {
	// Schedule is manually set at first.
	dummySchedule := "dummy"
	dummyDestination := "foo"
	config := &DummyScheduledTaskConfig{ScheduleValue: dummySchedule, DestinationValue: dummyDestination}
	taskFunc := func(_ context.Context, _ TaskConfig) ([]*ScheduledTaskResult, error) {
		return nil, nil
	}

	builder := &ScheduledTaskBuilder{}
	_, err := builder.Build("dummy")
	if err != ErrTaskInsufficientArgument {
		t.Fatalf("Expected error is not returned: %#v.", err)
	}

	id := "scheduled"
	builder.Identifier(id)
	builder.ConfigurableFunc(config, taskFunc)

	// When corresponding configuration file is not found, then manually set schedule must stay.
	_, err = builder.Build("no/corresponding/config")
	if err != nil {
		t.Fatal("Error on task construction with no config file.")
	}
	if config.Schedule() != dummySchedule {
		t.Errorf("Config value changed: %s.", config.DefaultDestination())
	}

	task, err := builder.Build("testdata/taskbuilder")

	if err != nil {
		t.Fatalf("Error returned on task build: %#v.", err)
	}

	if task.Identifier() != id {
		t.Errorf("Supplied id is not returned: %s", task.Identifier())
	}

	if builder.taskFunc == nil {
		t.Fatal("Supplied func is not set.")
	}

	if config.Schedule() == dummySchedule {
		t.Errorf("Config value is not overridden: %s.", config.DefaultDestination())
	}

	if task.DefaultDestination() != dummyDestination {
		t.Errorf("DefaultDestination value is not overridden: %s.", config.DefaultDestination())
	}
}

func TestScheduledTaskBuilder_MustBuild(t *testing.T) {
	builder := &ScheduledTaskBuilder{}
	builder.Identifier("dummy").
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
	command := builder.MustBuild()
	if command.Identifier() != builder.identifier {
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
