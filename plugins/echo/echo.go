package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"regexp"
)

var (
	identifier = "echo"
)

func echo(strippedMessage string, _ sarah.BotInput, _ sarah.CommandConfig) (*sarah.PluginResponse, error) {
	return slack.NewStringPluginResponse(strippedMessage), nil
}

func init() {
	builder := sarah.NewCommandBuilder().
		Identifier(identifier).
		ConfigStruct(sarah.NullConfig).
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		Func(echo).
		Example(".echo knock knock")
	sarah.AppendCommandBuilder(slack.SLACK, builder)
}
