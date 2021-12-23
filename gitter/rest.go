package gitter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	// RestAPIEndpoint defines base url of Gitter REST API.
	RestAPIEndpoint = "https://api.gitter.im/"
)

// RoomsFetcher defines an interface that fetches Gitter rooms.
type RoomsFetcher interface {
	// Rooms fetch the list of rooms the token's owner belongs.
	Rooms(context.Context) (*Rooms, error)
}

// RestAPIClient utilizes Gitter REST API.
type RestAPIClient struct {
	token      string
	apiVersion string
}

// NewVersionSpecificRestAPIClient creates a new API client instance with the given API version.
func NewVersionSpecificRestAPIClient(token string, apiVersion string) *RestAPIClient {
	return &RestAPIClient{
		token:      token,
		apiVersion: apiVersion,
	}
}

// NewRestAPIClient creates and returns a new API client instance. The version is fixed to v1.
func NewRestAPIClient(token string) *RestAPIClient {
	return NewVersionSpecificRestAPIClient(token, "v1")
}

func (client *RestAPIClient) buildEndpoint(resourceFragments []string) *url.URL {
	endpoint, _ := url.Parse(RestAPIEndpoint)
	fragments := append([]string{endpoint.Path, client.apiVersion}, resourceFragments...)
	endpoint.Path = path.Join(fragments...)
	return endpoint
}

// Get sends an HTTP GET request with the given path and parameters.
func (client *RestAPIClient) Get(ctx context.Context, resourceFragments []string, intf interface{}) error {
	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to construct HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")

	req = req.WithContext(ctx)

	// Do request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing HTTP request: %w", err)
	}

	defer resp.Body.Close()

	// Handle response
	err = json.NewDecoder(resp.Body).Decode(&intf)
	if err != nil {
		return fmt.Errorf("can not unmarshal given JSON structure: %w", err)
	}

	// Done
	return nil
}

// Post sends an HTTP POST request to Gitter with the given parameters.
func (client *RestAPIClient) Post(ctx context.Context, resourceFragments []string, sendingPayload interface{}, responsePayload interface{}) error {
	reqBody, err := json.Marshal(sendingPayload)
	if err != nil {
		return fmt.Errorf("can not marshal given payload: %w", err)
	}

	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("POST", endpoint.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to construct HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing HTTP request: %w", err)
	}

	defer resp.Body.Close()

	// TODO check status code

	// Handle response
	err = json.NewDecoder(resp.Body).Decode(&responsePayload)
	if err != nil {
		return fmt.Errorf("can not unmarshal given JSON structure: %w", err)
	}

	// Done
	return nil
}

// Rooms fetches belonging rooms' information.
func (client *RestAPIClient) Rooms(ctx context.Context) (*Rooms, error) {
	rooms := &Rooms{}
	if err := client.Get(ctx, []string{"rooms"}, &rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

// PostMessage sends a message to Gitter.
func (client *RestAPIClient) PostMessage(ctx context.Context, room *Room, text string) (*Message, error) {
	message := &Message{}
	err := client.Post(ctx, []string{"rooms", room.ID, "chatMessages"}, &PostingMessage{Text: text}, message)
	if err != nil {
		return nil, fmt.Errorf("failed to post message: %w", err)
	}
	return message, nil
}

// PostingMessage represents the sending message.
// This can be marshaled and be sent as a JSON-styled payload.
type PostingMessage struct {
	Text string `json:"text"`
}
