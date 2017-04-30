package worldweather

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"golang.org/x/net/context"
	"net/http"
	"net/url"
	"testing"
)

func TestNewConfig(t *testing.T) {
	apiKey := "dummyAPIKey"

	config := NewConfig(apiKey)

	if config == nil {
		t.Fatal("Expected Config instance is not returned.")
	}

	if config.apiKey != apiKey {
		t.Errorf("Provided api key is not set: %s.", config.apiKey)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(&Config{})

	if client == nil {
		t.Fatal("Expected Client instance is not returned.")
	}
}

func TestClient_Get(t *testing.T) {
	apiType := "weather"
	data := []*struct {
		status   int
		apiType  string
		response interface{}
	}{
		{
			status:  http.StatusOK,
			apiType: "weather",
			response: &CommonData{
				Error: []*ErrorDescription{},
			},
		},
		{
			status:  http.StatusOK,
			apiType: "weather",
			response: &CommonData{
				Error: []*ErrorDescription{
					{
						Message: "API error.",
					},
				},
			},
		},
		{
			status:   http.StatusOK,
			apiType:  "weather",
			response: "bad response",
		},
		{
			status:   http.StatusInternalServerError,
			apiType:  "weather",
			response: &CommonData{},
		},
	}

	client := &Client{
		config: &Config{
			apiKey: "dummyAPIKey",
		},
	}

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for i, datum := range data {
		testNo := i + 1

		var responder httpmock.Responder
		if str, ok := datum.response.(string); ok {
			responder = httpmock.NewStringResponder(datum.status, str)
		} else {
			responder, _ = httpmock.NewJsonResponder(datum.status, datum.response)
		}
		requestURL := fmt.Sprintf(weatherAPIEndpointFormat, datum.apiType)
		httpmock.RegisterResponder("GET", requestURL, responder)

		queryParams := &url.Values{}
		queryParams.Add("q", "1600 Pennsylvania Avenue NW Washington, DC 20500")
		response := &CommonData{}
		err := client.Get(context.TODO(), apiType, queryParams, response)

		if datum.status != http.StatusOK {
			if err == nil {
				t.Errorf("Expected error is not returned on test No. %d.", testNo)
			}
			continue
		}

		expectedResponse, ok := datum.response.(*CommonData)
		if ok && expectedResponse.HasError() && !response.HasError() {
			t.Errorf("Expected error response is not returned on test No. %d.", testNo)
		} else if !ok {
			if err == nil {
				t.Errorf("Expected error is not returned on test No. %d.", testNo)
			}
		}
	}
}

func TestClient_LocalWeather(t *testing.T) {
	data := []*struct {
		status   int
		apiType  string
		response *LocalWeatherResponse
	}{
		{
			status: http.StatusOK,
			response: &LocalWeatherResponse{
				Data: &WeatherData{
					CommonData: CommonData{
						Error: []*ErrorDescription{},
					},
				},
			},
		},
		{
			status:   http.StatusInternalServerError,
			response: &LocalWeatherResponse{},
		},
	}

	client := &Client{
		config: &Config{
			apiKey: "dummyAPIKey",
		},
	}

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for i, datum := range data {
		testNo := i + 1
		requestURL := fmt.Sprintf(weatherAPIEndpointFormat, "weather")
		responder, err := httpmock.NewJsonResponder(datum.status, datum.response)
		if err != nil {
			t.Fatalf("Error on mock setup: %s.", err.Error())
		}
		httpmock.RegisterResponder("GET", requestURL, responder)

		response, err := client.LocalWeather(context.TODO(), "1600 Pennsylvania Avenue NW Washington, DC 20500")

		if datum.status == http.StatusOK {
			if err != nil {
				t.Errorf("Unexected error is returned: %s.", err.Error())
			}

			if response == nil {
				t.Errorf("Expected response is not returned on test No. %d.", testNo)
				continue
			}

			if response.Data.HasError() {
				t.Errorf("Unexpected error indication on test No. %d: %#v", testNo, response.Data)
			}

		} else {
			if err == nil {
				t.Errorf("Expected error is not returned on test No. %d.", testNo)
			}
		}
	}
}
