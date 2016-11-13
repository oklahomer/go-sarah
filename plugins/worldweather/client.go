package worldweather

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	weatherAPIEndpointFormat = "https://api.worldweatheronline.com/free/v2/%s.ashx"
)

type Config struct {
	apiKey string
}

func NewConfig(apiKey string) *Config {
	return &Config{
		apiKey: apiKey,
	}
}

type Client struct {
	config *Config
}

func NewClient(config *Config) *Client {
	return &Client{config: config}
}

func (client *Client) buildEndpoint(apiType string, queryParams *url.Values) *url.URL {
	if queryParams == nil {
		queryParams = &url.Values{}
	}
	queryParams.Add("key", client.config.apiKey)
	queryParams.Add("format", "json")

	requestURL, err := url.Parse(fmt.Sprintf(weatherAPIEndpointFormat, apiType))
	if err != nil {
		panic(err.Error())
	}
	requestURL.RawQuery = queryParams.Encode()

	return requestURL
}

func (client *Client) Get(ctx context.Context, apiType string, queryParams *url.Values, intf interface{}) error {
	endpoint := client.buildEndpoint(apiType, queryParams)
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
		return fmt.Errorf("response status error. status: %d", resp.StatusCode)
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

func (client *Client) LocalWeather(ctx context.Context, location string) (*LocalWeatherResponse, error) {
	queryParams := &url.Values{}
	queryParams.Add("q", location)
	data := &LocalWeatherResponse{}
	if err := client.Get(ctx, "weather", queryParams, &data); err != nil {
		return nil, err
	}

	return data, nil
}
