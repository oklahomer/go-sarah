// Package hello provides an example to set up a relatively simple sarah.CommandProps.
//
// In this example, instead of simply assigning a regular expression to CommandPropsBuilder.MatchPattern,
// a function with a more complex matching logic is set via CommandPropsBuilder.MatchFunc to do the equivalent task.
// One more benefit of using CommandPropsBuilder.MatchFunc is that strings package or other packages with higher performance can be used internally like this example.
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"strings"
)

func init() {
	sarah.RegisterCommandProps(SlackProps)
}

// SlackProps is a pre-built hello command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("hello").
	Instruction("Input .hello to greet").
	MatchFunc(func(input sarah.Input) bool {
		return strings.HasPrefix(input.Message(), ".hello")
	}).
	Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewResponse(input, "Hello, 世界")
	}).
	MustBuild()
