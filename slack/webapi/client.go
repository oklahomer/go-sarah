package webapi

import (
	"encoding/json"
	"fmt"
	"github.com/oklahomer/go-sarah/httperror"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	slackAPIEndpointFormat = "https://slack.com/api/%s"
)

type Config struct {
	Token string
}

type Client struct {
	config *Config
}

func NewClient(config *Config) *Client {
	return &Client{config: config}
}

func (client *Client) buildEndpoint(slackMethod string, queryParams *url.Values) *url.URL {
	if queryParams == nil {
		queryParams = &url.Values{}
	}
	queryParams.Add("token", client.config.Token)

	requestURL, err := url.Parse(fmt.Sprintf(slackAPIEndpointFormat, slackMethod))
	if err != nil {
		panic(err.Error())
	}
	requestURL.RawQuery = queryParams.Encode()

	return requestURL
}

func (client *Client) Get(slackMethod string, queryParams *url.Values, intf interface{}) error {
	endpoint := client.buildEndpoint(slackMethod, queryParams)

	resp, err := http.Get(endpoint.String())
	if err != nil {
		switch e := err.(type) {
		case *url.Error:
			return e
		default:
			// Comes here when request URL is nil, but that MUST NOT happen.
			panic(fmt.Sprintf("error on HTTP GET request. %#v", e))
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httperror.NewResponseError(fmt.Sprintf("response status error. status: %d.", resp.StatusCode), resp)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &intf); err != nil {
		return err
	}

	return nil
}

func (client *Client) RtmStart() (*RtmStart, error) {
	rtmStart := &RtmStart{}
	if err := client.Get("rtm.start", nil, &rtmStart); err != nil {
		return nil, err
	}

	return rtmStart, nil
}

func (client *Client) Post(slackMethod string, bodyParam url.Values, intf interface{}) error {
	endpoint := client.buildEndpoint(slackMethod, nil)

	resp, err := http.PostForm(endpoint.String(), bodyParam)
	if err != nil {
		switch e := err.(type) {
		case *url.Error:
			return e
		default:
			// Comes here when request URL is nil, but that MUST NOT happen.
			panic(fmt.Sprintf("error on HTTP GET request. %#v", e))
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httperror.NewResponseError(fmt.Sprintf("response status error. status: %d.", resp.StatusCode), resp)
	}

	response, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(response, &intf); err != nil {
		return err
	}

	return nil
}

func (client *Client) PostMessage(postMessage *PostMessage) (*APIResponse, error) {
	response := &APIResponse{}
	err := client.Post("chat.postMessage", postMessage.ToURLValues(), &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}
