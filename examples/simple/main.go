/*
Package main provides a simple bot experience using slack.Adapter with multiple plugin commands and scheduled tasks.
*/
package main

import (
	"context"
	"flag"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/alerter/line"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/count"
	"github.com/oklahomer/go-sarah/v3/examples/simple/plugins/echo"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/fixedtimer"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/guess"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/hello"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/morning"
	_ "github.com/oklahomer/go-sarah/v3/examples/simple/plugins/timer"
	"github.com/oklahomer/go-sarah/v3/examples/simple/plugins/todo"
	"github.com/oklahomer/go-sarah/v3/log"
	"github.com/oklahomer/go-sarah/v3/slack"
	"github.com/oklahomer/go-sarah/v3/watchers"
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
		log.Info("Stopping due to signal reception.")
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

	bot, err := sarah.NewBot(adapter, sarah.BotWithStorage(storage))
	if err != nil {
		panic(err)
	}

	// Register bot to run.
	sarah.RegisterBot(bot)
}
