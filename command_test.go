package sarah

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type DummyCommand struct {
	IdentifierValue string
	ExecuteFunc     func(context.Context, Input) (*CommandResponse, error)
	InstructionFunc func(*HelpInput) string
	MatchFunc       func(Input) bool
}

var _ Command = (*DummyCommand)(nil)

func (command *DummyCommand) Identifier() string {
	return command.IdentifierValue
}

func (command *DummyCommand) Execute(ctx context.Context, input Input) (*CommandResponse, error) {
	return command.ExecuteFunc(ctx, input)
}

func (command *DummyCommand) Instruction(input *HelpInput) string {
	return command.InstructionFunc(input)
}

func (command *DummyCommand) Match(input Input) bool {
	return command.MatchFunc(input)
}

func TestNewCommandPropsBuilder(t *testing.T) {
	builder := NewCommandPropsBuilder()
	if builder == nil {
		t.Fatal("NewCommandPropsBuilder returned nil.")
	}
}

func TestCommandPropsBuilder_ConfigurableFunc(t *testing.T) {
	wrappedFncCalled := false
	config := &struct{}{}
	fnc := func(_ context.Context, _ Input, passedConfig CommandConfig) (*CommandResponse, error) {
		wrappedFncCalled = true
		if passedConfig != config {
			t.Errorf("Passed config is not the expected one: %#v", passedConfig)
		}
		return nil, nil
	}

	builder := &CommandPropsBuilder{props: &CommandProps{}}
	builder.ConfigurableFunc(config, fnc)
	if builder.props.config != config {
		t.Error("Passed config struct is not set.")
	}

	_, _ = builder.props.commandFunc(context.TODO(), &DummyInput{}, config)
	if wrappedFncCalled == false {
		t.Error("Provided func was not properly wrapped in builder.")
	}
}

func TestCommandPropsBuilder_BotType(t *testing.T) {
	var botType BotType = "dummy"
	builder := &CommandPropsBuilder{props: &CommandProps{}}

	builder.BotType(botType)
	if builder.props.botType != botType {
		t.Error("Provided BotType was not set.")
	}
}

func TestCommandPropsBuilder_Func(t *testing.T) {
	wrappedFncCalled := false
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	fnc := func(_ context.Context, _ Input) (*CommandResponse, error) {
		wrappedFncCalled = true
		return nil, nil
	}

	builder.Func(fnc)
	_, _ = builder.props.commandFunc(context.TODO(), &DummyInput{})
	if wrappedFncCalled == false {
		t.Error("Provided func was not properly wrapped in builder.")
	}
}

func TestCommandPropsBuilder_Identifier(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	id := "FOO"
	builder.Identifier(id)

	if builder.props.identifier != id {
		t.Error("Provided identifier is not set.")
	}
}

func TestCommandPropsBuilder_Instruction(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	example := ".echo foo"
	builder.Instruction(example)

	instruction := builder.props.instructionFunc(&HelpInput{})
	if instruction != example {
		t.Error("Provided instruction is not returned.")
	}
}

func TestCommandPropsBuilder_InstructionFunc(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	fnc := func(_ *HelpInput) string {
		return "dummy"
	}
	builder.InstructionFunc(fnc)

	if reflect.ValueOf(builder.props.instructionFunc).Pointer() != reflect.ValueOf(fnc).Pointer() {
		t.Error("Passed function is not set.")
	}
}

func TestCommandPropsBuilder_MatchPattern(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	builder.MatchPattern(regexp.MustCompile(`^\.echo`))

	if !builder.props.matchFunc(&DummyInput{MessageValue: ".echo"}) {
		t.Error("Expected true to return, but did not.")
	}
}

func TestCommandPropsBuilder_MatchFunc(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	builder.MatchFunc(func(input Input) bool {
		return regexp.MustCompile(`^\.echo`).MatchString(input.Message())
	})

	if !builder.props.matchFunc(&DummyInput{MessageValue: ".echo"}) {
		t.Error("Expected true to return, but did not.")
	}
}

func TestCommandPropsBuilder_Build(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	if _, err := builder.Build(); err == nil {
		t.Error("expected error not given.")
	} else if err != ErrCommandInsufficientArgument {
		t.Errorf("expected error not given. %#v", err)
	}

	var botType BotType = "dummy"
	matchPattern := regexp.MustCompile(`^\.echo`)
	identifier := "dummy"
	example := ".echo knock knock"
	config := &struct {
		Token string
	}{
		Token: "dummy",
	}
	fnc := func(_ context.Context, input Input, passedConfig CommandConfig) (*CommandResponse, error) {
		return nil, nil
	}
	builder.BotType(botType).
		Identifier(identifier).
		MatchPattern(matchPattern).
		Instruction(example).
		ConfigurableFunc(config, fnc)

	props, err := builder.Build()
	if err != nil {
		t.Errorf("something is wrong with command construction. %#v", err)
	}

	if props == nil {
		t.Fatal("Built command is not returned.")
	}

	if props.botType != botType {
		t.Errorf("Expected BotType is not set: %s.", props.botType)
	}

	if props.identifier != identifier {
		t.Errorf("Expected identifier is not set: %s.", props.identifier)
	}

	if !props.matchFunc(&DummyInput{MessageValue: ".echo foo"}) {
		t.Error("Expected match result is not given.")
	}

	instruction := props.instructionFunc(&HelpInput{})
	if instruction != example {
		t.Errorf("Expected example is not returned: %s.", instruction)
	}

	if props.config != config {
		t.Errorf("Expected config struct is not set: %#v.", config)
	}
}

func TestCommandPropsBuilder_MustBuild(t *testing.T) {
	builder := &CommandPropsBuilder{props: &CommandProps{}}
	builder.BotType("dummyBot").
		Identifier("dummy").
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		Instruction(".echo knock knock")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic did not occur.")
			}
		}()
		builder.MustBuild()
	}()

	builder.Func(func(_ context.Context, input Input) (*CommandResponse, error) {
		return nil, nil
	})
	props := builder.MustBuild()
	if props.identifier != builder.props.identifier {
		t.Error("Provided identifier is not set.")
	}
}
func TestNewCommands(t *testing.T) {
	commands := NewCommands()
	if commands == nil {
		t.Error("Not properly initialized.")
	}
}

func TestCommands_FindFirstMatched(t *testing.T) {
	commands := &Commands{}
	matchedCommand := commands.FindFirstMatched(&DummyInput{MessageValue: "echo"})
	if matchedCommand != nil {
		t.Fatalf("Something is returned while nothing other than nil may returned: %#v.", matchedCommand)
	}

	irrelevantCommand := &DummyCommand{}
	irrelevantCommand.MatchFunc = func(_ Input) bool {
		return false
	}
	echoCommand := &DummyCommand{}
	echoCommand.MatchFunc = func(input Input) bool {
		return strings.HasPrefix(input.Message(), "echo")
	}
	echoCommand.ExecuteFunc = func(_ context.Context, _ Input) (*CommandResponse, error) {
		return &CommandResponse{Content: ""}, nil
	}
	irrelevantCommand2 := &DummyCommand{}
	irrelevantCommand2.MatchFunc = func(_ Input) bool {
		return false
	}
	commands = &Commands{collection: []Command{irrelevantCommand, echoCommand, irrelevantCommand2}}

	matchedCommand = commands.FindFirstMatched(&DummyInput{MessageValue: "echo"})
	if matchedCommand == nil {
		t.Fatal("Expected command is not found.")
	}

	if matchedCommand != echoCommand {
		t.Fatalf("Expected command instance not returned: %#v.", matchedCommand)
	}
}

func TestCommands_ExecuteFirstMatched(t *testing.T) {
	commands := &Commands{}

	input := &DummyInput{}
	input.MessageValue = "echo foo"
	response, err := commands.ExecuteFirstMatched(context.TODO(), input)
	if err != nil {
		t.Error("Error is returned on non matching case.")
	}
	if response != nil {
		t.Error("Response should be nil on non matching case.")
	}

	echoCommand := &DummyCommand{}
	echoCommand.MatchFunc = func(input Input) bool {
		return strings.HasPrefix(input.Message(), "echo")
	}
	echoCommand.ExecuteFunc = func(_ context.Context, _ Input) (*CommandResponse, error) {
		return &CommandResponse{Content: ""}, nil
	}
	commands = &Commands{collection: []Command{echoCommand}}
	response, err = commands.ExecuteFirstMatched(context.TODO(), input)
	if err != nil {
		t.Errorf("Unexpected error on command execution: %#v.", err)
		return
	}

	if response == nil {
		t.Error("Response expected, but was not returned.")
		return
	}

	switch v := response.Content.(type) {
	case string:
	//OK
	default:
		t.Errorf("Expected string, but was %#v.", v)
	}
}

func TestCommands_Append(t *testing.T) {
	commands := &Commands{}

	command := &DummyCommand{
		IdentifierValue: "first",
	}

	// First operation
	commands.Append(command)
	if len(commands.collection) == 0 {
		t.Fatal("Provided command was not appended.")
	}

	if (commands.collection)[0] != command {
		t.Fatalf("Appended command is not the one provided: %#v", commands.collection[0])
	}

	// Second operation with same command
	commands.Append(command)
	if len(commands.collection) != 1 {
		t.Fatalf("Expected only one command to stay, but was: %d.", len(commands.collection))
	}

	// Third operation with different command
	anotherCommand := &DummyCommand{
		IdentifierValue: "second",
	}
	commands.Append(anotherCommand)
	if len(commands.collection) != 2 {
		t.Fatalf("Expected 2 commands to stay, but was: %d.", len(commands.collection))
	}
}

func TestCommands_Helps(t *testing.T) {
	cmd1 := &DummyCommand{
		IdentifierValue: "id",
		InstructionFunc: func(_ *HelpInput) string {
			return "example"
		},
	}
	cmd2 := &DummyCommand{
		IdentifierValue: "hiddenCommand",
		InstructionFunc: func(_ *HelpInput) string {
			return ""
		},
	}
	commands := &Commands{collection: []Command{cmd1, cmd2}}

	helps := commands.Helps(&HelpInput{})
	if len(*helps) != 1 {
		t.Fatalf("Expectnig one help to be given, but was %d.", len(*helps))
	}
	if (*helps)[0].Identifier != cmd1.IdentifierValue {
		t.Errorf("Expected ID was not returned: %s.", (*helps)[0].Identifier)
	}
	if (*helps)[0].Instruction != cmd1.InstructionFunc(&HelpInput{}) {
		t.Errorf("Expected instruction was not returned: %s.", (*helps)[0].Instruction)
	}
}

func TestSimpleCommand_Identifier(t *testing.T) {
	id := "bar"
	command := defaultCommand{identifier: id}

	if command.Identifier() != id {
		t.Errorf("Stored identifier is not returned: %s.", command.Identifier())
	}
}

func TestSimpleCommand_Instruction(t *testing.T) {
	instruction := "example foo"
	command := defaultCommand{
		instructionFunc: func(_ *HelpInput) string {
			return instruction
		},
	}

	if command.Instruction(&HelpInput{}) != instruction {
		t.Errorf("Stored example is not returned: %s.", command.Identifier())
	}
}

func TestSimpleCommand_Match(t *testing.T) {
	command := defaultCommand{matchFunc: func(input Input) bool {
		return regexp.MustCompile(`^\.echo`).MatchString(input.Message())
	}}

	if command.Match(&DummyInput{MessageValue: ".echo foo"}) == false {
		t.Error("Expected match result is not returned.")
	}
}

func TestSimpleCommand_Execute(t *testing.T) {
	wrappedFncCalled := false
	command := defaultCommand{
		configWrapper: &commandConfigWrapper{
			value: &struct{}{},
			mutex: &sync.RWMutex{},
		},
		commandFunc: func(ctx context.Context, input Input, cfg ...CommandConfig) (*CommandResponse, error) {
			wrappedFncCalled = true
			return nil, nil
		},
	}

	input := &DummyInput{}
	_, err := command.Execute(context.TODO(), input)
	if err != nil {
		t.Errorf("Error is returned: %s", err.Error())
	}
	if wrappedFncCalled == false {
		t.Error("Wrapped function is not called.")
	}
}

func TestStripMessage(t *testing.T) {
	pattern := regexp.MustCompile(`^\.echo`)
	stripped := StripMessage(pattern, ".echo foo bar")

	if stripped != "foo bar" {
		t.Errorf("Unexpected return value: %s.", stripped)
	}
}

func Test_buildCommand(t *testing.T) {
	type config struct {
		text string
	}
	tests := []struct {
		props          *CommandProps
		watcher        ConfigWatcher
		validateConfig func(cfg interface{}) error
		hasErr         bool
	}{
		{
			// No config
			props: &CommandProps{
				botType:    "botType",
				identifier: "fileNotFound",
				config:     nil,
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
					return nil, nil
				},
				matchFunc: func(_ Input) bool {
					return true
				},
				instructionFunc: func(_ *HelpInput) string {
					return ""
				},
			},
			watcher: nil,
			hasErr:  false,
		},
		{
			props: &CommandProps{
				botType:    "botType",
				identifier: "fileNotFound",
				config:     &config{},
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
					return nil, nil
				},
				matchFunc: func(_ Input) bool {
					return true
				},
				instructionFunc: func(_ *HelpInput) string {
					return ""
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, cfg interface{}) error {
					config, ok := cfg.(*config)
					if !ok {
						t.Errorf("Unexpected type is passed: %T.", cfg)
						return nil
					}

					config.text = "texts"
					return nil
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(*config)
				if !ok {
					return fmt.Errorf("unexpected type is passed: %T", cfg)
				}

				if config.text != "texts" {
					return fmt.Errorf("nexpected value is set: %s", config.text)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &CommandProps{
				botType:    "botType",
				identifier: "fileNotFound",
				// Not a pointer to the config value, but is well handled
				config: config{},
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
					return nil, nil
				},
				matchFunc: func(_ Input) bool {
					return true
				},
				instructionFunc: func(_ *HelpInput) string {
					return ""
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, cfg interface{}) error {
					config, ok := cfg.(*config) // Pointer is passed
					if !ok {
						t.Errorf("Unexpected type is passed: %T.", cfg)
						return nil
					}

					config.text = "texts"
					return nil
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(config) // Value is passed
				if !ok {
					return fmt.Errorf("unexpected type is passed: %T", cfg)
				}

				if config.text != "texts" {
					return fmt.Errorf("unexpected value is set: %s", config.text)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &CommandProps{
				botType:    "botType",
				identifier: "fileNotFound",
				config:     &config{},
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
					return nil, nil
				},
				matchFunc: func(_ Input) bool {
					return true
				},
				instructionFunc: func(_ *HelpInput) string {
					return ""
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, botType BotType, id string, cfg interface{}) error {
					return &ConfigNotFoundError{
						BotType: botType,
						ID:      id,
					}
				},
			},
			validateConfig: func(cfg interface{}) error {
				config, ok := cfg.(*config)
				if !ok {
					return fmt.Errorf("unexpected type is passed: %T", cfg)
				}

				if config.text != "" {
					return fmt.Errorf("unexpected value is set: %s", config.text)
				}

				return nil
			},
			hasErr: false,
		},
		{
			props: &CommandProps{
				botType:    "botType",
				identifier: "fileNotFound",
				config:     &config{},
				commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) {
					return nil, nil
				},
				matchFunc: func(_ Input) bool {
					return true
				},
				instructionFunc: func(_ *HelpInput) string {
					return ""
				},
			},
			watcher: &DummyConfigWatcher{
				ReadFunc: func(_ context.Context, _ BotType, _ string, _ interface{}) error {
					return errors.New("unacceptable error")
				},
			},
			hasErr: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			command, err := buildCommand(context.TODO(), tt.props, tt.watcher)
			if tt.hasErr {
				if err == nil {
					t.Error("Expected error is not returned.")
				}
				return
			}

			if command == nil {
				t.Fatal("Built command is not returned.")
			}

			if tt.props.config != nil {
				typed := command.(*defaultCommand)
				err = tt.validateConfig(typed.configWrapper.value)
				if err != nil {
					t.Error(err.Error())
				}
			}
		})
	}
}
