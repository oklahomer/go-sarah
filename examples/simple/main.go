/*
Package main provides a simple bot experience using slack.Adapter with multiple plugin commands and scheduled tasks.
*/
package main

import (
	"flag"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/alerter/line"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/count"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/echo"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/fixedtimer"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/guess"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/hello"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/morning"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/timer"
	"github.com/oklahomer/go-sarah/examples/simple/plugins/todo"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
)

type myConfig struct {
	CacheConfig *sarah.CacheConfig `yaml:"cache"`
	Slack       *slack.Config      `yaml:"slack"`
	Runner      *sarah.Config      `yaml:"runner"`
	LineAlerter *line.Config       `yaml:"line_alerter"`
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
	var path = flag.String("config", "", "apth to apllication configuration file.")
	flag.Parse()
	if *path == "" {
		panic("./bin/examples -config=/path/to/config/app.yml")
	}

	// Read configuration file.
	config, err := readConfig(*path)
	if err != nil {
		panic(err)
	}

	// A handy helper that holds arbitrary amount of RunnerOptions.
	runnerOptions := sarah.NewRunnerOptions()

	// When Bot encounters critical states, send alert to LINE.
	// Any number of Alerter implementation can be registered.
	alerter := line.New(config.LineAlerter)
	runnerOptions.Append(sarah.WithAlerter(alerter))

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
	runnerOptions.Append(sarah.WithBot(slackBot))

	// Setup some plugins to build on the fly.
	// Each configuration file, if exists, is subject to supervise.
	// If updated, Command is re-built with new configuration.
	runnerOptions.Append(sarah.WithCommandProps(hello.SlackProps))
	runnerOptions.Append(sarah.WithCommandProps(morning.SlackProps))
	runnerOptions.Append(sarah.WithCommandProps(count.SlackProps))
	runnerOptions.Append(sarah.WithCommandProps(guess.SlackProps))

	// Setup scheduled tasks.
	// Each configuration file, if exists, is subject to supervise.
	// If updated, Command is re-built with new configuration.
	runnerOptions.Append(sarah.WithScheduledTaskProps(timer.SlackProps))
	runnerOptions.Append(sarah.WithScheduledTaskProps(fixedtimer.SlackProps))

	// Directly add Command to Bot.
	// This Command is not subject to config file supervision.
	slackBot.AppendCommand(echo.Command)

	// Setup sarah.Runner.
	runner, err := sarah.NewRunner(config.Runner, runnerOptions.Arg())
	if err != nil {
		panic(err)
	}

	// Run sarah.Runner.
	run(runner)
}

func run(runner sarah.Runner) {
	ctx, cancel := context.WithCancel(context.Background())
	runnerStop := make(chan struct{})
	go func() {
		// Blocks til all belonging Bots stop, or context is canceled.
		runner.Run(ctx)
		runnerStop <- struct{}{}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	select {
	case <-c:
		log.Info("Stopping due to signal reception.")
		cancel()

	case <-runnerStop:
		log.Error("Runner stopped.")

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
