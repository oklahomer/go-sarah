/*
Package line provides sarah.Alerter implementation for LINE Notify.
*/
package line

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah/v2"
	"golang.org/x/xerrors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endpoint defines the API endpoint to be used for notification.
var Endpoint = "https://notify-api.line.me/api/notify"

// Config contains some configuration variables for gitter Adapter.
type Config struct {
	Token          string        `json:"token" yaml:"token"`
	RequestTimeout time.Duration `json:"timeout" yaml:"timeout"`
}

// NewConfig returns initialized Config struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal,
// or direct assignment.
func NewConfig() *Config {
	return &Config{
		Token:          "", // Updated on json/yaml unmarshal or by manually
		RequestTimeout: 3 * time.Second,
	}
}

// Option defines a function signature that New()'s functional options must satisfy.
type Option func(*Client)

// WithHTTPClient creates an Option that replaces http.DefaultClient with preferred one.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// Client is an API client for LINE notification.
type Client struct {
	config     *Config
	httpClient *http.Client
}

// New creates and returns new Client instant.
func New(config *Config, options ...Option) *Client {
	c := &Client{
		config:     config,
		httpClient: http.DefaultClient,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// Alert sends alert message to notify critical state of caller.
func (c *Client) Alert(ctx context.Context, botType sarah.BotType, err error) error {
	msg := fmt.Sprintf("Error on %s: %s.", botType.String(), err.Error())
	v := url.Values{"message": {msg}}
	req, err := http.NewRequest(http.MethodPost, Endpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	reqCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return xerrors.Errorf("response status %d is returned", resp.StatusCode)
	}

	return nil
}
