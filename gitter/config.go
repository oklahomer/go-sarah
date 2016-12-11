package gitter

import "time"

type Config struct {
	Token         string        `json:"token" yaml:"token"`
	RetryLimit    uint          `json:"retry_limit" yaml:"retry_limit"`
	RetryInterval time.Duration `json:"retry_interval" yaml:"retry_interval"`
}

// NewConfig returns initialized Config struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal,
// or direct assignment.
func NewConfig() *Config {
	return &Config{
		Token:         "",
		RetryLimit:    10,
		RetryInterval: 500 * time.Millisecond,
	}
}
