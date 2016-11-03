package sarah

import (
	"errors"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path"
	"regexp"
)

var (
	// NullConfig is an re-usable CommandConfig instance that indicates absence of command configuration.
	// e.g. NewCommandBuilder().ConfigStruct(NullConfig)
	NullConfig = &nullConfig{}

	// CommandInsufficientArgumentError depicts an error that not enough arguments are set to commandBuilder.
	// This is returned on commandBuilder.build() inside of Runner.Run()
	CommandInsufficientArgumentError = errors.New("Identifier, Example, MatchPattern, ConfigStruct and Func must be set.")
)

// ContextualFunc defines a function signature that defines user's next step.
// When a function or instance method is given as PluginResponse.Next, Bot implementation must store this with Input.SenderKey.
// On user's next input, inside of Bot.Respond, Bot retrieves stored ContextualFunc and execute this.
// If PluginResponse.Next is given again as part of result, the same step must be followed.
type ContextualFunc func(context.Context, Input) (*PluginResponse, error)

// PluginResponse is returned by Command or Task when the execution is finished.
type PluginResponse struct {
	Content interface{}
	Next    ContextualFunc
}

// Command defines interface that all Command must satisfy.
type Command interface {
	// Identifier returns unique id that represents this Command.
	Identifier() string

	// Execute receives input from user and returns response.
	Execute(context.Context, Input) (*PluginResponse, error)

	// Example returns example of user input.
	Example() string

	// Match is used to judge if this command corresponds to given user input.
	// If this returns true, Bot implementation should proceed to Execute with current user input.
	Match(string) bool
}

type simpleCommand struct {
	identifier string

	example string

	matchPattern *regexp.Regexp

	commandFunc commandFunc

	config CommandConfig
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

func (command *simpleCommand) Execute(ctx context.Context, input Input) (*PluginResponse, error) {
	return command.commandFunc(ctx, input, command.config)
}

// StripMessage is a utility function that strips string from given message based on given regular expression.
// This is to extract usable input value out of entire user message.
// e.g. ".echo Hey!" becomes "Hey!"
func StripMessage(pattern *regexp.Regexp, input string) string {
	return pattern.ReplaceAllString(input, "")
}

// Commands stashes all registered Command.
type Commands struct {
	cmd []Command
}

// NewCommands creates and returns new Commands instance.
func NewCommands() *Commands {
	return &Commands{cmd: make([]Command, 0)}
}

// Append let developers to register new Command to its internal stash.
func (commands *Commands) Append(command Command) {
	// TODO duplication check
	commands.cmd = append(commands.cmd, command)
}

// FindFirstMatched look for first matching command by calling Command's Match method: First Command.Match to return true
// is considered as "first matched" and is returned.
//
// This check is run in the order of Command registration: Earlier the Commands.Append is called, the command is checked
// earlier. So register important Command first.
func (commands *Commands) FindFirstMatched(text string) Command {
	for _, command := range commands.cmd {
		if command.Match(text) {
			return command
		}
	}

	return nil
}

// ExecuteFirstMatched tries find matching command with the given input, and execute it if one is available.
func (commands *Commands) ExecuteFirstMatched(ctx context.Context, input Input) (*PluginResponse, error) {
	inputMessage := input.Message()
	command := commands.FindFirstMatched(inputMessage)
	if command == nil {
		return nil, nil
	}

	return command.Execute(ctx, input)
}

type nullConfig struct{}

// CommandConfig provides an interface that every command configuration must satisfy, which actually means empty.
type CommandConfig interface{}

// commandFunc is a function type that represents command function
type commandFunc func(context.Context, Input, CommandConfig) (*PluginResponse, error)

type commandBuilder struct {
	identifier   string
	matchPattern *regexp.Regexp
	config       CommandConfig
	commandFunc  commandFunc
	example      string
}

// NewCommandBuilder returns new commandBuilder instance.
// This can be used to setup your desired bot Command. Pass this instance to sarah.AppendCommandBuilder, and the Command will be configured when Bot runs.
func NewCommandBuilder() *commandBuilder {
	return &commandBuilder{}
}

// Identifier is a setter for Command identifier.
func (builder *commandBuilder) Identifier(id string) *commandBuilder {
	builder.identifier = id
	return builder
}

// ConfigStruct is a setter for CommandConfig instance. Passed CommandConfig is used in readConfig to read and set corresponding values.
func (builder *commandBuilder) ConfigStruct(config CommandConfig) *commandBuilder {
	builder.config = config
	return builder
}

// MatchPattern is a setter to provide command match pattern.
// This regular expression is used to find matching command with given Input.
func (builder *commandBuilder) MatchPattern(pattern *regexp.Regexp) *commandBuilder {
	builder.matchPattern = pattern
	return builder
}

// Func is a setter to provide Command function.
func (builder *commandBuilder) Func(function commandFunc) *commandBuilder {
	builder.commandFunc = function
	return builder
}

// Example is a setter to provide example of command execution. This should be used to provide bot usage for end users.
func (builder *commandBuilder) Example(example string) *commandBuilder {
	builder.example = example
	return builder
}

// build builds new Command instance with provided values.
func (builder *commandBuilder) build(configDir string) (Command, error) {
	if builder.identifier == "" ||
		builder.example == "" ||
		builder.matchPattern == nil ||
		builder.config == nil ||
		builder.commandFunc == nil {

		return nil, CommandInsufficientArgumentError
	}

	commandConfig := builder.config
	switch commandConfig.(type) {
	case *nullConfig:
	// Do nothing about configuration settings.
	default:
		fileName := builder.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)
		err := readConfig(configPath, commandConfig)
		if err != nil {
			return nil, err
		}
	}

	return &simpleCommand{
		identifier:   builder.identifier,
		example:      builder.example,
		matchPattern: builder.matchPattern,
		commandFunc:  builder.commandFunc,
		config:       commandConfig,
	}, nil
}

func readConfig(configPath string, config CommandConfig) error {
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(buf, config)
}
