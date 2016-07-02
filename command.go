package sarah

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path"
)

type CommandConfig interface{}

type CommandResponse struct {
	ResponseContent interface{}
}

type Command interface {
	Identifier() string

	Execute(BotInput) (*CommandResponse, error)

	Example() string

	Match(string) bool

	StripCommand(string) string
}

type EmptyCommandConfig struct{}

func NewCommandBuilder() *commandBuilder {
	return &commandBuilder{}
}

type commandBuilder struct {
	identifier  string
	constructor func(CommandConfig) Command
	config      CommandConfig
}

func (builder *commandBuilder) Identifier(id string) *commandBuilder {
	builder.identifier = id
	return builder
}

func (builder *commandBuilder) Constructor(constructor func(CommandConfig) Command) *commandBuilder {
	builder.constructor = constructor
	return builder
}

func (builder *commandBuilder) ConfigStruct(config CommandConfig) *commandBuilder {
	builder.config = config
	return builder
}

func (builder *commandBuilder) build(configDir string) (Command, error) {
	if builder.identifier == "" {
		return nil, NewCommandInsufficientArgumentError("command identifier must be set.")
	}
	if builder.constructor == nil {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command constructor must be set. id: %s", builder.identifier))
	}
	if builder.config == nil {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command config struct must be set. id: %s", builder.identifier))
	}

	switch config := builder.config.(type) {
	case *EmptyCommandConfig:
		return builder.constructor(config), nil
	default:
		fileName := builder.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)
		config, err := readConfig(configPath, config)
		if err != nil {
			return nil, err
		}
		return builder.constructor(config), nil
	}
}

type CommandInsufficientArgumentError struct {
	Err string
}

func (e *CommandInsufficientArgumentError) Error() string {
	return e.Err
}

func NewCommandInsufficientArgumentError(err string) *CommandInsufficientArgumentError {
	return &CommandInsufficientArgumentError{err}
}

func readConfig(configPath string, config CommandConfig) (CommandConfig, error) {
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(buf, config); err != nil {
		return nil, err
	}

	return config, nil
}
