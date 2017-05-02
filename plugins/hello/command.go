package hello

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var slackFunc = func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewStringResponse("Hello!"), nil
}

// SlackProps provides default setup of hello command.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("hello").
	InputExample(".hello").
	MatchPattern(regexp.MustCompile(`\.hello`)).
	Func(slackFunc).
	MustBuild()
