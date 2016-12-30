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

var (
	identifier   = "weather"
	matchPattern = regexp.MustCompile(`^\.weather`)
)

type pluginConfig struct {
	APIKey string `yaml:"api_key"`
}

func weather(ctx context.Context, input sarah.Input, config sarah.CommandConfig) (*sarah.CommandResponse, error) {
	strippedMessage := sarah.StripMessage(matchPattern, input.Message())

	// Share client instance with later execution
	conf, _ := config.(*pluginConfig)
	client := NewClient(NewConfig(conf.APIKey))
	resp, err := client.LocalWeather(ctx, strippedMessage)

	// If error is returned with HTTP request level, just let it know and quit.
	if err != nil {
		log.Errorf("Error on weather api reqeust: %s.", err.Error())
		return sarah.NewStringResponse("Something went wrong with weather api request."), nil
	}
	// If status code of 200 is returned, which means successful API request, but still the content contains error message,
	// notify the user and put him in "the middle of conversation" for further communication.
	if resp.Data.HasError() {
		errorDescription := resp.Data.Error[0].Message
		return sarah.NewStringResponseWithNext(
			fmt.Sprintf("Error was returned: %s.\nInput location name to retry, please.", errorDescription),
			func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
				return weather(c, i, config)
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

func init() {
	builder := sarah.NewCommandBuilder().
		Identifier(identifier).
		MatchPattern(matchPattern).
		ConfigurableFunc(&pluginConfig{}, weather).
		InputExample(".weather")
	sarah.StashCommandBuilder(slack.SLACK, builder)
}
