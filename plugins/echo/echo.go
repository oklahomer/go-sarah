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

var commandFnc = func(input sarah.Input) string {
	return sarah.StripMessage(matchPattern, input.Message())
}

// SlackCommandFunc is a sarah.CommandFunc especially designed for Slack adapter.
func SlackCommandFunc(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewStringResponse(commandFnc(input)), nil
}

// GitterCommandFunc is a sarah.CommandFunc especially designed for gitter adapter.
func GitterCommandFunc(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return gitter.NewStringResponse(commandFnc(input)), nil
}

// SlackProps is a pre-built echo command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(SlackCommandFunc).
	InputExample(".echo knock knock").
	MustBuild()

// GitterProps is a pre-built echo command properties for Slack.
var GitterProps = sarah.NewCommandPropsBuilder().
	BotType(gitter.GITTER).
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(GitterCommandFunc).
	InputExample(".echo knock knock").
	MustBuild()
