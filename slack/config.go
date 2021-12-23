package slack

import (
	"github.com/oklahomer/go-kasumi/retry"
	"time"
)

// Config contains some configuration variables for Slack Adapter.
type Config struct {
	// Token declares the API token to integrate with Gitter.
	Token string `json:"token" yaml:"token"`

	// AppSecret declares the application secret issued by Slack.
	AppSecret string `json:"app_secret" yaml:"app_secret"`

	// ListenPort declares the port number that receives requests from Slack.
	ListenPort int `json:"listen_port" yaml:"listen_port"`

	// HelpCommand declares the command string that is converted to sarah.HelpInput.
	HelpCommand string `json:"help_command" yaml:"help_command"`

	// AbortCommand declares the command string to abort the current user context.
	AbortCommand string `json:"abort_command" yaml:"abort_command"`

	// SendingQueueSize declares the capacity of the outgoing message queue.
	SendingQueueSize uint `json:"sending_queue_size" yaml:"sending_queue_size"`

	// RequestTimeout declares the timeout interval for the Slack API calls.
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`

	// PingInterval declares the ping interval for RTM API interaction.
	PingInterval time.Duration `json:"ping_interval" yaml:"ping_interval"`

	// RetryPolicy declares how a retrial for an API call should behave.
	RetryPolicy *retry.Policy `json:"retry_policy" yaml:"retry_policy"`
}

// NewConfig creates and returns a new Config instance with default settings.
// Token and AppSecret are empty at this point as there can not be default values.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to populate the blank value or override those default values.
func NewConfig() *Config {
	return &Config{
		Token:            "",
		AppSecret:        "",
		ListenPort:       8080,
		HelpCommand:      ".help",
		AbortCommand:     ".abort",
		SendingQueueSize: 100,
		RequestTimeout:   3 * time.Second,
		PingInterval:     30 * time.Second,
		RetryPolicy: &retry.Policy{
			Trial:    10,
			Interval: 500 * time.Millisecond,
		},
	}
}
