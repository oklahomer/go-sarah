/*
Package timer provides example code to setup ScheduledTaskProps with re-configurable schedule and sending room.

The configuration struct, timerConfig, implements both ScheduledConfig and DestinatedConfig interface.
The configuration values are read from timer.yaml and Command is re-built when configuration file is updated.
*/
package timer

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/slackobject"
	"golang.org/x/net/context"
)

type timerConfig struct {
	TaskSchedule string                `yaml:"schedule"`
	ChannelID    slackobject.ChannelID `yaml:"channel_id"`
}

func (t *timerConfig) Schedule() string {
	return t.TaskSchedule
}

func (t *timerConfig) DefaultDestination() sarah.OutputDestination {
	return t.ChannelID
}

// SlackProps is a pre-built timer task properties for Slack.
var SlackProps = sarah.NewScheduledTaskPropsBuilder().
	BotType(slack.SLACK).
	Identifier("timer").
	ConfigurableFunc(&timerConfig{}, func(_ context.Context, config sarah.TaskConfig) ([]*sarah.ScheduledTaskResult, error) {
		return []*sarah.ScheduledTaskResult{
			{
				Content:     "It's time to work!!",
				Destination: config.(*timerConfig).DefaultDestination(),
			},
		}, nil
	}).
	MustBuild()

func init() {
	sarah.RegisterScheduledTaskProps(SlackProps)
}
