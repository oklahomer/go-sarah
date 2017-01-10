package sarah

import (
	"errors"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"os"
	"path"
)

var (
	ErrTaskInsufficientArgument = errors.New("Identifier, Func and ConfigStruct must be set.")
)

type ScheduledTaskResult struct {
	Content     interface{}
	Destination OutputDestination
}

// commandFunc is a function type that represents command function
type taskFunc func(context.Context, ScheduledTaskConfig) (*ScheduledTaskResult, error)

type ScheduledTaskConfig interface {
	Schedule() string
	Destination() OutputDestination
}

type scheduledTask struct {
	identifier string
	taskFunc   taskFunc
	config     ScheduledTaskConfig
}

func (task *scheduledTask) Identifier() string {
	return task.identifier
}

func (task *scheduledTask) Execute(ctx context.Context) (*ScheduledTaskResult, error) {
	return task.taskFunc(ctx, task.config)
}

type ScheduledTaskBuilder struct {
	identifier string
	taskFunc   taskFunc
	config     ScheduledTaskConfig
}

func NewScheduledTaskBuilder() *ScheduledTaskBuilder {
	return &ScheduledTaskBuilder{}
}

func (builder *ScheduledTaskBuilder) Identifier(id string) *ScheduledTaskBuilder {
	builder.identifier = id
	return builder
}

func (builder *ScheduledTaskBuilder) Func(function taskFunc) *ScheduledTaskBuilder {
	builder.taskFunc = function
	return builder
}

func (builder *ScheduledTaskBuilder) ConfigStruct(config ScheduledTaskConfig) *ScheduledTaskBuilder {
	builder.config = config
	return builder
}

func (builder *ScheduledTaskBuilder) Build(configDir string) (*scheduledTask, error) {
	if builder.identifier == "" || builder.taskFunc == nil || builder.config == nil {
		return nil, ErrTaskInsufficientArgument
	}

	// If path to the configuration files' directory is given, corresponding configuration file MAY exist.
	// If exists, read and map to given config struct; if file does not exist, assume the config struct is already configured by developer.
	taskConfig := builder.config
	if configDir != "" {
		fileName := builder.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)
		err := readConfig(configPath, taskConfig)
		if err != nil && os.IsNotExist(err) {
			log.Infof("config struct is set, but there was no corresponding setting file at %s. "+
				"assume config struct is already filled with appropriate value and keep going. command ID: %s.",
				configPath, builder.identifier)
		} else if err != nil {
			// File was there, but could not read.
			return nil, err
		}
	}

	return &scheduledTask{
		identifier: builder.identifier,
		taskFunc:   builder.taskFunc,
		config:     builder.config,
	}, nil
}
