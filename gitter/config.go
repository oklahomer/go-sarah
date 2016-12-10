package gitter

import "time"

type Config struct {
	Token         string
	RetryLimit    uint
	RetryInterval time.Duration
}

func NewConfig(token string) *Config {
	// minimal configuration
	return &Config{
		Token:         token,
		RetryLimit:    10,
		RetryInterval: 500 * time.Millisecond,
	}
}
