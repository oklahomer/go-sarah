package worldweather

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
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

func TestClient_buildEndPoint(t *testing.T) {
	apiKey := "key"
	client := &Client{
		config: &Config{
			apiKey: apiKey,
		},
	}

	apiType := "weather"
	uri := client.buildEndpoint(apiType, nil)

	if uri == nil {
		t.Fatal("Expected *url.URL is not returned.")
	}

	keyQuery := uri.Query().Get("key")
	if keyQuery != apiKey {
		t.Errorf("Appended key parameter differs: %s.", keyQuery)
	}

	formatQuery := uri.Query().Get("format")
	if formatQuery != "json" {
		t.Errorf("Appended format query differs: %s.", formatQuery)
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

	for i, datum := range data {
		testNo := i + 1

		var res *http.Response
		if str, ok := datum.response.(string); ok {
			res = &http.Response{
				StatusCode: datum.status,
				Body:       ioutil.NopCloser(strings.NewReader(str)),
			}
		} else {
			bytes, err := json.Marshal(datum.response)
			if err != nil {
				t.Fatalf("Unexpected json marshal error: %s.", err.Error())
			}
			res = &http.Response{
				StatusCode: datum.status,
				Body:       ioutil.NopCloser(strings.NewReader(string(bytes))),
			}
		}

		func(r *http.Response) {
			resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("Unexpected request method: %s.", req.Method)
				}

				return r, nil
			})
			defer resetClient()

			queryParams := &url.Values{}
			queryParams.Add("q", "1600 Pennsylvania Avenue NW Washington, DC 20500")
			response := &CommonData{}
			err := client.Get(context.TODO(), apiType, queryParams, response)

			if datum.status != http.StatusOK {
				if err == nil {
					t.Errorf("Expected error is not returned on test No. %d.", testNo)
				}
			}

			expectedResponse, ok := datum.response.(*CommonData)
			if ok && expectedResponse.HasError() && !response.HasError() {
				t.Errorf("Expected error response is not returned on test No. %d.", testNo)
			} else if !ok {
				if err == nil {
					t.Errorf("Expected error is not returned on test No. %d.", testNo)
				}
			}
		}(res)
	}
}

func TestClient_GetRequestError(t *testing.T) {
	client := &Client{
		config: &Config{
			apiKey: "dummyAPIKey",
		},
	}

	expectedErr := &url.Error{
		Op:  "dummy",
		URL: "http://sample.com/",
		Err: errors.New("dummy error"),
	}

	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		return nil, expectedErr
	})
	defer resetClient()

	response := &CommonData{}
	err := client.Get(context.TODO(), "weather", nil, response)

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Errorf("Unexpected error is returned: %#v.", err)
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

	for i, datum := range data {
		testNo := i + 1

		func(n int) {
			resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("Unexpected request method: %s.", req.Method)
				}

				bytes, err := json.Marshal(datum.response)
				if err != nil {
					t.Fatalf("Unexpected json marshal error: %s.", err.Error())
				}

				return &http.Response{
					StatusCode: datum.status,
					Body:       ioutil.NopCloser(strings.NewReader(string(bytes))),
				}, nil
			})
			defer resetClient()

			response, err := client.LocalWeather(context.TODO(), "1600 Pennsylvania Avenue NW Washington, DC 20500")

			if datum.status == http.StatusOK {
				if err != nil {
					t.Errorf("Unexected error is returned: %s.", err.Error())
				}

				if response == nil {
					t.Errorf("Expected response is not returned on test No. %d.", n)
					return
				}

				if response.Data.HasError() {
					t.Errorf("Unexpected error indication on test No. %d: %#v", n, response.Data)
				}

			} else {
				if err == nil {
					t.Errorf("Expected error is not returned on test No. %d.", n)
				}
			}
		}(testNo)
	}
}

type roundTripFnc func(*http.Request) (*http.Response, error)

func (fnc roundTripFnc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fnc(r)
}

func switchHTTPClient(fnc roundTripFnc) func() {
	oldClient := http.DefaultClient

	http.DefaultClient = &http.Client{
		Transport: fnc,
	}

	return func() {
		http.DefaultClient = oldClient
	}
}
