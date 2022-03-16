package worldweather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	weatherAPIEndpointFormat = "https://api.worldweatheronline.com/free/v2/%s.ashx"
)

// Config contains some configuration variables.
type Config struct {
	apiKey string
}

// NewConfig returns a newly initialized Config with the given API key.
func NewConfig(apiKey string) *Config {
	return &Config{
		apiKey: apiKey,
	}
}

// Client is an API client for World Weather.
type Client struct {
	config *Config
}

// NewClient creates and returns a new API client with the given Config struct.
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
		panic(fmt.Errorf("failed to parse construct URL for %s: %w", apiType, err))
	}
	requestURL.RawQuery = queryParams.Encode()

	return requestURL
}

// Get makes an HTTP GET request to the World Weather API endpoint.
func (client *Client) Get(ctx context.Context, apiType string, queryParams *url.Values, data interface{}) error {
	endpoint := client.buildEndpoint(apiType, queryParams)
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed on GET request for %s: %w", apiType, err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response status %d is returned", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, data); err != nil {
		return fmt.Errorf("failed to parse returned json data: %w", err)
	}

	return nil
}

// LocalWeather fetches the given location's weather.
func (client *Client) LocalWeather(ctx context.Context, location string) (*LocalWeatherResponse, error) {
	queryParams := &url.Values{}
	queryParams.Add("q", location)
	data := &LocalWeatherResponse{}
	if err := client.Get(ctx, "weather", queryParams, data); err != nil {
		return nil, fmt.Errorf("failed getting weather data: %w", err)
	}

	return data, nil
}
