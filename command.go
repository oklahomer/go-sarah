package sarah

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
)

var (
	NullConfig = &nullConfig{}
)

type CommandResponse struct {
	Input           BotInput
	ResponseContent interface{}
}

type Command interface {
	Identifier() string

	Execute(string, BotInput) (*CommandResponse, error)

	Example() string

	Match(string) bool

	StripMessage(string) string
}

type simpleCommand struct {
	identifier string

	example string

	matchPattern *regexp.Regexp

	commandFunc commandFunc

	config CommandConfig
}

func newSimpleCommand(identifier, example string, matchPattern *regexp.Regexp, commandFunc commandFunc, config CommandConfig) Command {
	return &simpleCommand{
		identifier:   identifier,
		example:      example,
		matchPattern: matchPattern,
		commandFunc:  commandFunc,
		config:       config,
	}
}

func (command *simpleCommand) Identifier() string {
	return command.identifier
}

func (command *simpleCommand) Example() string {
	return command.example
}

func (command *simpleCommand) Match(input string) bool {
	return command.matchPattern.MatchString(input)
}

func (command *simpleCommand) StripMessage(input string) string {
	text := command.matchPattern.ReplaceAllString(input, "")
	return strings.TrimSpace(text)
}

func (command *simpleCommand) Execute(strippedMessage string, input BotInput) (*CommandResponse, error) {
	return command.commandFunc(strippedMessage, input, command.config)
}

type Commands struct {
	cmd []Command
}

func NewCommands() *Commands {
	return &Commands{cmd: make([]Command, 0)}
}

func (commands *Commands) Append(command Command) {
	commands.cmd = append(commands.cmd, command)
}

func (commands *Commands) FindFirstMatched(text string) Command {
	for _, command := range commands.cmd {
		if command.Match(text) {
			return command
		}
	}

	return nil
}

func (commands *Commands) ExecuteFirstMatched(input BotInput) (*CommandResponse, error) {
	inputMessage := input.GetMessage()
	command := commands.FindFirstMatched(inputMessage)
	if command == nil {
		return nil, nil
	}

	res, err := command.Execute(command.StripMessage(inputMessage), input)
	if err != nil {
		return nil, err
	}
	res.Input = input

	return res, err
}

type nullConfig struct{}

type CommandConfig interface{}

type commandFunc func(string, BotInput, CommandConfig) (*CommandResponse, error)

type commandBuilder struct {
	identifier   string
	matchPattern *regexp.Regexp
	config       CommandConfig
	commandFunc  commandFunc
	example      string
}

func NewCommandBuilder() *commandBuilder {
	return &commandBuilder{}
}

func (builder *commandBuilder) Identifier(id string) *commandBuilder {
	builder.identifier = id
	return builder
}

func (builder *commandBuilder) ConfigStruct(config CommandConfig) *commandBuilder {
	builder.config = config
	return builder
}

func (builder *commandBuilder) MatchPattern(pattern *regexp.Regexp) *commandBuilder {
	builder.matchPattern = pattern
	return builder
}

func (builder *commandBuilder) Func(function commandFunc) *commandBuilder {
	builder.commandFunc = function
	return builder
}

func (builder *commandBuilder) Example(example string) *commandBuilder {
	builder.example = example
	return builder
}

func (builder *commandBuilder) build(configDir string) (Command, error) {
	if builder.identifier == "" {
		return nil, NewCommandInsufficientArgumentError("command identifier must be set.")
	}
	if builder.example == "" {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command example must be set. id: %s", builder.identifier))
	}
	if builder.matchPattern == nil {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command constructor must be set. id: %s", builder.identifier))
	}
	if builder.config == nil {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command config struct must be set. id: %s", builder.identifier))
	}
	if builder.commandFunc == nil {
		return nil, NewCommandInsufficientArgumentError(fmt.Sprintf("command function must be set. id: %s", builder.identifier))
	}

	switch config := builder.config.(type) {
	case *nullConfig:
		return newSimpleCommand(builder.identifier, builder.example, builder.matchPattern, builder.commandFunc, config), nil
	default:
		fileName := builder.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)
		config, err := readConfig(configPath, config)
		if err != nil {
			return nil, err
		}
		return newSimpleCommand(builder.identifier, builder.example, builder.matchPattern, builder.commandFunc, config), nil
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
