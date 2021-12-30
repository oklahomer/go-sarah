// Package morning provides an example to set up sarah.CommandProps with a relatively complex matching function.
//
// This setting does not simply provide a regular expression via CommandPropsBuilder.MatchPattern,
// but instead provide the whole matching function to implement a complex matcher.
package morning

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"strings"
	"time"
)

func init() {
	sarah.RegisterCommandProps(SlackProps)
}

// SlackProps is a pre-built morning command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("morning").
	InstructionFunc(func(input *sarah.HelpInput) string {
		hour := time.Now().Hour()
		if 12 < hour {
			// This command is only active in the morning.
			// Do not show instruction in the afternoon.
			return ""
		}

		return "Input .morning to greet."
	}).
	MatchFunc(func(input sarah.Input) bool {
		// 1. See if the input message starts with ".morning"
		match := strings.HasPrefix(input.Message(), ".morning")
		if !match {
			return false
		}

		// 2. See if the current time is between 00:00 - 11:59
		hour := time.Now().Hour()
		return hour >= 0 && hour < 12
	}).
	Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewResponse(input, "Good morning.")
	}).
	MustBuild()
