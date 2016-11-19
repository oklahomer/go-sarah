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
	// ErrCommandInsufficientArgument depicts an error that not enough arguments are set to CommandBuilder.
	// This is returned on CommandBuilder.build() inside of Runner.Run()
	ErrCommandInsufficientArgument = errors.New("Identifier, InputExample, MatchPattern, and (Configurable)Func must be set.")
)

// ContextualFunc defines a function signature that defines user's next step.
// When a function or instance method is given as CommandResponse.Next, Bot implementation must store this with Input.SenderKey.
// On user's next input, inside of Bot.Respond, Bot retrieves stored ContextualFunc and execute this.
// If CommandResponse.Next is given again as part of result, the same step must be followed.
type ContextualFunc func(context.Context, Input) (*CommandResponse, error)

// CommandResponse is returned by Command or Task when the execution is finished.
type CommandResponse struct {
	Content interface{}
	Next    ContextualFunc
}

// Command defines interface that all Command must satisfy.
type Command interface {
	// Identifier returns unique id that represents this Command.
	Identifier() string

	// Execute receives input from user and returns response.
	Execute(context.Context, Input) (*CommandResponse, error)

	// InputExample returns example of user input. This should be used to provide command usage for end users.
	InputExample() string

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

func (command *simpleCommand) InputExample() string {
	return command.example
}

func (command *simpleCommand) Match(input string) bool {
	return command.matchPattern.MatchString(input)
}

func (command *simpleCommand) Execute(ctx context.Context, input Input) (*CommandResponse, error) {
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
func (commands *Commands) ExecuteFirstMatched(ctx context.Context, input Input) (*CommandResponse, error) {
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

type commandFunc func(context.Context, Input, ...CommandConfig) (*CommandResponse, error)

type CommandBuilder struct {
	identifier   string
	matchPattern *regexp.Regexp
	config       CommandConfig
	commandFunc  commandFunc
	example      string
}

// NewCommandBuilder returns new CommandBuilder instance.
// This can be used to setup your desired bot Command. Pass this instance to sarah.AppendCommandBuilder, and the Command will be configured when Bot runs.
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// Identifier is a setter for Command identifier.
func (builder *CommandBuilder) Identifier(id string) *CommandBuilder {
	builder.identifier = id
	return builder
}

// MatchPattern is a setter to provide command match pattern.
// This regular expression is used to find matching command with given Input.
func (builder *CommandBuilder) MatchPattern(pattern *regexp.Regexp) *CommandBuilder {
	builder.matchPattern = pattern
	return builder
}

// Func is a setter to provide command function that requires no configuration.
// If ConfigurableFunc and Func are both called, later call overrides the previous one.
func (builder *CommandBuilder) Func(fn func(context.Context, Input) (*CommandResponse, error)) *CommandBuilder {
	builder.config = nil
	builder.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input)
	}
	return builder
}

// ConfigurableFunc is a setter to provide command function.
// While Func let developers set simple function, this allows them to provide function that requires some sort of configuration struct.
// On Runner.Run configuration is read from YAML file located at /path/to/config/dir/{commandIdentifier}.yaml and mapped to given CommandConfig struct.
// The configuration is passed to command function as its third argument.
func (builder *CommandBuilder) ConfigurableFunc(config CommandConfig, fn func(context.Context, Input, CommandConfig) (*CommandResponse, error)) *CommandBuilder {
	builder.config = config
	builder.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input, cfg[0])
	}
	return builder
}

// InputExample is a setter to provide example of command execution. This should be used to provide command usage for end users.
func (builder *CommandBuilder) InputExample(example string) *CommandBuilder {
	builder.example = example
	return builder
}

// build builds new Command instance with provided values.
func (builder *CommandBuilder) Build(configDir string) (Command, error) {
	if builder.identifier == "" ||
		builder.example == "" ||
		builder.matchPattern == nil ||
		builder.commandFunc == nil {

		return nil, ErrCommandInsufficientArgument
	}

	commandConfig := builder.config
	if commandConfig != nil {
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
