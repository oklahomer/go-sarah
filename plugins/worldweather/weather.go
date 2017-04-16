package worldweather

import (
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/webapi"
	"golang.org/x/net/context"
	"regexp"
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
// This can be fed to CommandBuilder.ConfigurableFunc.
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
			Fallback: fmt.Sprintf("Temperature: %s degrees Celsius.", currentCondition.Temperature),
			Title:    "Temperature",
			Color:    primaryLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Title: "Celsius",
					Value: string(currentCondition.Temperature),
					Short: true,
				},
			},
		},

		// Wind speed
		{
			Fallback: fmt.Sprintf("Wind speed: %s Km/h", currentCondition.WindSpeed),
			Title:    "Wind Speed",
			Color:    primaryLabelColor,
			Fields: []*webapi.AttachmentField{
				{
					Title: "kn/h",
					Value: string(currentCondition.WindSpeed),
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

	return slack.NewPostMessageResponse(input, "", attachments), nil
}
