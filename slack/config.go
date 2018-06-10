package slack

import (
	"github.com/oklahomer/go-sarah/retry"
	"time"
)

// Config contains some configuration variables for slack Adapter.
type Config struct {
	Token            string        `json:"token" yaml:"token"`
	HelpCommand      string        `json:"help_command" yaml:"help_command"`
	AbortCommand     string        `json:"abort_command" yaml:"abort_command"`
	SendingQueueSize uint          `json:"sending_queue_size" yaml:"sending_queue_size"`
	RequestTimeout   time.Duration `json:"request_timeout" yaml:"request_timeout"`
	PingInterval     time.Duration `json:"ping_interval" yaml:"ping_interval"`
	RetryPolicy      *retry.Policy `json:"retry_policy" yaml:"retry_policy"`
}

// NewConfig returns initialized Config struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal,
// or direct assignment.
func NewConfig() *Config {
	return &Config{
		Token:            "",
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
