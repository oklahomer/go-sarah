/*
Package morning provides example code to setup sarah.CommandProps with relatively complex matching function.

This setting does not simply provide regular expression via CommandPropsBuilder.MatchPattern,
but instead provide whole matching function to implement complex matcher.
*/
package morning

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"strings"
	"time"
)

// SlackProps is a pre-built morning command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("morning").
	InputExample(".morning").
	MatchFunc(func(input sarah.Input) bool {
		// 1. See if the input message starts with ".morning"
		match := strings.HasPrefix(input.Message(), ".morning")
		if !match {
			return false
		}

		// 2. See if current time between 00:00 - 11:59
		hour := time.Now().Hour()
		return hour >= 0 && hour < 12
	}).
	Func(func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewStringResponse("Good morning."), nil
	}).
	MustBuild()
