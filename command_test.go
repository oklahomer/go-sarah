package sarah

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestInsufficientSettings(t *testing.T) {
	builder := NewCommandBuilder().
		Identifier("someID").
		ConfigStruct(NullConfig).
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		Example(".echo knock knock")

	if _, err := builder.build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else {
		switch err.(type) {
		case *CommandInsufficientArgumentError:
		// O.K.
		default:
			t.Errorf("expected error not given. %#v", err)
		}
	}

	builder.Func(func(strippedMessage string, input BotInput, _ CommandConfig) (*PluginResponse, error) {
		return &PluginResponse{
			Content: strippedMessage,
		}, nil
	})

	if _, err := builder.build(""); err != nil {
		t.Errorf("something is wrong with command construction. %#v", err)
	}
}

type abandonedCommand struct{}

func (abandonedCommand *abandonedCommand) Identifier() string {
	return "arbitraryStringThatWouldNeverBeRecognized"
}

func (abandonedCommand *abandonedCommand) Execute(_ string, _ BotInput) (*PluginResponse, error) {
	return nil, nil
}

func (abandonedCommand *abandonedCommand) Example() string {
	return ""
}

func (abandonedCommand *abandonedCommand) Match(_ string) bool {
	return false
}

func (abandonedCommand *abandonedCommand) StripMessage(_ string) string {
	return ""
}

type echoCommand struct{}

func (echoCommand *echoCommand) Identifier() string {
	return "echo"
}

func (echoCommand *echoCommand) Execute(strippedMessage string, input BotInput) (*PluginResponse, error) {
	return &PluginResponse{Content: input.GetMessage()}, nil
}

func (echoCommand *echoCommand) Example() string {
	return ""
}

func (echoCommand *echoCommand) Match(msg string) bool {
	return strings.HasPrefix(msg, "echo")
}

func (echoCommand *echoCommand) StripMessage(msg string) string {
	return strings.TrimPrefix(msg, "echo")
}

type echoInput struct{}

func (echoInput *echoInput) GetSenderID() string {
	return ""
}

func (echoInput *echoInput) GetMessage() string {
	return "echo foo"
}

func (echoInput *echoInput) GetSentAt() time.Time {
	return time.Now()
}

func (echoInput *echoInput) ReplyTo() OutputDestination {
	return nil
}

func TestCommands_FindFirstMatched(t *testing.T) {
	commands := NewCommands()
	commands.Append(&abandonedCommand{})
	commands.Append(&echoCommand{})
	commands.Append(&abandonedCommand{})

	echo := commands.FindFirstMatched("echo")
	if echo == nil {
		t.Error("expected command is not found")
		return
	}

	switch echo.(type) {
	case *echoCommand:
	// O.K.
	default:
		t.Errorf("expecting echoCommand's pointer, but was %#v.", echo)
		return
	}

	response, err := commands.ExecuteFirstMatched(&echoInput{})
	if err != nil {
		t.Errorf("unexpected error on commands execution: %#v", err)
		return
	}

	if response == nil {
		t.Error("response expected, but was not returned")
		return
	}

	switch v := response.Content.(type) {
	case string:
	//OK
	default:
		t.Errorf("expected string, but was %#v", v)
	}
}
