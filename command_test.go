package sarah

import (
	"golang.org/x/net/context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
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

func Test_buildCommand(t *testing.T) {
	config := &struct {
		Token string `yaml:"token"`
	}{
		Token: "",
	}
	props := &CommandProps{
		identifier: "dummy",
		config:     config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	_, err := buildCommand(props, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
	if config.Token != "foobar" {
		t.Error("Configuration is not read from testdata/commandbuilder/dummy.yaml file.")
	}
}

func Test_buildCommand_WithOutConfig(t *testing.T) {
	props := &CommandProps{
		botType:    "foo",
		identifier: "bar",
		commandFunc: func(_ context.Context, _ Input, config ...CommandConfig) (*CommandResponse, error) {
			return nil, nil
		},
		matchFunc: func(_ Input) bool {
			return false
		},
		instructionFunc: func(_ *HelpInput) string {
			return ".foo"
		},
		config: nil,
	}

	cmd, err := buildCommand(props, nil)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if cmd == nil {
		t.Error("Expected Command is not returned.")
	}
}

func Test_buildCommand_WithOutConfigFile(t *testing.T) {
	props := &CommandProps{
		botType:    "foo",
		identifier: "bar",
		commandFunc: func(_ context.Context, _ Input, config ...CommandConfig) (*CommandResponse, error) {
			return nil, nil
		},
		matchFunc: func(_ Input) bool {
			return false
		},
		instructionFunc: func(_ *HelpInput) string {
			return ".foo"
		},
		config: struct{}{}, // non-nil
	}

	cmd, err := buildCommand(props, nil)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if cmd == nil {
		t.Error("Expected Command is not returned.")
	}
}

func Test_buildCommand_WithConfigValue(t *testing.T) {
	type config struct {
		Token  string `yaml:"token"`
		Foo    string `yaml:"foo"`
		hidden string
	}
	// *NOT* a pointer
	c := config{
		Token:  "default",
		Foo:    "initial value",
		hidden: "hashhash",
	}
	props := &CommandProps{
		identifier: "dummy",
		config:     c,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	cmd, err := buildCommand(props, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
	if cmd.(*defaultCommand).configWrapper.value.(config).Token != "foobar" {
		t.Errorf("Configuration is not read from testdata/commandbuilder/dummy.yaml file. %#v", cmd.(*defaultCommand).configWrapper.value)
	}
	if cmd.(*defaultCommand).configWrapper.value.(config).Foo != "initial value" {
		t.Errorf("Value is lost. %#v", cmd.(*defaultCommand).configWrapper.value)
	}
}

func Test_buildCommand_WithConfigMap(t *testing.T) {
	config := map[string]interface{}{
		"token": "default",
		"foo":   "initial value",
	}
	props := &CommandProps{
		identifier: "dummy",
		config:     config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	cmd, err := buildCommand(props, file)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	configWrapper := cmd.(*defaultCommand).configWrapper
	if configWrapper == nil {
		t.Fatal("CommandConfig is not set.")
	}

	newConfig, ok := configWrapper.value.(map[string]interface{})
	if !ok {
		t.Fatalf("CommandConfig type is not valid: %T", configWrapper.value)
	}

	// Make sure original value is updated.
	v, ok := config["token"]
	if ok && v != "foobar" {
		t.Errorf("Unexpected token value is set: %s", v)
	} else if !ok {
		t.Error("Token key does not exist.")
	}

	v, ok = newConfig["token"]
	if ok && v != "foobar" {
		t.Errorf("Unexpected token value is set: %s", v)
	} else if !ok {
		t.Error("Token key does not exist.")
	}

	v, ok = newConfig["foo"]
	if ok && v != "initial value" {
		t.Errorf("Unexpected foo value is set: %s", v)
	} else if !ok {
		t.Error("Foo key does not exist.")
	}
}

func Test_buildCommand_BrokenYaml(t *testing.T) {
	config := &struct {
		Token string `yaml:"token"`
	}{
		Token: "",
	}
	props := &CommandProps{
		identifier: "broken",
		config:     config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "broken.yaml"),
		fileType: yamlFile,
	}

	_, err := buildCommand(props, file)

	if err == nil {
		t.Fatal("Error must be returned.")
	}
}

func Test_buildCommand_WithUnlocatableConfigFile(t *testing.T) {
	config := &struct {
		Token string
	}{
		Token: "presetToken",
	}
	props := &CommandProps{
		identifier: "fileNotFound",
		instructionFunc: func(_ *HelpInput) string {
			return "example"
		},
		matchFunc:   func(_ Input) bool { return true },
		commandFunc: func(_ context.Context, _ Input, _ ...CommandConfig) (*CommandResponse, error) { return nil, nil },
		config:      config,
	}
	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "fileNotFound.yaml"),
		fileType: yamlFile,
	}

	_, err := buildCommand(props, file)

	if err == nil {
		t.Fatalf("Error should be returned when expecting config file is not located.")
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
	cmd := &DummyCommand{
		IdentifierValue: "id",
		InstructionFunc: func(_ *HelpInput) string {
			return "example"
		},
	}
	commands := &Commands{collection: []Command{cmd}}

	helps := commands.Helps(&HelpInput{})
	if len(*helps) != 1 {
		t.Fatalf("Expectnig one help to be given, but was %d.", len(*helps))
	}
	if (*helps)[0].Identifier != cmd.IdentifierValue {
		t.Errorf("Expected ID was not returned: %s.", (*helps)[0].Identifier)
	}
	if (*helps)[0].Instruction != cmd.InstructionFunc(&HelpInput{}) {
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

// Test_race_commandRebuild is an integration test to detect race condition on Command (re-)build.
func Test_race_commandRebuild(t *testing.T) {
	// Prepare CommandProps
	type config struct {
		Token string
	}
	props, err := NewCommandPropsBuilder().
		Identifier("dummy").
		Instruction(".dummy").
		BotType("dummyBot").
		ConfigurableFunc(
			&config{Token: "default"},
			func(ctx context.Context, _ Input, givenConfig CommandConfig) (*CommandResponse, error) {
				_, _ = ioutil.Discard.Write([]byte(givenConfig.(*config).Token)) // Read access to config struct
				return nil, nil
			},
		).
		MatchFunc(func(_ Input) bool { return true }).
		Build()
	if err != nil {
		t.Fatalf("Error on CommnadProps preparation: %s.", err.Error())
	}

	// Prepare a bot
	commands := NewCommands()
	bot := &DummyBot{
		RespondFunc: func(ctx context.Context, input Input) error {
			_, err := commands.ExecuteFirstMatched(ctx, input)
			return err
		},
		AppendCommandFunc: func(cmd Command) {
			commands.Append(cmd)
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	file := &pluginConfigFile{
		id:       props.identifier,
		path:     filepath.Join("testdata", "command", "dummy.yaml"),
		fileType: yamlFile,
	}

	// Continuously read configuration file and re-build Command
	go func(c context.Context, b Bot, p *CommandProps) {
		for {
			select {
			case <-c.Done():
				return

			default:
				// Write
				command, err := buildCommand(p, file)
				if err == nil {
					b.AppendCommand(command)
				} else {
					t.Errorf("Error on command build: %s.", err.Error())
				}

			}
		}
	}(ctx, bot, props)

	// Continuously read config struct's field value by calling Bot.Respond
	go func(c context.Context, b Bot) {
		for {
			select {
			case <-c.Done():
				return

			default:
				_ = b.Respond(c, &DummyInput{})

			}
		}
	}(ctx, bot)

	// Wait till race condition occurs
	time.Sleep(1 * time.Second)
	cancel()
}
