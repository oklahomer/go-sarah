/*
Package echo is an reference implementation to provide the simplest form of sarah.CommandProps.

In this package two sarah.CommandPros, SlackProps and GitterProps, are exported.
Developer may import this package and refer to those sarah.CommandProps to build echo command.

  runner, err := sarah.NewRunner(config, sarah.WithCommandProps(echo.SlackProps), ...)

This example also shows the use of utility method called sarah.StripMessage,
which strips string from given message based on given regular expression.
e.g. ".echo Hey!" becomes "Hey!"
*/
package echo

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/gitter"
	"github.com/oklahomer/go-sarah/slack"
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
	return slack.NewResponse(input, commandFnc(input))
}

// GitterCommandFunc is a sarah.CommandFunc especially designed for gitter adapter.
func GitterCommandFunc(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return gitter.NewResponse(commandFnc(input))
}

// SlackProps is a pre-built echo command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(SlackCommandFunc).
	Instruction(".echo knock knock").
	MustBuild()

// GitterProps is a pre-built echo command properties for Slack.
var GitterProps = sarah.NewCommandPropsBuilder().
	BotType(gitter.GITTER).
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(GitterCommandFunc).
	Instruction(".echo knock knock").
	MustBuild()
