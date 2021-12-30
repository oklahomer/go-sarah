package gitter

import (
	"github.com/oklahomer/go-kasumi/retry"
	"time"
)

// Config contains some configuration variables for Gitter Adapter.
type Config struct {
	// Token declares the API token to integrate with Gitter.
	Token string `json:"token" yaml:"token"`

	// RetryPolicy declares how a retrial for an API call should behave.
	RetryPolicy *retry.Policy `json:"retry_policy" yaml:"retry_policy"`
}

// NewConfig creates and returns a new Config instance with default settings.
// Token is empty at this point as there can not be a default value.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to populate the blank value or override those default values.
func NewConfig() *Config {
	return &Config{
		Token: "",
		RetryPolicy: &retry.Policy{
			Trial:    10,
			Interval: 500 * time.Millisecond,
		},
	}
}
