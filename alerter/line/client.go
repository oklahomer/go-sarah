/*
Package line provides sarah.Alerter implementation for LINE Notify.
*/
package line

import (
	"fmt"
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
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

// Client is an API client for LINE notification.
type Client struct {
	config *Config
}

// New creates and returns new Client instant.
func New(config *Config) *Client {
	return &Client{
		config: config,
	}
}

// Alert sends alert message to notify critical state of caller.
func (c *Client) Alert(ctx context.Context, botType sarah.BotType, err error) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	msg := fmt.Sprintf("Critical error on %s: %s.", botType.String(), err.Error())
	v := url.Values{"message": {msg}}
	req, err := http.NewRequest("POST", Endpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ctxhttp.Do(reqCtx, http.DefaultClient, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return xerrors.Errorf("response status %d is returned", resp.StatusCode)
	}

	return nil
}
