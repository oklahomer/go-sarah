/*
Package main provides a simple bot experience using slack.Adapter with multiple plugin commands and scheduled tasks.
*/
package main

import (
	"context"
	"flag"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/alerter/line"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/count"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/echo"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/fixedtimer"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/guess"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/hello"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/morning"
	_ "github.com/oklahomer/go-sarah/examples/simple/plugins/timer"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/todo"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/go-sarah/watchers"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
)

type myConfig struct {
	CacheConfig     *sarah.CacheConfig `yaml:"cache"`
	Slack           *slack.Config      `yaml:"slack"`
	Runner          *sarah.Config      `yaml:"runner"`
	LineAlerter     *line.Config       `yaml:"line_alerter"`
	PluginConfigDir string             `yaml:"plugin_config_dir"`
}

func newMyConfig() *myConfig {
	// Use constructor for each config struct, so default values are pre-set.
	return &myConfig{
		CacheConfig: sarah.NewCacheConfig(),
		Slack:       slack.NewConfig(),
		Runner:      sarah.NewConfig(),
		LineAlerter: line.NewConfig(),
	}
}

func main() {
	var path = flag.String("config", "", "path to application configuration file.")
	flag.Parse()
	if *path == "" {
		panic("./bin/examples -config=/path/to/config/app.yml")
	}

	// Read configuration file.
	config, err := readConfig(*path)
	if err != nil {
		panic(err)
	}

	// When Bot encounters critical states, send alert to LINE.
	// Any number of Alerter implementation can be registered.
	sarah.RegisterAlerter(line.New(config.LineAlerter))

	// Setup storage that can be shared among different Bot implementation.
	storage := sarah.NewUserContextStorage(config.CacheConfig)

	// Setup Slack Bot.
	slackBot, err := setupSlack(config.Slack, storage)
	if err != nil {
		panic(err)
	}

	// Setup some commands.
	todoCmd := todo.BuildCommand(&todo.DummyStorage{})
	slackBot.AppendCommand(todoCmd)

	// Register bot to run.
	sarah.RegisterBot(slackBot)

	// Directly add Command to Bot.
	// This Command is not subject to config file supervision.
	slackBot.AppendCommand(echo.Command)

	// Prepare go-sarah's core context
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare watcher that reads configuration from filesystem
	if config.PluginConfigDir != "" {
		configWatcher, _ := watchers.NewFileWatcher(ctx, config.PluginConfigDir)
		sarah.RegisterConfigWatcher(configWatcher)
	}

	// Run
	err = sarah.Run(ctx, config.Runner)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	select {
	case <-c:
		log.Info("Stopping due to signal reception.")
		cancel()

	}
}

func readConfig(path string) (*myConfig, error) {
	configBody, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := newMyConfig()
	err = yaml.Unmarshal(configBody, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func setupSlack(config *slack.Config, storage sarah.UserContextStorage) (sarah.Bot, error) {
	adapter, err := slack.NewAdapter(config)
	if err != nil {
		return nil, err
	}

	return sarah.NewBot(adapter, sarah.BotWithStorage(storage))
}
