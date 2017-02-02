package line

import (
	"fmt"
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var Endpoint = "https://notify-api.line.me/api/notify"

type Config struct {
	Token          string        `json:"token" yaml:"token"`
	RequestTimeout time.Duration `json:"timeout" yaml:"timeout"`
}

func NewConfig() *Config {
	return &Config{
		Token:          "", // Updated on json/yaml unmarshal or by manually
		RequestTimeout: 3 * time.Second,
	}
}

type Client struct {
	config *Config
}

func New(config *Config) *Client {
	return &Client{
		config: config,
	}
}

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
		return fmt.Errorf("response status error. Status: %d", resp.StatusCode)
	}

	return nil
}
