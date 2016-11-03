package gitter

import (
	"encoding/json"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	// GITTER_REST_API_ENDPOINT defines base url of gitter REST API.
	GITTER_REST_API_ENDPOINT = "https://api.gitter.im/"
)

type RoomsFetcher interface {
	Rooms(context.Context) (*Rooms, error)
}

// RestAPIClient utilizes gitter REST API.
type RestAPIClient struct {
	token      string
	apiVersion string
}

// NewVersionSpecificRestAPIClient creates API client instance with given API version.
func NewVersionSpecificRestAPIClient(token string, apiVersion string) *RestAPIClient {
	return &RestAPIClient{
		token:      token,
		apiVersion: apiVersion,
	}
}

// NewRestAPIClient creates and returns API client instance. Version is fixed to v1.
func NewRestAPIClient(token string) *RestAPIClient {
	return NewVersionSpecificRestAPIClient(token, "v1")
}

func (client *RestAPIClient) buildEndpoint(resourceFragments []string) *url.URL {
	endpoint, _ := url.Parse(GITTER_REST_API_ENDPOINT)
	fragments := append([]string{endpoint.Path, client.apiVersion}, resourceFragments...)
	endpoint.Path = path.Join(fragments...)
	return endpoint
}

// Get sends GET request with given path and parameters.
func (client *RestAPIClient) Get(ctx context.Context, resourceFragments []string, intf interface{}) error {
	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")

	// Do request
	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Handle response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &intf); err != nil {
		log.Errorf("can not unmarshal given JSON structure: %s", string(body))
		return err
	}

	// Done
	return nil
}

// Post sends POST requests to gitter with given paramters.
func (client *RestAPIClient) Post(ctx context.Context, resourceFragments []string, sendingPayload interface{}, responsePayload interface{}) error {
	reqBody, err := json.Marshal(sendingPayload)

	// Set up sending request
	endpoint := client.buildEndpoint(resourceFragments)
	req, err := http.NewRequest("POST", endpoint.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if err != nil {
		return err
	}

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// TODO check status code

	// Handle response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &responsePayload); err != nil {
		log.Errorf("can not unmarshal given JSON structure: %s", string(body))
		return err
	}

	// Done
	return nil
}

// Rooms fetches belonging rooms information.
func (client *RestAPIClient) Rooms(ctx context.Context) (*Rooms, error) {
	rooms := &Rooms{}
	if err := client.Get(ctx, []string{"rooms"}, &rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

// PostMessage sends message to gitter.
func (client *RestAPIClient) PostMessage(ctx context.Context, room *Room, text string) (*Message, error) {
	message := &Message{}
	if err := client.Post(ctx, []string{"rooms", room.ID, "chatMessages"}, &PostingMessage{Text: text}, message); err != nil {
		log.Error(err)
		return nil, err
	}
	return message, nil
}

// PostingMessage represents the sending message.
// This can be marshaled and sent as JSON-styled payload.
type PostingMessage struct {
	Text string `json:"text"`
}
