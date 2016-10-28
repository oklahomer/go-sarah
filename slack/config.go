package slack

import "time"

type Config struct {
	token            string
	sendingQueueSize uint
	retryLimit       uint
	retryInterval    time.Duration
	pingInterval     time.Duration
}

func NewConfig(token string) *Config {
	// minimal configuration
	return &Config{
		token:            token,
		sendingQueueSize: 100,
		retryLimit:       10,
		retryInterval:    500 * time.Millisecond,
		pingInterval:     30 * time.Second,
	}
}
