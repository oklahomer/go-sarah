package line

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	if config == nil {
		t.Fatal("Config struct is not retuned.")
	}

	if config.RequestTimeout == 0 {
		t.Error("Timeout value is not set.")
	}

	if config.Token != "" {
		t.Errorf("Token value is set: %s.", config.Token)
	}
}

func TestWithHTTPClient(t *testing.T) {
	httpClient := &http.Client{}
	option := WithHTTPClient(httpClient)
	client := &Client{}

	option(client)

	if client.httpClient != httpClient {
		t.Error("Expected http client is not set.")
	}
}

func TestNew(t *testing.T) {
	optCalled := false
	config := NewConfig()
	client := New(config, func(_ *Client) {
		optCalled = true
	})

	if client == nil {
		t.Fatal("Client struct is not returned.")
	}

	if client.config == nil {
		t.Fatal("Config is not set.")
	}

	if !optCalled {
		t.Error("Given Option is not applied.")
	}
}

func TestClient_Alert(t *testing.T) {
	responses := []*struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		{
			Status:  403,
			Message: "forbidden",
		},
		{
			Status:  200,
			Message: "ok",
		},
	}

	for _, r := range responses {
		httpClient := &http.Client{
			Transport: roundTripFnc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "POST" {
					t.Fatalf("Unexpected request method: %s.", req.Method)
				}

				bytes, err := json.Marshal(r)
				if err != nil {
					t.Fatalf("Unexpected json marshal error: %s.", err.Error())
				}
				return &http.Response{
					StatusCode: r.Status,
					Body:       ioutil.NopCloser(strings.NewReader(string(bytes))),
				}, nil
			}),
		}

		client := &Client{
			config: &Config{
				RequestTimeout: 3 * time.Second,
				Token:          "dummy",
			},
			httpClient: httpClient,
		}
		err := client.Alert(context.TODO(), "DUMMY", errors.New("message"))
		if r.Status == 200 && err != nil {
			t.Errorf("Unexpected error is returned: %s.", err.Error())
		} else if r.Status != 200 && err == nil {
			t.Error("Expected error is not returned.")
		}
	}
}

type roundTripFnc func(*http.Request) (*http.Response, error)

func (fnc roundTripFnc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fnc(r)
}
