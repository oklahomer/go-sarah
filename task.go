package sarah

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"os"
	"path"
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
	config             TaskConfig
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

func newScheduledTask(props *ScheduledTaskProps, configDir string) (ScheduledTask, error) {
	// If path to the configuration files' directory is given, corresponding configuration file MAY exist.
	// If exists, read and map to given config struct; if file does not exist, assume the config struct is already configured by developer.
	taskConfig := props.config
	var configWrapper *taskConfigWrapper
	if configDir != "" && taskConfig != nil {
		fileName := props.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)

		// https://github.com/oklahomer/go-sarah/issues/44
		locker := configLocker.get(configPath)

		err := func() error {
			locker.Lock()
			defer locker.Unlock()

			return readConfig(configPath, taskConfig)
		}()
		if err != nil && os.IsNotExist(err) {
			log.Infof("config struct is set, but there was no corresponding setting file at %s. "+
				"assume config struct is already filled with appropriate value and keep going. command ID: %s.",
				configPath, props.identifier)
		} else if err != nil {
			// File was there, but could not read.
			return nil, err
		}

		configWrapper = &taskConfigWrapper{
			value: taskConfig,
			mutex: locker,
		}
	}

	// Setup execution schedule
	schedule := props.schedule
	if taskConfig != nil {
		if scheduledConfig, ok := (taskConfig).(ScheduledConfig); ok {
			if s := scheduledConfig.Schedule(); s != "" {
				schedule = s
			}
		}
	}
	if schedule == "" {
		return nil, ErrTaskScheduleNotGiven
	}

	// Setup default destination
	// This can be nil since each task execution may return corresponding destination
	dest := props.defaultDestination
	if taskConfig != nil {
		if destConfig, ok := (taskConfig).(DestinatedConfig); ok {
			if d := destConfig.DefaultDestination(); d != nil {
				dest = d
			}
		}
	}

	return &scheduledTask{
		identifier:         props.identifier,
		taskFunc:           props.taskFunc,
		schedule:           schedule,
		defaultDestination: dest,
		config:             props.config,
		configWrapper:      configWrapper,
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
