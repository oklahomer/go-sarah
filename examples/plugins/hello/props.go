/*
Package hello provides example code to setup relatively simple sarah.CommandProps.

This sarah.CommandProps can be fed to Runner.New as below.

  runner, err := sarah.NewRunner(config.Runner, sarah.WithCommandProps(hello.SlackProps), ... )
*/
package hello

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

// SlackProps is a pre-built hello command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("hello").
	InputExample(".hello").
	MatchPattern(regexp.MustCompile(`^\.hello`)).
	Func(func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewStringResponse("Hello, 世界"), nil
	}).
	MustBuild()
