/*
Package fixedtimer provides example code to setup ScheduledTaskProps with fixed schedule.

The configuration struct, timerConfig, does not implement ScheduledConfig interface,
but instead fixed schedule is provided via ScheduledTaskPropsBuilder.Schedule.
Schedule never changes no matter how many times the configuration file, fixed_timer.yaml, is updated.
*/
package fixedtimer

import (
	"context"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/slack"
	"github.com/oklahomer/golack/v2/event"
)

func init() {
	sarah.RegisterScheduledTaskProps(SlackProps)
}

type timerConfig struct {
	ChannelID event.ChannelID `yaml:"channel_id"`
}

func (t *timerConfig) DefaultDestination() sarah.OutputDestination {
	return t.ChannelID
}

// SlackProps is a pre-built fixed_timer task properties for Slack.
var SlackProps = sarah.NewScheduledTaskPropsBuilder().
	BotType(slack.SLACK).
	Identifier("fixed_timer").
	ConfigurableFunc(&timerConfig{}, func(_ context.Context, config sarah.TaskConfig) ([]*sarah.ScheduledTaskResult, error) {
		return []*sarah.ScheduledTaskResult{
			{
				Content:     "Howdy!!",
				Destination: config.(*timerConfig).DefaultDestination(),
			},
		}, nil
	}).
	Schedule("@every 1m").
	MustBuild()
