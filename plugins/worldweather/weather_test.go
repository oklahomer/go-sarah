package worldweather

import (
	"bytes"
	"encoding/json"
	"github.com/jarcoal/httpmock"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/rtmapi"
	"github.com/oklahomer/golack/slackobject"
	"github.com/oklahomer/golack/webapi"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSlackCommandFunc(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "plugins", "worldweather", "weather.json"))
	if err != nil {
		t.Fatalf("Test file could not be located: %s.", err.Error())
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Test data could not be loaded: %s.", err.Error())
	}

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Request:    req,
			Body:       ioutil.NopCloser(bytes.NewReader(buf)),
		}, nil
	})

	response, err := SlackCommandFunc(
		context.TODO(),
		slack.NewMessageInput(
			&rtmapi.Message{
				ChannelID: slackobject.ChannelID("dummy"),
				Sender:    slackobject.UserID("user"),
				Text:      ".weather tokyo",
			},
		),
		&CommandConfig{
			APIKey: "dummy",
		},
	)

	if err != nil {
		t.Fatalf("Unexpected error was returned: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Expected response is not returned.")
	}

	if _, ok := response.Content.(*webapi.PostMessage); !ok {
		t.Errorf("Unexpected content type is returned %#v.", response.Content)
	}

	if response.UserContext != nil {
		t.Errorf("Unexpected UserContext is returned: %#v.", response.UserContext)
	}
}

func TestSlackCommandFunc_WithDataErrorAndSuccessiveAPIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	apiResponse := &LocalWeatherResponse{
		Data: &WeatherData{
			CommonData: CommonData{
				Error: []*ErrorDescription{
					{
						Message: "Location not found.",
					},
				},
			},
		},
	}
	apiResponseBytes, _ := json.Marshal(apiResponse)

	httpmock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Request:    req,
			Body:       ioutil.NopCloser(bytes.NewReader(apiResponseBytes)),
		}, nil
	})

	response, err := SlackCommandFunc(
		context.TODO(),
		slack.NewMessageInput(
			&rtmapi.Message{
				ChannelID: slackobject.ChannelID("dummy"),
				Sender:    slackobject.UserID("user"),
				Text:      ".weather tokyo",
			},
		),
		&CommandConfig{
			APIKey: "dummy",
		},
	)

	if err != nil {
		t.Fatalf("Error should not be returned even when API returns error: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Expected response is not returned.")
	}

	if _, ok := response.Content.(string); !ok {
		t.Errorf("Unexpected content type is returned %#v.", response.Content)
	}

	if response.UserContext == nil {
		t.Errorf("Expected UserContext is not returned: %#v.", response.UserContext)
	}

	// Check returned user context execution that makes another API call.

	httpmock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "500 OK",
			StatusCode: http.StatusInternalServerError,
			Request:    req,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	})

	response, err = response.UserContext.Next(
		context.TODO(),
		slack.NewMessageInput(
			&rtmapi.Message{
				ChannelID: slackobject.ChannelID("dummy"),
				Sender:    slackobject.UserID("user"),
				Text:      "tokyo",
			},
		),
	)

	if err != nil {
		t.Fatalf("Error should not be returned even when API returns error: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Expected response is not returned.")
	}

	if _, ok := response.Content.(string); !ok {
		t.Errorf("Unexpected content type is returned %#v.", response.Content)
	}

	if response.UserContext != nil {
		t.Errorf("Unexpected UserContext is returned: %#v.", response.UserContext)
	}
}
