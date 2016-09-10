package slack

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var (
	identifier = "echo"
)

func echo(_ context.Context, strippedMessage string, _ sarah.BotInput, _ sarah.CommandConfig) (*sarah.PluginResponse, error) {
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
