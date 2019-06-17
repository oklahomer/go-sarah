package gitter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const (
	// StreamingAPIEndpointFormat defines basic url format of gitter streaming API.
	StreamingAPIEndpointFormat = "https://stream.gitter.im/%s/rooms/%s/chatMessages"
)

// StreamConnector defines an interface that connects to given gitter room
type StreamConnector interface {
	Connect(context.Context, *Room) (Connection, error)
}

// StreamingAPIClient utilizes gitter streaming API.
type StreamingAPIClient struct {
	token      string
	apiVersion string
}

// NewVersionSpecificStreamingAPIClient creates and returns API client instance.
func NewVersionSpecificStreamingAPIClient(apiVersion string, token string) *StreamingAPIClient {
	return &StreamingAPIClient{
		token:      token,
		apiVersion: apiVersion,
	}
}

// NewStreamingAPIClient creates and returns API client instance.
// API version is fixed to v1.
func NewStreamingAPIClient(token string) *StreamingAPIClient {
	return NewVersionSpecificStreamingAPIClient("v1", token)
}

func (client *StreamingAPIClient) buildEndpoint(room *Room) *url.URL {
	endpoint, _ := url.Parse(fmt.Sprintf(StreamingAPIEndpointFormat, client.apiVersion, room.ID))
	return endpoint
}

// Connect initiates request to streaming API and returns established connection.
func (client *StreamingAPIClient) Connect(ctx context.Context, room *Room) (Connection, error) {
	requestURL := client.buildEndpoint(room)

	// Set up sending request
	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(ctx)

	// Do request
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	return newConnWrapper(room, resp.Body), nil
}
