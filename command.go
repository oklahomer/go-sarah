package sarah

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

var (
	// ErrCommandInsufficientArgument depicts an error that not enough arguments are set to CommandProps.
	// This can be returned by CommandPropsBuilder.Build.
	ErrCommandInsufficientArgument = errors.New("BotType, Identifier, InstructionFunc, MatchFunc and (Configurable)Func must be set")
)

// CommandResponse is returned by Command or ContextualFunc when the execution is finished.
type CommandResponse struct {
	// Content represents a group of data returned to the user.
	// Since this is passed to Bot.SendMessage as part of OutputMessage,
	// its type may vary depending on the Bot's integrating chat service.
	Content interface{}

	// UserContext represents a user's contextual state to be stored.
	// When this is non-nil and a UserContextStorage is present for the Bot, this value is passed to UserContextStorage.
	// The user's next Input is fed to UserContext.Next so the user can continue the interaction until UserContext is no longer returned.
	UserContext *UserContext
}

// Command defines an interface that all executable command MUST satisfy.
// This is executed against a user's input.
type Command interface {
	// Identifier returns a unique id of this Command.
	Identifier() string

	// Execute receives an Input sent by a user and returns a response in a form of CommandResponse.
	Execute(context.Context, Input) (*CommandResponse, error)

	// Instruction returns a help message to show the Command usage to the requesting user.
	// A list of instructions may be returned to a user at once, so the message should be simple.
	Instruction(*HelpInput) string

	// Match is used to judge if this Command corresponds to the given Input.
	// If this returns true, the Bot implementation should proceed to Execute this Command with the current Input.
	Match(Input) bool
}

type commandConfigWrapper struct {
	value CommandConfig
	mutex *sync.RWMutex
}

type defaultCommand struct {
	identifier      string
	matchFunc       func(Input) bool
	instructionFunc func(*HelpInput) string
	commandFunc     commandFunc
	configWrapper   *commandConfigWrapper
}

func (command *defaultCommand) Identifier() string {
	return command.identifier
}

func (command *defaultCommand) Instruction(input *HelpInput) string {
	return command.instructionFunc(input)
}

func (command *defaultCommand) Match(input Input) bool {
	return command.matchFunc(input)
}

func (command *defaultCommand) Execute(ctx context.Context, input Input) (*CommandResponse, error) {
	wrapper := command.configWrapper
	if wrapper == nil {
		return command.commandFunc(ctx, input)
	}

	// If the command has a config struct, lock before execution.
	// The config struct may be updated by ConfigWatcher at the same time.
	wrapper.mutex.RLock()
	defer wrapper.mutex.RUnlock()
	return command.commandFunc(ctx, input, wrapper.value)
}

func buildCommand(ctx context.Context, props *CommandProps, watcher ConfigWatcher) (Command, error) {
	if props.config == nil {
		return &defaultCommand{
			identifier:      props.identifier,
			matchFunc:       props.matchFunc,
			instructionFunc: props.instructionFunc,
			commandFunc:     props.commandFunc,
			configWrapper:   nil,
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

		// Copy the current field value to the newly created instance.
		// This includes private field values.
		n.Elem().Set(rv)

		// Pass the pointer of the created instance.
		e := watcher.Read(ctx, props.botType, props.identifier, n.Interface())
		if e == nil {
			// Replace the current value with the updated one.
			cfg = n.Elem().Interface()
		}
		return e
	}()

	var notFoundErr *ConfigNotFoundError
	if err != nil && !errors.As(err, &notFoundErr) {
		// Unacceptable error
		return nil, fmt.Errorf("failed to read config for %s:%s: %w", props.botType, props.identifier, err)
	}

	return &defaultCommand{
		identifier:      props.identifier,
		matchFunc:       props.matchFunc,
		instructionFunc: props.instructionFunc,
		commandFunc:     props.commandFunc,
		configWrapper: &commandConfigWrapper{
			value: cfg,
			mutex: locker,
		},
	}, nil
}

// StripMessage is a utility function that applies the given regular expression to the input string and replaces the matching part with the empty string.
// Use this to extract the meaningful input value out of the entire user message.
// e.g. ".echo Hey!" becomes "Hey!"
func StripMessage(pattern *regexp.Regexp, input string) string {
	return strings.TrimSpace(pattern.ReplaceAllString(input, ""))
}

// Commands stashes all registered Command.
// A Bot implementation can refer to this to register a given command on Bot.AppendCommand call, and to find a matching Command on Bot.Respond call.
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

// Append lets developers register a new Command to its internal stash.
// If another command is already registered with the same ID, the existing one is replaced in favor of the new one.
func (commands *Commands) Append(command Command) {
	commands.mutex.Lock()
	defer commands.mutex.Unlock()

	// See if a command with the same identifier exists.
	for i, cmd := range commands.collection {
		if cmd.Identifier() == command.Identifier() {
			logger.Infof("Replace old command in favor of newly appending one: %s.", command.Identifier())
			commands.collection[i] = command
			return
		}
	}

	// Not stored, then append to the last.
	logger.Infof("Append new command: %s.", command.Identifier())
	commands.collection = append(commands.collection, command)
}

// FindFirstMatched looks for the first matching command by calling each Command's Command.Match method:
// The first Command to return true is considered as "first matched" and is returned.
//
// The check for each Command is run in the order of registration; The earlier the Commands.Append is called, the earlier the check.
// Be sure to register an important Command first.
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

// ExecuteFirstMatched tries finding a matching command with the given Input and executes a Command if one is available.
func (commands *Commands) ExecuteFirstMatched(ctx context.Context, input Input) (*CommandResponse, error) {
	command := commands.FindFirstMatched(input)
	if command == nil {
		return nil, nil
	}

	return command.Execute(ctx, input)
}

// Helps returns all belonging commands' help messages in a form of *CommandHelps.
func (commands *Commands) Helps(input *HelpInput) *CommandHelps {
	commands.mutex.RLock()
	defer commands.mutex.RUnlock()

	helps := &CommandHelps{}
	for _, command := range commands.collection {
		instruction := command.Instruction(input)
		if instruction == "" {
			continue
		}

		h := &CommandHelp{
			Identifier:  command.Identifier(),
			Instruction: instruction,
		}
		*helps = append(*helps, h)
	}
	return helps
}

// CommandHelps is an alias to a slice of CommandHelp pointers.
type CommandHelps []*CommandHelp

// CommandHelp represents an instruction for the corresponding Command.
type CommandHelp struct {
	// Identifier represents the unique id of the corresponding Command.
	Identifier string

	// Instruction represents a help message to guide the Command usage.
	Instruction string
}

// CommandConfig provides an interface that every command configuration value must satisfy, which actually is empty.
// Think of this as a kind of marker interface with a more meaningful name.
type CommandConfig interface{}

type commandFunc func(context.Context, Input, ...CommandConfig) (*CommandResponse, error)

// NewCommandPropsBuilder returns new CommandPropsBuilder instance.
// Use this to set up a CommandProps that can be used to build a Command on the fly.
func NewCommandPropsBuilder() *CommandPropsBuilder {
	return &CommandPropsBuilder{
		props: &CommandProps{},
	}
}

// CommandProps is a designated non-serializable configuration struct to be used for Command construction.
// This holds a relatively complex set of Command construction arguments and properties.
type CommandProps struct {
	botType         BotType
	identifier      string
	config          CommandConfig
	commandFunc     commandFunc
	matchFunc       func(Input) bool
	instructionFunc func(*HelpInput) string
}

// CommandPropsBuilder helps to construct a CommandProps.
// A developer may set up a Command construction property -- CommandProps -- by calling CommandPropsBuilder.Build or CommandPropsBuilder.MustBuild at the end.
// A validation logic runs on build, so the returning CommandProps instant is safe to be passed to RegisterCommandProps.
type CommandPropsBuilder struct {
	props *CommandProps // This instance is not fully constructed til Build() is called.
}

// BotType is a setter to provide the belonging BotType.
func (builder *CommandPropsBuilder) BotType(botType BotType) *CommandPropsBuilder {
	builder.props.botType = botType
	return builder
}

// Identifier is a setter for a Command identifier.
func (builder *CommandPropsBuilder) Identifier(id string) *CommandPropsBuilder {
	builder.props.identifier = id
	return builder
}

// MatchPattern is a setter to provide a command match pattern.
// This regular expression is used against the given Input to see if the Command matches the Input.
//
// Use MatchFunc to set a more customizable matcher logic.
func (builder *CommandPropsBuilder) MatchPattern(pattern *regexp.Regexp) *CommandPropsBuilder {
	builder.props.matchFunc = func(input Input) bool {
		return pattern.MatchString(input.Message())
	}
	return builder
}

// MatchFunc is a setter for a function to judge if an incoming Input "matches" the Command.
// When this returns true, this command is considered to "match the user input" and becomes a Command execution candidate.
//
// MatchPattern may be used to specify a regular expression that is checked against user input, Input.Message();
// MatchFunc can specify more customizable matching logic.
// e.g. only return true on a specific sender's specific message on a specific time range.
func (builder *CommandPropsBuilder) MatchFunc(matchFunc func(Input) bool) *CommandPropsBuilder {
	builder.props.matchFunc = matchFunc
	return builder
}

// Func is a setter to provide a command function that requires no configuration.
// If ConfigurableFunc and Func are both called, the later call overrides the previous one.
func (builder *CommandPropsBuilder) Func(fn func(context.Context, Input) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.props.config = nil
	builder.props.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input)
	}
	return builder
}

// ConfigurableFunc is a setter to provide a command function that takes a configuration value as a struct.
// While Func lets developers set a simple function, this allows them to provide a function that requires some sort of configuration struct.
// On Sarah initiation, configuration settings are read by ConfigWatcher and mapped to the given CommandConfig value.
// This configuration value is passed to the command -- fn -- as its third argument.
func (builder *CommandPropsBuilder) ConfigurableFunc(config CommandConfig, fn func(context.Context, Input, CommandConfig) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.props.config = config
	builder.props.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input, cfg[0])
	}
	return builder
}

// Instruction is a setter to provide an instruction of command execution.
// This should be used to provide command usage for end-users.
func (builder *CommandPropsBuilder) Instruction(instruction string) *CommandPropsBuilder {
	builder.props.instructionFunc = func(input *HelpInput) string {
		return instruction
	}
	return builder
}

// InstructionFunc is a setter to provide a function that receives a user input and returns an instruction.
// Use Instruction() when a simple text instruction can always be returned.
// If the instruction has to be customized per user or the instruction has to be hidden in a certain group or from a certain user,
// use InstructionFunc() as it takes HelpInput as its argument.
// Use *HelpInput and judge if an instruction should be returned to the user.
// e.g. .reboot command is only supported for administrator users in the admin group so this command should be hidden in other groups.
//
// Also, see MatchFunc() for such an authentication mechanism.
func (builder *CommandPropsBuilder) InstructionFunc(fnc func(input *HelpInput) string) *CommandPropsBuilder {
	builder.props.instructionFunc = fnc
	return builder
}

// Build builds a new CommandProps instance with the provided values.
func (builder *CommandPropsBuilder) Build() (*CommandProps, error) {
	if builder.props.botType == "" ||
		builder.props.identifier == "" ||
		builder.props.instructionFunc == nil ||
		builder.props.matchFunc == nil ||
		builder.props.commandFunc == nil {

		return nil, ErrCommandInsufficientArgument
	}

	return builder.props, nil
}

// MustBuild is like Build but panics if any error occurs on Build.
// It simplifies the initialization of a global variable holding the built CommandProps instance.
func (builder *CommandPropsBuilder) MustBuild() *CommandProps {
	props, err := builder.Build()
	if err != nil {
		panic(fmt.Errorf("error on building CommandProps: %w", err))
	}

	return props
}
