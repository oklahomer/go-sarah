/*
Package worldweather is a reference implementation that provides relatively practical use of sarah.CommandProps.

This illustrates the use of a user's conversational context, sarah.UserContext.
When weather API returns a response that indicates input error, this command returns text message along with a sarah.UserContext
so the user's next input will be directly fed to the designated function, which actually is equivalent to second command call in this time.
To see the detailed implementation, read the corresponding code where this command is calling slack.NewResponse.

Setup can be done by importing this package since sarah.RegisterCommandProps() is called in init().
However, to read the configuration on the fly, sarah.ConfigWatcher's implementation must be set.

  package main

  import (
    _ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/worldweather"
    "github.com/oklahomer/go-sarah/v3/watchers"
  )

  func main() {
    // setup watcher
    watcher, _ := watchers.NewFileWatcher(context.TODO(), "/path/to/config/dir/")
    sarah.RegisterConfigWatcher(watcher)

    // Do the rest

  }
*/
package worldweather

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/log"
	"github.com/oklahomer/go-sarah/v3/slack"
	"github.com/oklahomer/golack/v2/webapi"
	"regexp"
	"time"
)

func init() {
	sarah.RegisterCommandProps(SlackProps)
}

// MatchPattern defines regular expression pattern that is checked against user input
var MatchPattern = regexp.MustCompile(`^\.weather`)

// SlackProps provide a set of command configuration variables for weather command.
// Since this sets *CommandConfig in ConfigurableFunc, configuration file is observed by Runner and CommandConfig is updated on file change.
// Weather command is re-built on configuration update.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("weather").
	ConfigurableFunc(NewCommandConfig(), SlackCommandFunc).
	Instruction(`Input ".weather" followed by city name e.g. ".weather tokyo"`).
	MatchPattern(MatchPattern).
	MustBuild()

// CommandConfig contains some configuration variables for weather command.
type CommandConfig struct {
	APIKey string `yaml:"api_key"`
}

// NewCommandConfig creates and returns CommandConfig with default settings.
// To override default settings, pass the returned value to (json|yaml).Unmarshal or do this manually.
func NewCommandConfig() *CommandConfig {
	return &CommandConfig{
		APIKey: "",
	}
}

// SlackCommandFunc is a function that satisfies sarah.CommandConfig type.
// This can be fed to CommandPropsBuilder.ConfigurableFunc.
func SlackCommandFunc(ctx context.Context, input sarah.Input, config sarah.CommandConfig) (*sarah.CommandResponse, error) {
	strippedMessage := sarah.StripMessage(MatchPattern, input.Message())

	// Share client instance with later execution
	conf, _ := config.(*CommandConfig)
	client := NewClient(NewConfig(conf.APIKey))
	resp, err := client.LocalWeather(ctx, strippedMessage)

	// If error is returned with HTTP request level, just let it know and quit.
	if err != nil {
		log.Errorf("Error on weather api request: %+v", err)
		return slack.NewResponse(input, "Something went wrong with weather API request.")
	}
	// If status code of 200 is returned, which means successful API request, but still the content contains error message,
	// notify the user and put him in "the middle of conversation" for further communication.
	if resp.Data.HasError() {
		errorDescription := resp.Data.Error[0].Message
		return slack.NewResponse(
			input,
			fmt.Sprintf("Error was returned: %s.\nInput location name to retry, please.", errorDescription),
			slack.RespWithNext(func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
				return SlackCommandFunc(c, i, config)
			}),
		)
	}

	request := resp.Data.Request[0]
	currentCondition := resp.Data.CurrentCondition[0]
	forecast := resp.Data.Weather[0]
	astronomy := forecast.Astronomy[0]
	currentDesc := fmt.Sprintf("Current weather at %s is... %s.", request.Query, currentCondition.Description[0].Content)
	primaryLabelColor := "#32CD32"   // lime green
	secondaryLabelColor := "#006400" // dark green
	miscLabelColor := "#808080"

	attachments := []*webapi.MessageAttachment{
		// Current condition and overall description
		{
			Fallback: currentDesc,
			Pretext:  "Current Condition",
			Title:    currentDesc,
			Color:    primaryLabelColor,
			ImageURL: currentCondition.WeatherIcon[0].URL,
		},

		// Temperature
		{
			Fallback: fmt.Sprintf("Temperature: %d degrees Celsius.", currentCondition.Temperature),
			Title:    "Temperature",
			Color:    primaryLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Value: fmt.Sprintf("%d ℃", currentCondition.Temperature),
					Short: true,
				},
			},
		},

		// Wind speed
		{
			Fallback: fmt.Sprintf("Wind speed: %d Km/h. Direction: %s.", currentCondition.WindSpeed, currentCondition.WindDirectionCardinal),
			Title:    "Wind",
			Color:    primaryLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Title: "Speed",
					Value: fmt.Sprintf("%d km/h", currentCondition.WindSpeed),
					Short: true,
				},
				{
					Title: "Direction",
					Value: currentCondition.WindDirectionCardinal,
					Short: true,
				},
			},
		},

		// Astronomy
		{
			Fallback: fmt.Sprintf("Sunrise at %s. Sunset at %s.", astronomy.Sunrise, astronomy.Sunset),
			Pretext:  "Astronomy",
			Title:    "",
			Color:    secondaryLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Title: "Sunrise",
					Value: astronomy.Sunrise,
					Short: true,
				},
				{
					Title: "Sunset",
					Value: astronomy.Sunset,
					Short: true,
				},
				{
					Title: "Moonrise",
					Value: astronomy.MoonRise,
					Short: true,
				},
				{
					Title: "Moonset",
					Value: astronomy.MoonSet,
					Short: true,
				},
			},
		},
	}

	now := time.Now()
	for _, hourly := range forecast.Hourly {
		if now.Hour() > hourly.Time.Hour {
			continue
		}

		attachments = append(attachments, &webapi.MessageAttachment{
			Fallback: "Hourly Forecast",
			Pretext:  "Hourly Forecast for " + hourly.Time.DisplayTime,
			Title:    hourly.Description[0].Content,
			Color:    miscLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Title: "Temperature",
					Value: fmt.Sprintf("%d ℃", hourly.Temperature),
					Short: true,
				},
				{
					Title: "Precipitation",
					Value: fmt.Sprintf("%6.2f mm", hourly.Precipitation),
					Short: true,
				},
				{
					Title: "Wind Speed",
					Value: fmt.Sprintf("%d km/h", hourly.WindSpeed),
					Short: true,
				},
				{
					Title: "Wind Direction",
					Value: hourly.WindDirectionCardinal,
					Short: true,
				},
			},
			ImageURL: hourly.WeatherIcon[0].URL,
		})
	}

	return slack.NewResponse(input, "", slack.RespWithAttachments(attachments))
}
