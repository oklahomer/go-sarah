package slack

import "time"

type Config struct {
	Token            string
	SendingQueueSize uint
	RetryLimit       uint
	RetryInterval    time.Duration
	PingInterval     time.Duration
}

func NewConfig(token string) *Config {
	// minimal configuration
	return &Config{
		Token:            token,
		SendingQueueSize: 100,
		RetryLimit:       10,
		RetryInterval:    500 * time.Millisecond,
		PingInterval:     30 * time.Second,
	}
}
