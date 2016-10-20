package rtmapi

import (
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// Client utilizes Slack REST API.
type Client struct {
}

// NewClient creates and returns new Client instance
func NewClient() *Client {
	return &Client{}
}

// Connect connects to Slack WebSocket server.
func (client *Client) Connect(_ context.Context, url string) (Connection, error) {
	conn, err := websocket.Dial(url, "", "http://localhost")
	if err != nil {
		return nil, err
	}

	return newConnectionWrapper(conn), nil
}
