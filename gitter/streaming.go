package gitter

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
	"net/http"
	"net/url"
)

const (
	// StreamingAPIEndpointFormat defines basic url format of gitter streaming API.
	StreamingAPIEndpointFormat = "https://stream.gitter.im/%s/rooms/%s/chatMessages"
)

// StreamingAPIConfig represents configuration value for Rest API.
type StreamingAPIConfig struct {
	Token      string `json:"token" yaml:"token"`
	APIVersion string `json:"api_version" yaml:"api_version"`
}

// NewStreamingAPIConfig returns initialized configuration struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal
// or by direct assignment.
func NewStreamingAPIConfig() *StreamingAPIConfig {
	return &StreamingAPIConfig{
		APIVersion: "v1",
	}
}

// StreamingClientOption defines function signature that StreamingAPIClient's functional option must satisfy.
type StreamingClientOption func(*StreamingAPIClient)

// StreamingClientWithHTTPClient replaces the http.DefaultClient with customized one.
func StreamingClientWithHTTPClient(httpClient *http.Client) StreamingClientOption {
	return func(client *StreamingAPIClient) {
		client.httpClient = httpClient
	}
}

// StreamConnector defines an interface that connects to given gitter room
type StreamConnector interface {
	Connect(context.Context, *Room) (Connection, error)
}

// StreamingAPIClient utilizes gitter streaming API.
type StreamingAPIClient struct {
	config     *StreamingAPIConfig
	httpClient *http.Client
}

// NewStreamingAPIClient creates and returns API client instance.
// API version is fixed to v1.
func NewStreamingAPIClient(config *StreamingAPIConfig, options ...StreamingClientOption) *StreamingAPIClient {
	client := &StreamingAPIClient{
		config:     config,
		httpClient: http.DefaultClient,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

func (client *StreamingAPIClient) buildEndpoint(room *Room) *url.URL {
	endpoint, _ := url.Parse(fmt.Sprintf(StreamingAPIEndpointFormat, client.config.APIVersion, room.ID))
	return endpoint
}

// Connect initiates request to streaming API and returns established connection.
func (client *StreamingAPIClient) Connect(ctx context.Context, room *Room) (Connection, error) {
	requestURL := client.buildEndpoint(room)

	// Set up sending request
	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, xerrors.Errorf("failed creating new request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.config.Token))
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(ctx)

	// Do request
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, xerrors.Errorf("failed sending HTTP request: %w", err)
	}

	return newConnWrapper(room, resp.Body), nil
}
