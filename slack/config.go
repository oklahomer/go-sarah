package slack

import "time"

type Config struct {
	Token            string        `json:"token" yaml:"token"`
	SendingQueueSize uint          `json:"sending_queue_size" yaml:"sending_queue_size"`
	RetryLimit       uint          `json:"retry_limit" yaml:"retry_limit"`
	RetryInterval    time.Duration `json:"retry_interval" yaml:"retry_interval"`
	PingInterval     time.Duration `json:"ping_interval" yaml:"ping_interval"`
}

// NewConfig returns initialized Config struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal,
// or direct assignment.
func NewConfig() *Config {
	return &Config{
		Token:            "",
		SendingQueueSize: 100,
		RetryLimit:       10,
		RetryInterval:    500 * time.Millisecond,
		PingInterval:     30 * time.Second,
	}
}
