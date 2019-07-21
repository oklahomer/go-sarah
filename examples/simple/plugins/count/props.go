/*
Package count provides example code to setup sarah.CommandProps.

One counter instance is shared between two CommandPropsBuilder.Func,
which means resulting Slack/Gitter Commands access to same counter instance.
This illustrates that, when multiple Bots are registered to Runner, same memory space can be shared.
*/
package count

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/oklahomer/go-sarah/v2/gitter"
	"github.com/oklahomer/go-sarah/v2/slack"
	"regexp"
	"sync"
)

func init() {
	sarah.RegisterCommandProps(SlackProps)
	sarah.RegisterCommandProps(GitterProps)
}

type counter struct {
	count uint
	mutex *sync.Mutex
}

func (c *counter) increment() uint {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.count++
	return c.count
}

// globalCounter is a counter instance that is shared by both Slack command and Gitter command.
var globalCounter = &counter{
	count: 0,
	mutex: &sync.Mutex{},
}

// SlackProps is a pre-built count command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("counter").
	Instruction("Input .count to count up").
	MatchPattern(regexp.MustCompile(`^\.count`)).
	Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewResponse(input, fmt.Sprint(globalCounter.increment()))
	}).
	MustBuild()

// GitterProps is a pre-built count command properties for Slack.
var GitterProps = sarah.NewCommandPropsBuilder().
	BotType(gitter.GITTER).
	Identifier("counter").
	Instruction("Input .count to count up").
	MatchPattern(regexp.MustCompile(`^\.count`)).
	Func(func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return gitter.NewResponse(fmt.Sprint(globalCounter.increment()))
	}).
	MustBuild()
