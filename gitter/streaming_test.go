package gitter

import (
	"context"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestNewStreamingAPIConfig(t *testing.T) {
	config := NewStreamingAPIConfig()
	if config == nil {
		t.Fatal("StreamingAPIConfig is not returned.")
	}

	if config.APIVersion == "" {
		t.Error("Default API version is not set.")
	}
}

func TestStreamingClientWithHTTPClient(t *testing.T) {
	httpClient := &http.Client{}
	option := StreamingClientWithHTTPClient(httpClient)
	c := &StreamingAPIClient{}

	option(c)

	if c.httpClient != httpClient {
		t.Error("Passed HTTP client is not set.")
	}
}

func TestNewStreamingAPIClient(t *testing.T) {
	tests := []struct {
		config  *StreamingAPIConfig
		options []StreamingClientOption
	}{
		{
			config:  &StreamingAPIConfig{},
			options: []StreamingClientOption{},
		},
		{
			config: &StreamingAPIConfig{},
			options: []StreamingClientOption{
				StreamingClientWithHTTPClient(&http.Client{}),
			},
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			client := NewStreamingAPIClient(tt.config, tt.options...)
			if client == nil {
				t.Fatal("StreamingAPIClient is not returned.")
			}

			if client.httpClient == nil {
				t.Error("HTTP client is not set.")
			}

			if client.config != tt.config {
				t.Error("Passed RestAPIConfig is not set.")
			}
		})
	}
}

func TestStreamingAPIClient_buildEndPoint(t *testing.T) {
	version := "v1"
	client := &StreamingAPIClient{
		config: &StreamingAPIConfig{
			APIVersion: version,
		},
	}

	room := &Room{
		ID: "foo",
	}
	endpoint := client.buildEndpoint(room)

	if !strings.HasPrefix(endpoint.Path, "/"+version) {
		t.Errorf("Buit path does not start with version: %s.", endpoint)
	}
}

func TestStreamingAPIClient_Connect(t *testing.T) {
	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("https://stream.gitter.im/v1/rooms/foo/chatMessages")),
		}, nil
	})
	defer resetClient()

	client := &StreamingAPIClient{
		config: &StreamingAPIConfig{
			APIVersion: "v1",
			Token:      "dummy",
		},
		httpClient: http.DefaultClient,
	}

	room := &Room{
		ID: "foo",
	}
	conn, err := client.Connect(context.TODO(), room)

	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if conn == nil {
		t.Error("Connection is not returned.")
	}
}
