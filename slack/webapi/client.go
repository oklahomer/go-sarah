package webapi

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const (
	slackAPIEndpointFormat = "https://slack.com/api/%s"
)

type Config struct {
	Token string
}

type RtmStarter interface {
	RtmStart(context.Context) (*RtmStart, error)
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

func (client *Client) Get(ctx context.Context, slackMethod string, queryParams *url.Values, intf interface{}) error {
	endpoint := client.buildEndpoint(slackMethod, queryParams)

	resp, err := ctxhttp.Get(ctx, http.DefaultClient, endpoint.String())
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
		return statusErr(resp)
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

func statusErr(resp *http.Response) error {
	reqDump, reqErr := httputil.DumpRequestOut(resp.Request, true)
	if reqErr != nil {
		reqDump = []byte("N/A")
	}

	resDump, resErr := httputil.DumpResponse(resp, true)
	if resErr != nil {
		resDump = []byte("N/A")
	}

	return fmt.Errorf("response status error. Status: %d.\nRequest: %s\nResponse: %s", resp.StatusCode, string(reqDump), string(resDump))
}

func (client *Client) RtmStart(ctx context.Context) (*RtmStart, error) {
	rtmStart := &RtmStart{}
	if err := client.Get(ctx, "rtm.start", nil, &rtmStart); err != nil {
		return nil, err
	}

	if rtmStart.OK != true {
		return nil, fmt.Errorf("Error on rtm.start : %s", rtmStart.Error)
	}

	return rtmStart, nil
}

func (client *Client) Post(ctx context.Context, slackMethod string, bodyParam url.Values, intf interface{}) error {
	endpoint := client.buildEndpoint(slackMethod, nil)

	resp, err := ctxhttp.PostForm(ctx, http.DefaultClient, endpoint.String(), bodyParam)
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
		return statusErr(resp)
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

func (client *Client) PostMessage(ctx context.Context, postMessage *PostMessage) (*APIResponse, error) {
	response := &APIResponse{}
	err := client.Post(ctx, "chat.postMessage", postMessage.ToURLValues(), &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}
