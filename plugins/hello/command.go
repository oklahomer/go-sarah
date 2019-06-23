/*
Package hello is an reference implementation to provide the simplest form of sarah.CommandProps.

Developer may import this package and refer to hello.SlackProps to build hello command.

  runner, err := sarah.NewRunner(config, sarah.WithCommandProps(hello.SlackProps), ...)
*/
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"regexp"
)

var slackFunc = func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewResponse(input, "Hello!")
}

// SlackProps provides default setup of hello command.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("hello").
	Instruction("Input .hello to greet.").
	MatchPattern(regexp.MustCompile(`\.hello`)).
	Func(slackFunc).
	MustBuild()
