package sarah

import (
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"os"
	"reflect"
	"sync"
)

var (
	// ErrTaskInsufficientArgument is returned when required parameters are not set.
	ErrTaskInsufficientArgument = errors.New("BotType, Identifier and Func must be set.")

	// ErrTaskScheduleNotGiven is returned when schedule is provided by neither ScheduledTaskPropsBuilder's parameter nor config.
	ErrTaskScheduleNotGiven = errors.New("Task schedule is not set or given from config struct.")
)

// ScheduledTaskResult is a struct that ScheduledTask returns on its execution.
type ScheduledTaskResult struct {
	Content     interface{}
	Destination OutputDestination
}

// taskFunc is a function type that represents scheduled task.
type taskFunc func(context.Context, ...TaskConfig) ([]*ScheduledTaskResult, error)

// TaskConfig provides an interface that every task configuration must satisfy, which actually means empty.
type TaskConfig interface{}

// ScheduledConfig defines an interface that config with schedule MUST satisfy.
// When no execution schedule is set with ScheduledTaskPropsBuilder.Schedule, this value is taken as default on ScheduledTaskPropsBuilder.Build.
type ScheduledConfig interface {
	Schedule() string
}

// DestinatedConfig defines an interface that config with default destination MUST satisfy.
// When no default output destination is set with ScheduledTaskPropsBuilder.DefaultDestination, this value is taken as default on ScheduledTaskPropsBuilder.Build.
type DestinatedConfig interface {
	DefaultDestination() OutputDestination
}

// ScheduledTask defines interface that all scheduled task MUST satisfy.
// As long as a struct satisfies this interface, the struct can be registered as ScheduledTask via Runner.RegisterScheduledTask.
type ScheduledTask interface {
	Identifier() string
	Execute(context.Context) ([]*ScheduledTaskResult, error)
	DefaultDestination() OutputDestination
	Schedule() string
}

type taskConfigWrapper struct {
	value TaskConfig
	mutex *sync.RWMutex
}

type scheduledTask struct {
	identifier         string
	taskFunc           taskFunc
	schedule           string
	defaultDestination OutputDestination
	configWrapper      *taskConfigWrapper
}

// Identifier returns unique ID of this task.
func (task *scheduledTask) Identifier() string {
	return task.identifier
}

// Execute executes scheduled task and returns *slice* of results.
//
// Note that scheduled task may result in sending messages to multiple destinations;
// sending taking-out-trash alarm to #dady-chores room while sending go-to-school alarm to #daughter room.
//
// When sending messages to multiple destinations, create and return results as many as destinations.
// If output destination is nil, caller tries to find corresponding destination from config struct.
func (task *scheduledTask) Execute(ctx context.Context) ([]*ScheduledTaskResult, error) {
	wrapper := task.configWrapper
	if wrapper == nil {
		return task.taskFunc(ctx)
	}

	// If the ScheduledTask has configuration struct, lock before execution.
	// Config struct may be updated on configuration file change.
	wrapper.mutex.RLock()
	defer wrapper.mutex.RUnlock()
	return task.taskFunc(ctx, wrapper.value)
}

// Schedule returns execution schedule.
func (task *scheduledTask) Schedule() string {
	return task.schedule
}

// DefaultDestination returns the default output destination of this task.
// OutputDestination returned by task execution has higher priority.
func (task *scheduledTask) DefaultDestination() OutputDestination {
	return task.defaultDestination
}

func buildScheduledTask(props *ScheduledTaskProps, file *pluginConfigFile) (ScheduledTask, error) {
	taskConfig := props.config
	if taskConfig == nil {
		// If config struct is not set, props MUST provide settings to set schedule.
		if props.schedule == "" {
			return nil, ErrTaskScheduleNotGiven
		}

		dest := props.defaultDestination // Can be nil because task response may return specific destination to send result to.
		return &scheduledTask{
			identifier:         props.identifier,
			taskFunc:           props.taskFunc,
			schedule:           props.schedule,
			defaultDestination: dest,
			configWrapper:      nil,
		}, nil
	}

	// https://github.com/oklahomer/go-sarah/issues/44
	//
	// Because config file can be created later and that may trigger config struct's live update,
	// locker needs to be obtained and passed to ScheduledTask to avoid concurrent read/write.
	// Until then, assume given TaskConfig is already configured and proceed to build.
	locker := configLocker.get(props.botType, props.identifier)
	if file != nil {
		// Minimize the scope of mutex with anonymous function for better performance.
		err := func() error {
			locker.Lock()
			defer locker.Unlock()

			rv := reflect.ValueOf(taskConfig)
			if rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Map {
				return updatePluginConfig(file, taskConfig)
			}

			// https://groups.google.com/forum/#!topic/Golang-Nuts/KB3_Yj3Ny4c
			// Obtain a pointer to the *underlying type* instead of not sarah.CommandConfig.
			n := reflect.New(reflect.TypeOf(taskConfig))

			// Copy the current value to newly created instance.
			// This includes private field values.
			n.Elem().Set(rv)

			// Pass the pointer to the newly created instance.
			e := updatePluginConfig(file, n.Interface())

			// Replace the current value with updated value.
			taskConfig = n.Elem().Interface()
			return e
		}()
		if err != nil && os.IsNotExist(err) {
			return nil, fmt.Errorf("Config file property was given, but failed to locate it: %s", err.Error())
		} else if err != nil {
			// File was there, but could not read.
			return nil, err
		}
	}

	// Setup execution schedule
	schedule := props.schedule
	if scheduledConfig, ok := (taskConfig).(ScheduledConfig); ok {
		if s := scheduledConfig.Schedule(); s != "" {
			schedule = s
		}
	}
	if schedule == "" {
		return nil, ErrTaskScheduleNotGiven
	}

	// Setup default destination
	// This can be nil since each task execution may return corresponding destination
	dest := props.defaultDestination
	if destConfig, ok := (taskConfig).(DestinatedConfig); ok {
		if d := destConfig.DefaultDestination(); d != nil {
			dest = d
		}
	}

	return &scheduledTask{
		identifier:         props.identifier,
		taskFunc:           props.taskFunc,
		schedule:           schedule,
		defaultDestination: dest,
		configWrapper: &taskConfigWrapper{
			value: taskConfig,
			mutex: locker,
		},
	}, nil
}

// ScheduledTaskProps is a designated non-serializable configuration struct to be used in ScheduledTask construction.
// This holds relatively complex set of ScheduledTask construction arguments that should be treated as one in logical term.
type ScheduledTaskProps struct {
	botType            BotType
	identifier         string
	taskFunc           taskFunc
	schedule           string
	defaultDestination OutputDestination
	config             TaskConfig
}

// ScheduledTaskPropsBuilder helps to construct ScheduledTaskProps.
// Developer may set desired property as she goes and call ScheduledTaskPropsBuilder.Build or ScheduledTaskPropsBuilder.MustBuild to construct ScheduledTaskProps at the end.
// A validation logic runs on build, so the returning ScheduledTaskProps instant is safe to be passed to Runner.
type ScheduledTaskPropsBuilder struct {
	props *ScheduledTaskProps
}

// NewScheduledTaskPropsBuilder creates and returns ScheduledTaskPropsBuilder instance.
func NewScheduledTaskPropsBuilder() *ScheduledTaskPropsBuilder {
	return &ScheduledTaskPropsBuilder{
		props: &ScheduledTaskProps{},
	}
}

// BotType is a setter to provide belonging BotType.
func (builder *ScheduledTaskPropsBuilder) BotType(botType BotType) *ScheduledTaskPropsBuilder {
	builder.props.botType = botType
	return builder
}

// Identifier sets unique ID of this task.
// This is used to identify re-configure tasks and replace old ones.
func (builder *ScheduledTaskPropsBuilder) Identifier(id string) *ScheduledTaskPropsBuilder {
	builder.props.identifier = id
	return builder
}

// Func sets function to be called on task execution.
// To set function that requires some sort of configuration struct, use ConfigurableFunc.
func (builder *ScheduledTaskPropsBuilder) Func(fn func(context.Context) ([]*ScheduledTaskResult, error)) *ScheduledTaskPropsBuilder {
	builder.props.config = nil
	builder.props.taskFunc = func(ctx context.Context, cfg ...TaskConfig) ([]*ScheduledTaskResult, error) {
		return fn(ctx)
	}
	return builder
}

// Schedule sets execution schedule.
// Representation spec. is identical to that of github.com/robfig/cron.
func (builder *ScheduledTaskPropsBuilder) Schedule(schedule string) *ScheduledTaskPropsBuilder {
	builder.props.schedule = schedule
	return builder
}

// DefaultDestination sets default output destination of this task.
// OutputDestination returned by task execution has higher priority.
func (builder *ScheduledTaskPropsBuilder) DefaultDestination(dest OutputDestination) *ScheduledTaskPropsBuilder {
	builder.props.defaultDestination = dest
	return builder
}

// ConfigurableFunc sets function for ScheduledTask with configuration struct.
// Passed configuration struct is passed to function as a third argument.
//
// When resulting ScheduledTaskProps is passed Runner.New as part of sarah.WithScheduledTaskProps and Runner runs with Config.PluginConfigRoot,
// configuration struct gets updated automatically when corresponding configuration file is updated.
func (builder *ScheduledTaskPropsBuilder) ConfigurableFunc(config TaskConfig, fn func(context.Context, TaskConfig) ([]*ScheduledTaskResult, error)) *ScheduledTaskPropsBuilder {
	builder.props.config = config
	builder.props.taskFunc = func(ctx context.Context, cfg ...TaskConfig) ([]*ScheduledTaskResult, error) {
		return fn(ctx, cfg[0])
	}
	return builder
}

// Build builds new ScheduledProps instance with provided values.
func (builder *ScheduledTaskPropsBuilder) Build() (*ScheduledTaskProps, error) {
	if builder.props.botType == "" ||
		builder.props.identifier == "" ||
		builder.props.taskFunc == nil {

		return nil, ErrTaskInsufficientArgument
	}

	taskConfig := builder.props.config
	if taskConfig == nil && builder.props.schedule == "" {
		// Task Schedule can never be specified.
		return nil, ErrTaskScheduleNotGiven
	}

	if taskConfig != nil {
		if _, ok := (taskConfig).(ScheduledConfig); !ok && builder.props.schedule == "" {
			// Task Schedule can never be specified.
			return nil, ErrTaskScheduleNotGiven
		}
	}

	return builder.props, nil
}

// MustBuild is like Build, but panics if any error occurs on Build.
// It simplifies safe initialization of global variables holding built ScheduledTaskProps instances.
func (builder *ScheduledTaskPropsBuilder) MustBuild() *ScheduledTaskProps {
	task, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("Error on building ScheduledTaskProps: %s", err.Error()))
	}

	return task
}
