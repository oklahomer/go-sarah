package gitter

import "time"

type Config struct {
	token         string
	retryLimit    uint
	retryInterval time.Duration
}

func NewConfig(token string) *Config {
	// minimal configuration
	return &Config{
		token:         token,
		retryLimit:    10,
		retryInterval: 500 * time.Millisecond,
	}
}
