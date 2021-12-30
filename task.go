package sarah

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	// ErrTaskInsufficientArgument is returned when required parameters are not set.
	ErrTaskInsufficientArgument = errors.New("one or more of required fields -- BotType, Identifier and Func -- are empty")

	// ErrTaskScheduleNotGiven is returned when a schedule is provided by neither ScheduledTaskPropsBuilder's parameter nor config.
	ErrTaskScheduleNotGiven = errors.New("task schedule is not set or given from config struct")
)

// ScheduledTaskResult is a struct that ScheduledTask returns on its execution.
type ScheduledTaskResult struct {
	// Content represents a group of data to be sent as a result of task execution.
	// Since this is passed to Bot.SendMessage as part of OutputMessage,
	// its type may vary depending on the Bot's integrating chat service.
	Content interface{}

	// Destination is passed to Bot.SendMessage as part of OutputMessage value to specify the sending destination.
	// This typically contains a chat room, member id, or e-mail address.
	// e.g. JID of XMPP server/client.
	//
	// When this is nil, Sarah tries to fall back to a default destination given by ScheduledTask.
	// If no default destination is set, then the task execution is considered a failure.
	Destination OutputDestination
}

// taskFunc is a function type that represents a scheduled task.
type taskFunc func(context.Context, ...TaskConfig) ([]*ScheduledTaskResult, error)

// TaskConfig provides an interface that every task configuration must satisfy, which actually is empty.
// Think of this as a kind of marker interface with a more meaningful name.
type TaskConfig interface{}

// ScheduledConfig defines an interface that a configuration with a default schedule MUST satisfy.
// When no execution schedule is set with ScheduledTaskPropsBuilder.Schedule, this value is taken as a default value on ScheduledTaskPropsBuilder.Build.
type ScheduledConfig interface {
	Schedule() string
}

// DestinatedConfig defines an interface that a configuration with a default destination MUST satisfy.
// When no output destination is set with ScheduledTaskPropsBuilder.DefaultDestination, this value is taken as a default value on ScheduledTaskPropsBuilder.Build.
type DestinatedConfig interface {
	DefaultDestination() OutputDestination
}

// ScheduledTask defines an interface that all scheduled task MUST satisfy.
// As long as a struct satisfies this interface, the struct can be registered as ScheduledTask via RegisterScheduledTask.
//
// ScheduledTaskPropsBuilder and RegisterScheduledTaskProps to set up a ScheduledTask on the fly.
// That will give more flexibility such as the task rebuild feature on live configuration updates.
type ScheduledTask interface {
	// Identifier returns a unique id of this ScheduledTask.
	Identifier() string

	// Execute runs the scheduled task and returns the result in a form of slice.
	// When the task needs to send multiple payloads to multiple destinations, then return as many ScheduledTaskResult as the destinations.
	// Note that scheduled task may result in sending messages to multiple destinations.
	// e.g. Sending taking-out-trash alarm to #dady-chores room while sending go-to-school alarm to #daughter room.
	Execute(context.Context) ([]*ScheduledTaskResult, error)

	// DefaultDestination returns the default destination to send the result to.
	// When ScheduledTaskResult does not specify an output destination, Sarah falls back to use this value as a default.
	// If a default destination is nil, then the task execution is considered a failure.
	DefaultDestination() OutputDestination

	// Schedule returns the stringified representation of the execution schedule.
	// The schedule can be expressed in a crontab way with seconds precision such as "0 30 * * * *" but some variations are also available.
	// See https://pkg.go.dev/github.com/robfig/cron/v3 for details.
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

// Identifier returns unique id of this task.
func (task *scheduledTask) Identifier() string {
	return task.identifier
}

// Execute runs the scheduled task and returns the result in a form of slice.
func (task *scheduledTask) Execute(ctx context.Context) ([]*ScheduledTaskResult, error) {
	wrapper := task.configWrapper
	if wrapper == nil {
		return task.taskFunc(ctx)
	}

	// If the ScheduledTask has a configuration struct, lock before execution.
	// The config struct may be updated by ConfigWatcher at the same time.
	wrapper.mutex.RLock()
	defer wrapper.mutex.RUnlock()
	return task.taskFunc(ctx, wrapper.value)
}

// Schedule returns the stringified representation of the execution schedule.
func (task *scheduledTask) Schedule() string {
	return task.schedule
}

// DefaultDestination returns the default destination to send the result to.
func (task *scheduledTask) DefaultDestination() OutputDestination {
	return task.defaultDestination
}

func buildScheduledTask(ctx context.Context, props *ScheduledTaskProps, watcher ConfigWatcher) (ScheduledTask, error) {
	if props.config == nil {
		// If a config struct is not set, props MUST provide a default schedule to execute the task.
		if props.schedule == "" {
			return nil, ErrTaskScheduleNotGiven
		}

		dest := props.defaultDestination // Can be nil because the task response may return a specific destination to send the result to.
		return &scheduledTask{
			identifier:         props.identifier,
			taskFunc:           props.taskFunc,
			schedule:           props.schedule,
			defaultDestination: dest,
			configWrapper:      nil,
		}, nil
	}

	// https://github.com/oklahomer/go-sarah/issues/44
	locker := configLocker.get(props.botType, props.identifier)

	cfg := props.config
	err := func() error {
		locker.Lock()
		defer locker.Unlock()

		rv := reflect.ValueOf(cfg)
		if rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Map {
			return watcher.Read(ctx, props.botType, props.identifier, cfg)
		}

		// https://groups.google.com/forum/#!topic/Golang-Nuts/KB3_Yj3Ny4c
		// Obtain a pointer to the *underlying type* instead of CommandConfig.
		n := reflect.New(reflect.TypeOf(cfg))

		// Copy the current value to the newly created instance.
		// This includes private field values.
		n.Elem().Set(rv)

		// Pass the pointer to the newly created instance.
		e := watcher.Read(ctx, props.botType, props.identifier, n.Interface())
		if e == nil {
			cfg = n.Elem().Interface()
		}
		return e
	}()

	var notFoundErr *ConfigNotFoundError
	if err != nil && !errors.As(err, &notFoundErr) {
		// Unacceptable error
		return nil, fmt.Errorf("failed to read config for %s:%s: %w", props.botType, props.identifier, err)
	}

	// Set up the execution schedule
	schedule := props.schedule
	if scheduledConfig, ok := (cfg).(ScheduledConfig); ok {
		if s := scheduledConfig.Schedule(); s != "" {
			schedule = s
		}
	}
	if schedule == "" {
		return nil, ErrTaskScheduleNotGiven
	}

	// Set up default destination
	// This can be nil since each task execution may return a specific destination.
	dest := props.defaultDestination
	if destConfig, ok := (cfg).(DestinatedConfig); ok {
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
			value: cfg,
			mutex: locker,
		},
	}, nil
}

// ScheduledTaskProps is a designated non-serializable configuration struct to be used for ScheduledTask construction.
// This holds a relatively complex set of ScheduledTask construction arguments and properties.
type ScheduledTaskProps struct {
	botType            BotType
	identifier         string
	taskFunc           taskFunc
	schedule           string
	defaultDestination OutputDestination
	config             TaskConfig
}

// ScheduledTaskPropsBuilder helps to construct a ScheduledTaskProps.
// A developer may set up a ScheduledTask property -- ScheduledTaskProps -- by calling ScheduledTaskPropsBuilder.Build or ScheduledTaskPropsBuilder.MustBuild at the end.
// A validation logic runs on build, so the returning ScheduledTaskProps instant is safe to be passed to RegisterScheduledTaskProps.
type ScheduledTaskPropsBuilder struct {
	props *ScheduledTaskProps
}

// NewScheduledTaskPropsBuilder creates and returns a new ScheduledTaskPropsBuilder instance.
func NewScheduledTaskPropsBuilder() *ScheduledTaskPropsBuilder {
	return &ScheduledTaskPropsBuilder{
		props: &ScheduledTaskProps{},
	}
}

// BotType is a setter to provide the belonging BotType.
func (builder *ScheduledTaskPropsBuilder) BotType(botType BotType) *ScheduledTaskPropsBuilder {
	builder.props.botType = botType
	return builder
}

// Identifier is a setter for a ScheduledTask identifier.
func (builder *ScheduledTaskPropsBuilder) Identifier(id string) *ScheduledTaskPropsBuilder {
	builder.props.identifier = id
	return builder
}

// Func sets a function to be called on task execution.
// To set a function that requires some sort of configuration value, use ConfigurableFunc.
func (builder *ScheduledTaskPropsBuilder) Func(fn func(context.Context) ([]*ScheduledTaskResult, error)) *ScheduledTaskPropsBuilder {
	builder.props.config = nil
	builder.props.taskFunc = func(ctx context.Context, cfg ...TaskConfig) ([]*ScheduledTaskResult, error) {
		return fn(ctx)
	}
	return builder
}

// Schedule sets the execution schedule.
// The schedule can be expressed in a crontab way with seconds precision such as "0 30 * * * *" but some variations are also available.
// See https://pkg.go.dev/github.com/robfig/cron/v3 for details.
func (builder *ScheduledTaskPropsBuilder) Schedule(schedule string) *ScheduledTaskPropsBuilder {
	builder.props.schedule = schedule
	return builder
}

// DefaultDestination sets a default output destination of this task.
// OutputDestination returned as part of ScheduledTaskResult has higher priority;
// When none is specified by the result, then the default output destination is used.
func (builder *ScheduledTaskPropsBuilder) DefaultDestination(dest OutputDestination) *ScheduledTaskPropsBuilder {
	builder.props.defaultDestination = dest
	return builder
}

// ConfigurableFunc sets a function for the ScheduledTask with a configuration value.
// The given configuration value -- config -- is passed to the function as a third argument.
//
// When the resulting ScheduledTaskProps is passed to RegisterScheduledTask and Sarah runs with a ConfigWatcher,
// the configuration value is updated automatically when the corresponding setting is updated.
func (builder *ScheduledTaskPropsBuilder) ConfigurableFunc(config TaskConfig, fn func(context.Context, TaskConfig) ([]*ScheduledTaskResult, error)) *ScheduledTaskPropsBuilder {
	builder.props.config = config
	builder.props.taskFunc = func(ctx context.Context, cfg ...TaskConfig) ([]*ScheduledTaskResult, error) {
		return fn(ctx, cfg[0])
	}
	return builder
}

// Build builds new ScheduledTaskProps instance with the provided values.
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
// It simplifies the initialization of a global variable holding the built ScheduledTaskProps instance.
func (builder *ScheduledTaskPropsBuilder) MustBuild() *ScheduledTaskProps {
	task, err := builder.Build()
	if err != nil {
		panic(fmt.Errorf("error on building ScheduledTaskProps: %w", err))
	}

	return task
}
