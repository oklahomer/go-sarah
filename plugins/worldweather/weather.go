/*
Package worldweather is an reference implementation that provides relatively practical sarah.CommandProps.

This illustrates the use of user's conversational context, sarah.UserContext.
When weather API returns response that indicates input error, this command returns text message along with a sarah.UserContext
so the user's next input will be directly fed to the designated function, which actually is equivalent to second command call in this time.
To see detailed implementation, read corresponding code where this command is calling slack.NewStringResponseWithNext.

When this sarah.CommandProps is passed to sarah.RegisterCommandProps, go-sarah tries to read configuration file and map content to weather.CommandConfig.
Setup should be somewhat like below:

  sarah.RegisterCommandProps(hello.SlackProps)
  sarah.RegisterCommandProps(echo.SlackProps)
  sarah.RegisterCommandProps(worldweather.SlackProps)

  // Config.PluginConfigRoot must be set to read configuration file for this command.
  // Runner searches for configuration file located at config.PluginConfigRoot + "/slack/weather.(yaml|yml|json)".
  config := sarah.NewConfig()
  config.PluginConfigRoot = "/path/to/config/" // Or do yaml.Unmarshal(fileBuf, config), json.Unmarshal(fileBuf, config)
  err := sarah.Run(context.TODO(), config)
*/
package worldweather

import (
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/webapi"
	"golang.org/x/net/context"
	"regexp"
	"time"
)

// MatchPattern defines regular expression pattern that is checked against user input
var MatchPattern = regexp.MustCompile(`^\.weather`)

// SlackProps provide a set of command configuration variables for weather command.
// Since this sets *CommandConfig in ConfigurableFunc, configuration file is observed by Runner and CommandConfig is updated on file change.
// Weather command is re-built on configuration update.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("weather").
	ConfigurableFunc(NewCommandConfig(), SlackCommandFunc).
	InputExample(".weather tokyo").
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
		log.Errorf("Error on weather api reqeust: %s.", err.Error())
		return slack.NewStringResponse("Something went wrong with weather api request."), nil
	}
	// If status code of 200 is returned, which means successful API request, but still the content contains error message,
	// notify the user and put him in "the middle of conversation" for further communication.
	if resp.Data.HasError() {
		errorDescription := resp.Data.Error[0].Message
		return slack.NewStringResponseWithNext(
			fmt.Sprintf("Error was returned: %s.\nInput location name to retry, please.", errorDescription),
			func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
				return SlackCommandFunc(c, i, config)
			},
		), nil
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

	return slack.NewPostMessageResponse(input, "", attachments), nil
}
