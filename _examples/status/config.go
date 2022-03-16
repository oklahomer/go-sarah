package main

import (
	"fmt"
	"github.com/oklahomer/go-kasumi/worker"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"gopkg.in/yaml.v2"
	"os"
)

func readConfig(path string) (*config, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Populate with default configuration value by calling each constructor.
	c := &config{
		Runner:       sarah.NewConfig(),
		Slack:        slack.NewConfig(),
		ContextCache: sarah.NewCacheConfig(),
		Worker:       worker.NewConfig(),
	}
	err = yaml.Unmarshal(body, c)
	if err != nil {
		return nil, fmt.Errorf("failed to read yaml: %w", err)
	}

	return c, nil
}

type config struct {
	Runner       *sarah.Config      `json:"runner" yaml:"runner"`
	Slack        *slack.Config      `json:"slack" yaml:"slack"`
	ContextCache *sarah.CacheConfig `json:"context_cache" yaml:"context_cache"`
	Worker       *worker.Config     `json:"worker" yaml:"worker"`
}
