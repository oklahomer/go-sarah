/*
Package main provides a simple bot experience using slack.Adapter with multiple plugin commands and scheduled tasks.
*/
package main

import (
	"context"
	"flag"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/alerter/line"
	"github.com/oklahomer/go-sarah/v4/slack"
	"github.com/oklahomer/go-sarah/v4/watchers"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	_ "simple/plugins/count"
	"simple/plugins/echo"
	_ "simple/plugins/fixedtimer"
	_ "simple/plugins/guess"
	_ "simple/plugins/hello"
	_ "simple/plugins/morning"
	_ "simple/plugins/timer"
	"simple/plugins/todo"
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
	config := readConfig(*path)

	// When Bot encounters critical states, send alert to LINE.
	// Any number of Alerter implementation can be registered.
	sarah.RegisterAlerter(line.New(config.LineAlerter))

	// Setup storage that can be shared among different Bot implementation.
	storage := sarah.NewUserContextStorage(config.CacheConfig)

	// Setup Slack Bot.
	setupSlack(config.Slack, storage)

	// Setup some commands.
	todoCmd := todo.BuildCommand(&todo.DummyStorage{})
	sarah.RegisterCommand(slack.SLACK, todoCmd)

	// Directly add Command to Bot.
	// This Command is not subject to config file supervision.
	sarah.RegisterCommand(slack.SLACK, echo.Command)

	// Prepare go-sarah's core context
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare watcher that reads configuration from filesystem
	if config.PluginConfigDir != "" {
		configWatcher, _ := watchers.NewFileWatcher(ctx, config.PluginConfigDir)
		sarah.RegisterConfigWatcher(configWatcher)
	}

	// Run
	err := sarah.Run(ctx, config.Runner)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	select {
	case <-c:
		logger.Info("Stopping due to signal reception.")
		cancel()

	}
}

func readConfig(path string) *myConfig {
	configBody, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	config := newMyConfig()
	err = yaml.Unmarshal(configBody, config)
	if err != nil {
		panic(err)
	}

	return config
}

func setupSlack(config *slack.Config, storage sarah.UserContextStorage) {
	//adapter, err := slack.NewAdapter(config, slack.WithEventsPayloadHandler(slack.DefaultEventsPayloadHandler))
	adapter, err := slack.NewAdapter(config, slack.WithRTMPayloadHandler(slack.DefaultRTMPayloadHandler))
	if err != nil {
		panic(err)
	}

	bot := sarah.NewBot(adapter, sarah.BotWithStorage(storage))

	// Register bot to run.
	sarah.RegisterBot(bot)
}
