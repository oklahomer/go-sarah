package sarah

import (
	"context"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/xerrors"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

var (
	// ErrCommandInsufficientArgument depicts an error that not enough arguments are set to CommandProps.
	// This is returned on CommandProps.Build() inside of runner.Run()
	ErrCommandInsufficientArgument = xerrors.New("BotType, Identifier, InstructionFunc, MatchFunc and (Configurable)Func must be set.")
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

	// Instruction returns example of user input. This should be used to provide command usage for end users.
	Instruction(input *HelpInput) string

	// Match is used to judge if this command corresponds to given user input.
	// If this returns true, Bot implementation should proceed to Execute with current user input.
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

	// If the command has configuration struct, lock before execution.
	// Config struct may be updated on configuration file change.
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
		// Obtain a pointer to the *underlying type* instead of sarah.CommandConfig.
		n := reflect.New(reflect.TypeOf(cfg))

		// Copy the current field value to newly created instance.
		// This includes private field values.
		n.Elem().Set(rv)

		// Pass the pointer to the newly created instance.
		e := watcher.Read(ctx, props.botType, props.identifier, n.Interface())
		if e == nil {
			// Replace the current value with updated value.
			cfg = n.Elem().Interface()
		}
		return e
	}()

	var notFoundErr *ConfigNotFoundError
	if err != nil && !xerrors.As(err, &notFoundErr) {
		// Unacceptable error
		return nil, xerrors.Errorf("failed to read config for %s:%s: %w", props.botType, props.identifier, err)
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

// CommandHelps is an alias to slice of CommandHelps' pointers.
type CommandHelps []*CommandHelp

// CommandHelp represents help messages for corresponding Command.
type CommandHelp struct {
	Identifier  string
	Instruction string
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
	botType         BotType
	identifier      string
	config          CommandConfig
	commandFunc     commandFunc
	matchFunc       func(Input) bool
	instructionFunc func(*HelpInput) string
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
		// Copy regexp pattern
		// https://golang.org/doc/go1.6#minor_library_changes
		// Some high-concurrency servers using the same Regexp from many goroutines have seen degraded performance due to contention on that mutex.
		// To help such servers, Regexp now has a Copy method, which makes a copy of a Regexp that shares most of the structure of the original
		// but has its own scratch space cache.
		//
		// Copy() employed in above context is no longer required as of Golang 1.12.
		// TODO Consider removing Copy() call when minimum supported version becomes 1.12 or most users switch to 1.12.
		// https://golang.org/doc/go1.12#regexp
		// Copy is no longer necessary to avoid lock contention, so it has been given a partial deprecation comment.
		// Copy may still be appropriate if the reason for its use is to make two copies with different Longest settings.
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
// On Runner.Run configuration is read from YAML/JSON file located at /path/to/config/dir/{commandIdentifier}.(yaml|yml|json) and mapped to given CommandConfig struct.
// If no YAML/JSON file is found, runner considers the given CommandConfig is fully configured and ready to use.
// This configuration struct is passed to command function as its third argument.
func (builder *CommandPropsBuilder) ConfigurableFunc(config CommandConfig, fn func(context.Context, Input, CommandConfig) (*CommandResponse, error)) *CommandPropsBuilder {
	builder.props.config = config
	builder.props.commandFunc = func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
		return fn(ctx, input, cfg[0])
	}
	return builder
}

// Instruction is a setter to provide an instruction of command execution.
// This should be used to provide command usage for end users.
func (builder *CommandPropsBuilder) Instruction(instruction string) *CommandPropsBuilder {
	builder.props.instructionFunc = func(input *HelpInput) string {
		return instruction
	}
	return builder
}

// InstructionFunc is a setter to provide a function that receives user input and returns instruction.
// Use Instruction() when a simple text instruction can always be returned.
// If the instruction has to be customized per user or the instruction has to be hidden in a certain group or from a certain user,
// use InstructionFunc().
// Use receiving *HelpInput and judge if an instruction should be returned.
// e.g. .reboot command is only supported for administrator users in admin group so this command should be hidden in other groups.
//
// Also see MatchFunc() for such authentication mechanism.
func (builder *CommandPropsBuilder) InstructionFunc(fnc func(input *HelpInput) string) *CommandPropsBuilder {
	builder.props.instructionFunc = fnc
	return builder
}

// Build builds new CommandProps instance with provided values.
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
// It simplifies safe initialization of global variables holding built CommandProps instances.
func (builder *CommandPropsBuilder) MustBuild() *CommandProps {
	props, err := builder.Build()
	if err != nil {
		panic(xerrors.Errorf("error on building CommandProps: %w", err))
	}

	return props
}
