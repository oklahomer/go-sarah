package slack

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var (
	identifier   = "echo"
	matchPattern = regexp.MustCompile(`^\.echo`)
)

func echo(_ context.Context, input sarah.Input, _ sarah.CommandConfig) (*sarah.PluginResponse, error) {
	return slack.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
}

func init() {
	builder := sarah.NewCommandBuilder().
		Identifier(identifier).
		ConfigStruct(sarah.NullConfig).
		MatchPattern(matchPattern).
		Func(echo).
		Example(".echo knock knock")
	sarah.AppendCommandBuilder(slack.SLACK, builder)
}
