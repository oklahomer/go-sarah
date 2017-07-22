package sarah

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	// ErrCommandInsufficientArgument depicts an error that not enough arguments are set to CommandProps.
	// This is returned on CommandProps.Build() inside of Runner.Run()
	ErrCommandInsufficientArgument = errors.New("BotType, Identifier, InputExample, MatchFunc, and (Configurable)Func must be set.")
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
	Match(Input) bool
}

type commandConfigWrapper struct {
	value CommandConfig
	mutex *sync.RWMutex
}

type defaultCommand struct {
	identifier    string
	example       string
	matchFunc     func(Input) bool
	commandFunc   commandFunc
	configWrapper *commandConfigWrapper
}

func (command *defaultCommand) Identifier() string {
	return command.identifier
}

func (command *defaultCommand) InputExample() string {
	return command.example
}

func (command *defaultCommand) Match(input Input) bool {
	return command.matchFunc(input)
}

func (command *defaultCommand) Execute(ctx context.Context, input Input) (*CommandResponse, error) {
	wrapper := command.configWrapper
	if wrapper == nil {
		return command.commandFunc(ctx, input)
	}

	// If the command has configuration struct, lock before execution.
	// Config struct may be updated on configuration file change.
	wrapper.mutex.RLock()
	defer wrapper.mutex.RUnlock()
	return command.commandFunc(ctx, input, wrapper.value)
}

func buildCommand(props *CommandProps, configDir string) (Command, error) {
	// If path to the configuration files' directory and config struct's pointer is given, corresponding configuration file MAY exist.
	// If exists, read and map to given config struct; if file does not exist, assume the config struct is already configured by developer.
	commandConfig := props.config
	var configWrapper *commandConfigWrapper
	if configDir != "" && commandConfig != nil {
		file := findPluginConfigFile(configDir, props.identifier)

		// https://github.com/oklahomer/go-sarah/issues/44
		locker := configLocker.get(configDir, props.identifier)
		if file != nil {
			err := func() error {
				locker.Lock()
				defer locker.Unlock()

				return updatePluginConfig(file, commandConfig)
			}()
			if err != nil && os.IsNotExist(err) {
				log.Infof("Config struct is set, but there was no corresponding setting file under %s. "+
					"Assume config struct is already filled with appropriate value and keep going. command ID: %s.",
					configDir, props.identifier)
			} else if err != nil {
				// File was there, but could not read.
				return nil, err
			}
		}

		configWrapper = &commandConfigWrapper{
			value: commandConfig,
			mutex: locker,
		}
	}

	return &defaultCommand{
		identifier:    props.identifier,
		example:       props.example,
		matchFunc:     props.matchFunc,
		commandFunc:   props.commandFunc,
		configWrapper: configWrapper,
	}, nil
}

// StripMessage is a utility function that strips string from given message based on given regular expression.
// This is to extract usable input value out of entire user message.
// e.g. ".echo Hey!" becomes "Hey!"
func StripMessage(pattern *regexp.Regexp, input string) string {
	return strings.TrimSpace(pattern.ReplaceAllString(input, ""))
}

// Commands stashes all registered Command.
type Commands struct {
	collection []Command
	mutex      sync.RWMutex
}

// NewCommands creates and returns new Commands instance.
func NewCommands() *Commands {
	return &Commands{
		collection: []Command{},
		mutex:      sync.RWMutex{},
	}
}

// Append let developers register new Command to its internal stash.
// If any command is registered with the same ID, the old one is replaced in favor of new one.
func (commands *Commands) Append(command Command) {
	commands.mutex.Lock()
	defer commands.mutex.Unlock()

	// See if command with the same identifier exists.
	for i, cmd := range commands.collection {
		if cmd.Identifier() == command.Identifier() {
			log.Infof("replacing old command in favor of newly appending one: %s.", command.Identifier())
			commands.collection[i] = command
			return
		}
	}

	// Not stored, then append to the last.
	log.Infof("appending new command: %s.", command.Identifier())
	commands.collection = append(commands.collection, command)
}

// FindFirstMatched look for first matching command by calling Command's Match method: First Command.Match to return true
// is considered as "first matched" and is returned.
//
// This check is run in the order of Command registration: Earlier the Commands.Append is called, the command is checked
// earlier. So register important Command first.
func (commands *Commands) FindFirstMatched(input Input) Command {
	commands.mutex.RLock()
	defer commands.mutex.RUnlock()

	for _, command := range commands.collection {
		if command.Match(input) {
			return command
		}
	}

	return nil
}

// ExecuteFirstMatched tries find matching command with the given input, and execute it if one is available.
func (commands *Commands) ExecuteFirstMatched(ctx context.Context, input Input) (*CommandResponse, error) {
	command := commands.FindFirstMatched(input)
	if command == nil {
		return nil, nil
	}

	return command.Execute(ctx, input)
}

// Helps returns underlying commands help messages in a form of *CommandHelps.
func (commands *Commands) Helps() *CommandHelps {
	commands.mutex.RLock()
	defer commands.mutex.RUnlock()

	helps := &CommandHelps{}
	for _, command := range commands.collection {
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
	return &CommandPropsBuilder{
		props: &CommandProps{},
	}
}

// CommandProps is a designated non-serializable configuration struct to be used in Command construction.
// This holds relatively complex set of Command construction arguments that should be treated as one in logical term.
type CommandProps struct {
	botType     BotType
	identifier  string
	config      CommandConfig
	commandFunc commandFunc
	matchFunc   func(Input) bool
	example     string
}

// CommandPropsBuilder helps to construct CommandProps.
// Developer may set desired property as she goes and call CommandPropsBuilder.Build or CommandPropsBuilder.MustBuild to construct CommandProps at the end.
// A validation logic runs on build, so the returning CommandProps instant is safe to be passed to Runner.
type CommandPropsBuilder struct {
	props *CommandProps // This props instance is not fully constructed til Build() is called.
}

// BotType is a setter to provide belonging BotType.
func (builder *CommandPropsBuilder) BotType(botType BotType) *CommandPropsBuilder {
	builder.props.botType = botType
	return builder
}

// Identifier is a setter for Command identifier.
func (builder *CommandPropsBuilder) Identifier(id string) *CommandPropsBuilder {
	builder.props.identifier = id
	return builder
}

// MatchPattern is a setter to provide command match pattern.
// This regular expression is used to find matching command with given Input.
//
// Use MatchFunc to set more customizable matching logic.
func (builder *CommandPropsBuilder) MatchPattern(pattern *regexp.Regexp) *CommandPropsBuilder {
	builder.props.matchFunc = func(input Input) bool {
		// https://golang.org/doc/go1.6#minor_library_changes
		return pattern.Copy().MatchString(input.Message())
	}
	return builder
}

// MatchFunc is a setter to provide a function that judges if an incoming input "matches" to this Command.
// When this returns true, this Command is considered as "corresponding to user input" and becomes Command execution candidate.
//
// MatchPattern may be used to specify a regular expression that is checked against user input, Input.Message();
// MatchFunc can specify more customizable matching logic. e.g. only return true on specific sender's specific message on specific time range.
func (builder *CommandPropsBuilder) MatchFunc(matchFunc func(Input) bool) *CommandPropsBuilder {
	builder.props.matchFunc = matchFunc
	return builder
}

// Func is a setter to provide command function that requires no configuration.
// If ConfigurableFunc and Func are both called, later call overrides the previous one.
func (builder *CommandPropsBuilder) Func(fn func(context.Context, Input) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.props.config = nil
	builder.props.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
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
	builder.props.config = config
	builder.props.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input, cfg[0])
	}
	return builder
}

// InputExample is a setter to provide example of command execution. This should be used to provide command usage for end users.
func (builder *CommandPropsBuilder) InputExample(example string) *CommandPropsBuilder {
	builder.props.example = example
	return builder
}

// Build builds new CommandProps instance with provided values.
func (builder *CommandPropsBuilder) Build() (*CommandProps, error) {
	if builder.props.botType == "" ||
		builder.props.identifier == "" ||
		builder.props.example == "" ||
		builder.props.matchFunc == nil ||
		builder.props.commandFunc == nil {

		return nil, ErrCommandInsufficientArgument
	}

	return builder.props, nil
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
