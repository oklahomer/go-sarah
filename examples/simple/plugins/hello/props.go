/*
Package hello provides example code to setup relatively simple sarah.CommandProps.

In this example, instead of simply assigning regular expression to CommandPropsBuilder.MatchPattern,
a function is set via CommandPropsBuilder.MatchFunc to do the equivalent task.
With CommandPropsBuilder.MatchFunc, developers may define more complex matching logic than assigning simple regular expression to CommandPropsBuilder.MatchPattern.
One more benefit is that strings package or other packages with higher performance can be used internally like this example.

This sarah.CommandProps can be fed to sarah.NewRunner() as below.

  runner, err := sarah.NewRunner(config.Runner, sarah.WithCommandProps(hello.SlackProps), ... )
*/
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"strings"
)

// SlackProps is a pre-built hello command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("hello").
	Instruction("Input .hello to greet").
	MatchFunc(func(input sarah.Input) bool {
		return strings.HasPrefix(input.Message(), ".hello")
	}).
	Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewResponse(input, "Hello, 世界", slack.RespAsThreadReply(true))
	}).
	MustBuild()

func init() {
	sarah.RegisterCommandProps(SlackProps)
}
