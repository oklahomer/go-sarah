package sarah

import (
	"golang.org/x/net/context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type DummyCommand struct {
	IdentifierValue  string
	ExecuteFunc      func(context.Context, Input) (*CommandResponse, error)
	InputExampleFunc func() string
	MatchFunc        func(string) bool
}

func (command *DummyCommand) Identifier() string {
	return command.IdentifierValue
}

func (command *DummyCommand) Execute(ctx context.Context, input Input) (*CommandResponse, error) {
	return command.ExecuteFunc(ctx, input)
}

func (command *DummyCommand) InputExample() string {
	return command.InputExampleFunc()
}

func (command *DummyCommand) Match(str string) bool {
	return command.MatchFunc(str)
}

func TestNewCommandBuilder(t *testing.T) {
	builder := NewCommandBuilder()
	if builder == nil {
		t.Fatal("NewCommandBuilder returned nil.")
	}
}

func TestCommandBuilder_ConfigurableFunc(t *testing.T) {
	wrappedFncCalled := false
	config := &struct{}{}
	fnc := func(_ context.Context, _ Input, passedConfig CommandConfig) (*CommandResponse, error) {
		wrappedFncCalled = true
		if passedConfig != config {
			t.Errorf("Passed config is not the expected one: %#v", passedConfig)
		}
		return nil, nil
	}

	builder := &CommandBuilder{}
	builder.ConfigurableFunc(config, fnc)
	if builder.config != config {
		t.Error("Passed config struct is not set.")
	}

	builder.commandFunc(context.TODO(), &DummyInput{}, config)
	if wrappedFncCalled == false {
		t.Error("Provided func was not properlly wrapped in builder.")
	}
}

func TestCommandBuilder_Func(t *testing.T) {
	wrappedFncCalled := false
	builder := &CommandBuilder{}
	fnc := func(_ context.Context, _ Input) (*CommandResponse, error) {
		wrappedFncCalled = true
		return nil, nil
	}

	builder.Func(fnc)
	builder.commandFunc(context.TODO(), &DummyInput{})
	if wrappedFncCalled == false {
		t.Error("Provided func was not properlly wrapped in builder.")
	}
}

func TestCommandBuilder_Identifier(t *testing.T) {
	builder := &CommandBuilder{}
	id := "FOO"
	builder.Identifier(id)

	if builder.identifier != id {
		t.Error("Provided identifier is not set.")
	}
}

func TestCommandBuilder_InputExample(t *testing.T) {
	builder := &CommandBuilder{}
	example := ".echo foo"
	builder.InputExample(example)

	if builder.example != example {
		t.Error("Provided example is not set.")
	}
}

func TestCommandBuilder_MatchPattern(t *testing.T) {
	builder := &CommandBuilder{}
	pattern := regexp.MustCompile(`^\.echo`)
	builder.MatchPattern(pattern)

	if builder.matchPattern != pattern {
		t.Error("Provided match pattern is not set.")
	}
}

func TestCommandBuilder_Build(t *testing.T) {
	builder := &CommandBuilder{}
	if _, err := builder.Build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else if err != ErrCommandInsufficientArgument {
		t.Errorf("expected error not given. %#v", err)
	}

	matchPattern := regexp.MustCompile(`^\.echo`)
	builder.Identifier("dummy").
		MatchPattern(matchPattern).
		InputExample(".echo knock knock")

	// When corresponding configuration file is not found, then manually set schedule must stay.
	dummyToken := "dummy"
	config := &struct {
		Token string `yaml:"token"`
	}{
		Token: dummyToken,
	}
	builder.ConfigurableFunc(config, func(_ context.Context, input Input, passedConfig CommandConfig) (*CommandResponse, error) {
		return &CommandResponse{
			Content: StripMessage(matchPattern, input.Message()),
		}, nil
	})

	command, err := builder.Build(filepath.Join("unknown", "path", "foo"))
	if err != nil {
		t.Fatal("Error on command construction with no config file.")
	}
	if config.Token != dummyToken {
		t.Errorf("Config value changed: %s.", config.Token)
	}

	command, err = builder.Build(filepath.Join("testdata", "commandbuilder"))
	if err != nil {
		t.Errorf("something is wrong with command construction. %#v", err)
	}

	if command == nil {
		t.Fatal("Built command is not returned.")
	}

	if _, ok := command.(*simpleCommand); !ok {
		t.Fatalf("Returned command is not type of *simpleCommand: %T.", command)
	}

	if config.Token != "foobar" {
		t.Error("Configuration is not read from testdata/commandbuilder/dummy.yaml file.")
	}
}

func TestCommandBuilder_Build_BrokenYaml(t *testing.T) {
	builder := &CommandBuilder{}
	builder.Identifier("broken").
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		InputExample(".echo knock knock")

	config := &struct {
		Token string `yaml:"token"`
	}{}
	builder.ConfigurableFunc(config, func(_ context.Context, input Input, passedConfig CommandConfig) (*CommandResponse, error) {
		return &CommandResponse{
			Content: "",
		}, nil
	})

	command, err := builder.Build(filepath.Join("testdata", "commandbuilder"))
	if err == nil {
		t.Fatal("Error must be returned")
	}
	if command != nil {
		t.Fatal("Expected nil command, but was not")
	}
}

func TestCommandBuilder_MustBuild(t *testing.T) {
	builder := &CommandBuilder{}
	builder.Identifier("dummy").
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		InputExample(".echo knock knock")

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
	command := builder.MustBuild()
	if command.Identifier() != builder.identifier {
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
	matchedCommand := commands.FindFirstMatched("echo")
	if matchedCommand != nil {
		t.Fatalf("Something is returned while nothing other than nil may returned: %#v.", matchedCommand)
	}

	irrelevantCommand := &DummyCommand{}
	irrelevantCommand.MatchFunc = func(msg string) bool {
		return false
	}
	echoCommand := &DummyCommand{}
	echoCommand.MatchFunc = func(msg string) bool {
		return strings.HasPrefix(msg, "echo")
	}
	echoCommand.ExecuteFunc = func(_ context.Context, _ Input) (*CommandResponse, error) {
		return &CommandResponse{Content: ""}, nil
	}
	irrelevantCommand2 := &DummyCommand{}
	irrelevantCommand2.MatchFunc = func(msg string) bool {
		return false
	}
	commands = &Commands{irrelevantCommand, echoCommand, irrelevantCommand2}

	matchedCommand = commands.FindFirstMatched("echo")
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
	echoCommand.MatchFunc = func(msg string) bool {
		return strings.HasPrefix(msg, "echo")
	}
	echoCommand.ExecuteFunc = func(_ context.Context, _ Input) (*CommandResponse, error) {
		return &CommandResponse{Content: ""}, nil
	}
	commands = &Commands{echoCommand}
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
	if len(*commands) == 0 {
		t.Fatal("Provided command was not appended.")
	}

	if (*commands)[0] != command {
		t.Fatalf("Appended command is not the one provided: %#v", (*commands)[0])
	}

	// Second operation with same command
	commands.Append(command)
	if len(*commands) != 1 {
		t.Fatalf("Expected only one command to stay, but was: %d.", len(*commands))
	}

	// Third operation with different command
	anotherCommand := &DummyCommand{
		IdentifierValue: "second",
	}
	commands.Append(anotherCommand)
	if len(*commands) != 2 {
		t.Fatalf("Expected 2 commands to stay, but was: %d.", len(*commands))
	}
}

func TestCommands_Helps(t *testing.T) {
	cmd := &DummyCommand{
		IdentifierValue: "id",
		InputExampleFunc: func() string {
			return "example"
		},
	}
	commands := &Commands{cmd}

	helps := commands.Helps()
	if len(*helps) != 1 {
		t.Fatalf("Expectnig one help to be given, but was %d.", len(*helps))
	}
	if (*helps)[0].Identifier != cmd.IdentifierValue {
		t.Errorf("Expected ID was not returned: %s.", (*helps)[0].Identifier)
	}
	if (*helps)[0].InputExample != cmd.InputExampleFunc() {
		t.Errorf("Expected example was not returned: %s.", (*helps)[0].InputExample)
	}
}

func TestSimpleCommand_Identifier(t *testing.T) {
	id := "bar"
	command := simpleCommand{identifier: id}

	if command.Identifier() != id {
		t.Errorf("Stored identifier is not returned: %s.", command.Identifier())
	}
}

func TestSimpleCommand_InputExample(t *testing.T) {
	example := "example foo"
	command := simpleCommand{example: example}

	if command.InputExample() != example {
		t.Errorf("Stored example is not returned: %s.", command.Identifier())
	}
}

func TestSimpleCommand_Match(t *testing.T) {
	pattern := regexp.MustCompile(`^\.echo`)
	command := simpleCommand{matchPattern: pattern}

	if command.Match(".echo foo") == false {
		t.Error("Expected match result is not returned.")
	}
}

func TestSimpleCommand_Execute(t *testing.T) {
	wrappedFncCalled := false
	command := simpleCommand{
		config: &struct{}{},
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
