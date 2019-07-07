package main

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/go-sarah/workers"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func readConfig(path string) (*config, error) {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file: %w", err)
	}

	// Populate with default configuration value by calling each constructor.
	c := &config{
		Runner:       sarah.NewConfig(),
		Slack:        slack.NewConfig(),
		ContextCache: sarah.NewCacheConfig(),
		Worker:       workers.NewConfig(),
	}
	err = yaml.Unmarshal(body, c)
	if err != nil {
		return nil, xerrors.Errorf("failed to read yaml: %w", err)
	}

	return c, nil
}

type config struct {
	Runner       *sarah.Config      `json:"runner" yaml:"runner"`
	Slack        *slack.Config      `json:"slack" yaml:"slack"`
	ContextCache *sarah.CacheConfig `json:"context_cache" yaml:"context_cache"`
	Worker       *workers.Config    `json:"worker" yaml:"worker"`
}
