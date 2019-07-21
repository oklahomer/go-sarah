package gitter

import (
	"github.com/oklahomer/go-sarah/v2/retry"
	"time"
)

// Config contains some configuration variables for gitter Adapter.
type Config struct {
	Token       string        `json:"token" yaml:"token"`
	RetryPolicy *retry.Policy `json:"retry_policy" yaml:"retry_policy"`
}

// NewConfig returns initialized Config struct with default settings.
// Token is empty at this point. Token can be set by feeding this instance to json.Unmarshal/yaml.Unmarshal,
// or direct assignment.
func NewConfig() *Config {
	return &Config{
		Token: "",
		RetryPolicy: &retry.Policy{
			Trial:    10,
			Interval: 500 * time.Millisecond,
		},
	}
}
