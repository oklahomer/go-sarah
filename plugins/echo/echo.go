package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"regexp"
)

var (
	identifier = "echo"
)

type Echo struct {
	sarah.SimpleCommand
}

func NewEcho(_ sarah.CommandConfig) sarah.Command {
	matchPattern := regexp.MustCompile(`^\.echo`)

	return &Echo{SimpleCommand: *sarah.NewSimpleCommand(identifier, ".echo Hello, world!", matchPattern)}
}

func (echo *Echo) Execute(input sarah.BotInput) (*sarah.CommandResponse, error) {
	return slack.NewStringCommandResponse(echo.StripCommand(input.GetMessage())), nil
}

func init() {
	builder := sarah.NewCommandBuilder().
		ConfigStruct(sarah.NullConfig).
		Identifier(identifier).
		Constructor(NewEcho)
	sarah.AppendCommandBuilder(slack.SLACK, builder)
}
