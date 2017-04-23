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
