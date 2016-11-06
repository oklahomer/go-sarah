package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/gitter"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var (
	identifier   = "echo"
	matchPattern = regexp.MustCompile(`^\.echo`)
)

func slackEcho(_ context.Context, input sarah.Input) (*sarah.PluginResponse, error) {
	return slack.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
}

func gitterEcho(_ context.Context, input sarah.Input) (*sarah.PluginResponse, error) {
	return gitter.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
}

func init() {
	// For slack interaction
	slackBuilder := sarah.NewCommandBuilder().
		Identifier(identifier).
		MatchPattern(matchPattern).
		Func(slackEcho).
		InputExample(".echo knock knock")
	sarah.AppendCommandBuilder(slack.SLACK, slackBuilder)

	// For gitter interaction
	gitterBuilder := sarah.NewCommandBuilder().
		Identifier(identifier).
		MatchPattern(matchPattern).
		Func(gitterEcho).
		InputExample(".echo knock knock")
	sarah.AppendCommandBuilder(gitter.GITTER, gitterBuilder)
}
