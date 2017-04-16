package sarah

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
)

var (
	// ErrCommandInsufficientArgument depicts an error that not enough arguments are set to CommandProps.
	// This is returned on CommandProps.Build() inside of Runner.Run()
	ErrCommandInsufficientArgument = errors.New("Identifier, InputExample, MatchPattern, and (Configurable)Func must be set.")
)

// CommandResponse is returned by Command or Task when the execution is finished.
type CommandResponse struct {
	Content     interface{}
	UserContext *UserContext
}

// Command defines interface that all command MUST satisfy.
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

func newCommand(props *CommandProps, configDir string) (Command, error) {
	// If path to the configuration files' directory and config struct's pointer is given, corresponding configuration file MAY exist.
	// If exists, read and map to given config struct; if file does not exist, assume the config struct is already configured by developer.
	commandConfig := props.config
	if configDir != "" && commandConfig != nil {
		fileName := props.identifier + ".yaml"
		configPath := path.Join(configDir, fileName)
		err := readConfig(configPath, commandConfig)
		if err != nil && os.IsNotExist(err) {
			log.Infof("config struct is set, but there was no corresponding setting file at %s. "+
				"assume config struct is already filled with appropriate value and keep going. command ID: %s.",
				configPath, props.identifier)
		} else if err != nil {
			// File was there, but could not read.
			return nil, err
		}
	}

	return &simpleCommand{
		identifier:   props.identifier,
		example:      props.example,
		matchPattern: props.matchPattern,
		commandFunc:  props.commandFunc,
		config:       commandConfig,
	}, nil
}

// StripMessage is a utility function that strips string from given message based on given regular expression.
// This is to extract usable input value out of entire user message.
// e.g. ".echo Hey!" becomes "Hey!"
func StripMessage(pattern *regexp.Regexp, input string) string {
	return strings.TrimSpace(pattern.ReplaceAllString(input, ""))
}

// Commands stashes all registered Command.
type Commands []Command

// NewCommands creates and returns new Commands instance.
func NewCommands() *Commands {
	return &Commands{}
}

// Append let developers register new Command to its internal stash.
// If any command is registered with the same ID, the old one is replaced in favor of new one.
func (commands *Commands) Append(command Command) {
	// See if command with the same identifier exists.
	for i, cmd := range *commands {
		if cmd.Identifier() == command.Identifier() {
			log.Infof("replacing old command in favor of newly appending one: %s.", command.Identifier())
			(*commands)[i] = command
			return
		}
	}

	// Not stored, then append to the last.
	log.Infof("appending new command: %s.", command.Identifier())
	*commands = append(*commands, command)
}

// FindFirstMatched look for first matching command by calling Command's Match method: First Command.Match to return true
// is considered as "first matched" and is returned.
//
// This check is run in the order of Command registration: Earlier the Commands.Append is called, the command is checked
// earlier. So register important Command first.
func (commands *Commands) FindFirstMatched(text string) Command {
	for _, command := range *commands {
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

// Helps returns underlying commands help messages in a form of *CommandHelps.
func (commands *Commands) Helps() *CommandHelps {
	helps := &CommandHelps{}
	for _, command := range *commands {
		h := &CommandHelp{
			Identifier:   command.Identifier(),
			InputExample: command.InputExample(),
		}
		*helps = append(*helps, h)
	}
	return helps
}

// CommandHelps is an alias to slice of CommandHelps' pointers.
type CommandHelps []*CommandHelp

// CommandHelp represents help messages for corresponding Command.
type CommandHelp struct {
	Identifier   string
	InputExample string
}

// CommandConfig provides an interface that every command configuration must satisfy, which actually means empty.
type CommandConfig interface{}

type commandFunc func(context.Context, Input, ...CommandConfig) (*CommandResponse, error)

// NewCommandPropsBuilder returns new CommandPropsBuilder instance.
func NewCommandPropsBuilder() *CommandPropsBuilder {
	return &CommandPropsBuilder{}
}

// CommandProps is a designated non-serializable configuration struct to be used in Command construction.
// This holds relatively complex set of Command construction arguments that should be treated as one in logical term.
type CommandProps struct {
	botType      BotType
	identifier   string
	matchPattern *regexp.Regexp
	config       CommandConfig
	commandFunc  commandFunc
	example      string
}

// CommandPropsBuilder helps to construct CommandProps.
// Developer may set desired property as she goes and call CommandPropsBuilder.Build or CommandPropsBuilder.MustBuild to construct CommandProps at the end.
// A validation logic runs on build, so the returning CommandProps instant is safe to be passed to Runner.
type CommandPropsBuilder struct {
	botType      BotType
	identifier   string
	matchPattern *regexp.Regexp
	config       CommandConfig
	commandFunc  commandFunc
	example      string
}

// BotType is a setter to provide belonging BotType.
func (builder *CommandPropsBuilder) BotType(botType BotType) *CommandPropsBuilder {
	builder.botType = botType
	return builder
}

// Identifier is a setter for Command identifier.
func (builder *CommandPropsBuilder) Identifier(id string) *CommandPropsBuilder {
	builder.identifier = id
	return builder
}

// MatchPattern is a setter to provide command match pattern.
// This regular expression is used to find matching command with given Input.
func (builder *CommandPropsBuilder) MatchPattern(pattern *regexp.Regexp) *CommandPropsBuilder {
	builder.matchPattern = pattern
	return builder
}

// Func is a setter to provide command function that requires no configuration.
// If ConfigurableFunc and Func are both called, later call overrides the previous one.
func (builder *CommandPropsBuilder) Func(fn func(context.Context, Input) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.config = nil
	builder.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input)
	}
	return builder
}

// ConfigurableFunc is a setter to provide command function.
// While Func let developers set simple function, this allows them to provide function that requires some sort of configuration struct.
// On Runner.Run configuration is read from YAML file located at /path/to/config/dir/{commandIdentifier}.yaml and mapped to given CommandConfig struct.
// If no YAML file is found, Runner considers the given CommandConfig is fully configured and ready to use.
// This configuration struct is passed to command function as its third argument.
func (builder *CommandPropsBuilder) ConfigurableFunc(config CommandConfig, fn func(context.Context, Input, CommandConfig) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.config = config
	builder.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input, cfg[0])
	}
	return builder
}

// InputExample is a setter to provide example of command execution. This should be used to provide command usage for end users.
func (builder *CommandPropsBuilder) InputExample(example string) *CommandPropsBuilder {
	builder.example = example
	return builder
}

// Build builds new CommandProps instance with provided values.
func (builder *CommandPropsBuilder) Build() (*CommandProps, error) {
	if builder.botType == "" ||
		builder.identifier == "" ||
		builder.example == "" ||
		builder.matchPattern == nil ||
		builder.commandFunc == nil {

		return nil, ErrCommandInsufficientArgument
	}

	return &CommandProps{
		botType:      builder.botType,
		identifier:   builder.identifier,
		matchPattern: builder.matchPattern,
		config:       builder.config,
		commandFunc:  builder.commandFunc,
		example:      builder.example,
	}, nil
}

// MustBuild is like Build but panics if any error occurs on Build.
// It simplifies safe initialization of global variables holding built CommandProps instances.
func (builder *CommandPropsBuilder) MustBuild() *CommandProps {
	props, err := builder.Build()
	if err != nil {
		panic(fmt.Sprintf("Error on building CommandProps: %s", err.Error()))
	}

	return props
}

func readConfig(configPath string, config CommandConfig) error {
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(buf, config)
}
