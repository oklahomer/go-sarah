package sarah

import (
	"golang.org/x/net/context"
	"regexp"
	"strings"
	"testing"
)

type DummyCommand struct {
	IdentifierValue string

	ExecuteFunc func(context.Context, Input) (*CommandResponse, error)

	InputExampleFunc func() string

	MatchFunc func(string) bool
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

func TestInsufficientSettings(t *testing.T) {
	matchPattern := regexp.MustCompile(`^\.echo`)

	builder := NewCommandBuilder().
		Identifier("someID").
		MatchPattern(matchPattern).
		InputExample(".echo knock knock")

	if _, err := builder.Build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else {
		if err != ErrCommandInsufficientArgument {
			t.Errorf("expected error not given. %#v", err)
		}
	}

	builder.Func(func(_ context.Context, input Input) (*CommandResponse, error) {
		return &CommandResponse{
			Content: StripMessage(matchPattern, input.Message()),
		}, nil
	})

	if _, err := builder.Build(""); err != nil {
		t.Errorf("something is wrong with command construction. %#v", err)
	}
}

func TestCommands_FindFirstMatched(t *testing.T) {
	commands := NewCommands()

	irrelevantCommand := &DummyCommand{}
	irrelevantCommand.MatchFunc = func(msg string) bool {
		return false
	}
	commands.Append(irrelevantCommand)

	echoCommand := &DummyCommand{}
	echoCommand.MatchFunc = func(msg string) bool {
		return strings.HasPrefix(msg, "echo")
	}
	echoCommand.ExecuteFunc = func(_ context.Context, _ Input) (*CommandResponse, error) {
		return &CommandResponse{Content: ""}, nil
	}
	commands.Append(echoCommand)

	irrelevantCommand2 := &DummyCommand{}
	irrelevantCommand2.MatchFunc = func(msg string) bool {
		return false
	}
	commands.Append(irrelevantCommand2)

	matchedCommand := commands.FindFirstMatched("echo")
	if matchedCommand == nil {
		t.Fatal("Expected command is not found.")
	}

	if matchedCommand != echoCommand {
		t.Fatalf("Expected command instance not returned: %#v.", matchedCommand)
	}

	input := &DummyInput{}
	input.MessageValue = "echo foo"

	response, err := commands.ExecuteFirstMatched(context.TODO(), input)
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
