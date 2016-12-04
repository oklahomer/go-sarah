package sarah

import (
	"golang.org/x/net/context"
	"reflect"
	"testing"
)

type DummyScheduledTaskConfig struct {
	ScheduleValue    string `yaml:"schedule"`
	DestinationValue OutputDestination
}

func (config *DummyScheduledTaskConfig) Schedule() string {
	return config.ScheduleValue
}

func (config *DummyScheduledTaskConfig) Destination() OutputDestination {
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
	taskFunc := func(_ context.Context, _ ScheduledTaskConfig) (*CommandResponse, error) {
		return nil, nil
	}
	builder := &ScheduledTaskBuilder{}
	builder.Func(taskFunc)

	if reflect.ValueOf(builder.taskFunc).Pointer() != reflect.ValueOf(taskFunc).Pointer() {
		t.Fatal("Supplied func is not set.")
	}
}

func TestScheduledTaskBuilder_ConfigStruct(t *testing.T) {
	config := &DummyScheduledTaskConfig{}
	builder := &ScheduledTaskBuilder{}
	builder.ConfigStruct(config)

	if builder.config != config {
		t.Fatal("Supplied config is not set.")
	}
}

func TestScheduledTaskBuilder_Build(t *testing.T) {
	id := "scheduled"
	config := &DummyScheduledTaskConfig{}
	taskFunc := func(_ context.Context, _ ScheduledTaskConfig) (*CommandResponse, error) {
		return nil, nil
	}

	builder := &ScheduledTaskBuilder{}
	_, err := builder.Build("dummy")
	if err != ErrTaskInsufficientArgument {
		t.Fatalf("Expected error is not returned: %#v.", err)
	}

	builder.Identifier(id)
	builder.ConfigStruct(config)
	builder.Func(taskFunc)

	_, err = builder.Build("invalidPath")
	if err == nil {
		t.Fatal("Error is expected, but is not returned.")
	}

	task, err := builder.Build("testdata/taskbuilder")

	if err != nil {
		t.Fatalf("Error returned on task build: %#v.", err)
	}

	if task.Identifier() != id {
		t.Errorf("Supplied id is not returned: %s", task.Identifier())
	}

	if reflect.ValueOf(builder.taskFunc).Pointer() != reflect.ValueOf(taskFunc).Pointer() {
		t.Fatal("Supplied func is not set.")
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
	taskFunc := func(_ context.Context, _ ScheduledTaskConfig) (*CommandResponse, error) {
		return &CommandResponse{Content: returningContent}, nil
	}
	task := &scheduledTask{taskFunc: taskFunc}
	response, err := task.Execute(context.TODO())

	if err != nil {
		t.Fatalf("Unexpected error is returned: %#v.", err)
	}

	if response.Content != returningContent {
		t.Errorf("Unexpected content is returned: %s", response.Content)
	}
}
