package gitter

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/xerrors"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	// RestAPIEndpoint defines base url of gitter REST API.
	RestAPIEndpoint = "https://api.gitter.im/"
)

// RestAPIConfig represents configuration value for Rest API.
type RestAPIConfig struct {
	Token      string `json:"token" yaml:"token"`
	APIVersion string `json:"api_version" yaml:"api_version"`
}

// NewRestAPIConfig returns initialized configuration struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal
// or by direct assignment.
func NewRestAPIConfig() *RestAPIConfig {
	return &RestAPIConfig{
		APIVersion: "v1",
	}
}

// RoomsFetcher defines interface that fetch gitter rooms.
type RoomsFetcher interface {
	Rooms(context.Context) (*Rooms, error)
}

// RestAPIClient utilizes gitter REST API.
type RestAPIClient struct {
	config     *RestAPIConfig
	httpClient *http.Client
}

// APIClientOption defines function signature that RestAPIClient's functional option must satisfy.
type APIClientOption func(*RestAPIClient)

// WithAPIClient replaces the http.DefaultClient with customized one.
func APIClientWithHTTPClient(httpClient *http.Client) APIClientOption {
	return func(c *RestAPIClient) {
		c.httpClient = httpClient
	}
}

// NewRestAPIClient creates and returns API client instance.
func NewRestAPIClient(config *RestAPIConfig, options ...APIClientOption) *RestAPIClient {
	client := &RestAPIClient{
		config:     config,
		httpClient: http.DefaultClient,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

func (client *RestAPIClient) buildEndpoint(resourceFragments []string) *url.URL {
	endpoint, _ := url.Parse(RestAPIEndpoint)
	fragments := append([]string{endpoint.Path, client.config.APIVersion}, resourceFragments...)
	endpoint.Path = path.Join(fragments...)
	return endpoint
}

// Get sends GET request with given path and parameters.
func (client *RestAPIClient) Get(ctx context.Context, resourceFragments []string, intf interface{}) error {
	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return xerrors.Errorf("failed to construct request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.config.Token))
	req.Header.Set("Accept", "application/json")

	req = req.WithContext(ctx)

	// Do request
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return xerrors.Errorf("failed reading response: %w", err)
	}
	if err := json.Unmarshal(body, &intf); err != nil {
		return xerrors.Errorf("failed to deserialize returned JSON structure %s: %w", string(body), err)
	}

	// Done
	return nil
}

// Post sends POST requests to gitter with given parameters.
func (client *RestAPIClient) Post(ctx context.Context, resourceFragments []string, sendingPayload interface{}, responsePayload interface{}) error {
	reqBody, err := json.Marshal(sendingPayload)
	if err != nil {
		return xerrors.Errorf("failed serializing given payload %+v: %w", err)
	}

	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("POST", endpoint.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		return xerrors.Errorf("failed to create new request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.config.Token))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// TODO check status code

	// Handle response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return xerrors.Errorf("failed reading response: %w", err)
	}
	if err := json.Unmarshal(body, &responsePayload); err != nil {
		return xerrors.Errorf("failed to deserialize returned JSON structure %s: %w", string(body), err)
	}

	// Done
	return nil
}

// Rooms fetches belonging rooms information.
func (client *RestAPIClient) Rooms(ctx context.Context) (*Rooms, error) {
	rooms := &Rooms{}
	if err := client.Get(ctx, []string{"rooms"}, &rooms); err != nil {
		return nil, xerrors.Errorf("failed to get list of belonging rooms: %w", err)
	}
	return rooms, nil
}

// PostMessage sends message to gitter.
func (client *RestAPIClient) PostMessage(ctx context.Context, room *Room, text string) (*Message, error) {
	message := &Message{}
	err := client.Post(ctx, []string{"rooms", room.ID, "chatMessages"}, &PostingMessage{Text: text}, message)
	if err != nil {
		return nil, xerrors.Errorf("failed to post message: %w", err)
	}
	return message, nil
}

// PostingMessage represents the sending message.
// This can be marshaled and sent as JSON-styled payload.
type PostingMessage struct {
	Text string `json:"text"`
}
