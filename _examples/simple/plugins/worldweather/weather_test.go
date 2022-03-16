package worldweather

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/webapi"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSlackCommandFunc(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "testdata", "examples", "simple", "plugins", "worldweather", "weather.json"))
	if err != nil {
		t.Fatalf("Test file could not be located: %s.", err.Error())
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Test data could not be loaded: %s.", err.Error())
	}

	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Request:    req,
			Body:       io.NopCloser(bytes.NewReader(buf)),
		}, nil
	})
	defer resetClient()

	input, _ := slack.EventToInput(&event.Message{
		ChannelID: "dummy",
		UserID:    "user",
		Text:      ".weather tokyo",
	})
	response, err := SlackCommandFunc(
		context.TODO(),
		input,
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
	response := func() *sarah.CommandResponse {
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

		resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Request:    req,
				Body:       io.NopCloser(bytes.NewReader(apiResponseBytes)),
			}, nil
		})
		defer resetClient()

		input, _ := slack.EventToInput(
			&event.Message{
				ChannelID: "dummy",
				UserID:    "user",
				Text:      ".weather tokyo",
			},
		)
		response, err := SlackCommandFunc(
			context.TODO(),
			input,
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

		if _, ok := response.Content.(*webapi.PostMessage); !ok {
			t.Errorf("Unexpected content type is returned %#v.", response.Content)
		}

		if response.UserContext == nil {
			t.Errorf("Expected UserContext is not returned: %#v.", response.UserContext)
		}

		return response
	}()

	// Check returned user context execution that makes another API call.
	func() {
		switchHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:     "500 OK",
				StatusCode: http.StatusInternalServerError,
				Request:    req,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		})

		input, _ := slack.EventToInput(
			&event.Message{
				ChannelID: "dummy",
				UserID:    "user",
				Text:      "tokyo",
			},
		)
		response, err := response.UserContext.Next(
			context.TODO(),
			input,
		)

		if err != nil {
			t.Fatalf("Error should not be returned even when API returns error: %s.", err.Error())
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
	}()
}
