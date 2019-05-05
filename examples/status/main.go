/*
Package main provides an example that uses sarah.CurrentStatus() to get current go-sarah and its belonging Bot's status via HTTP server.

In this example two bots, slack and nullBot, are registered to go-sarah and become subject to supervise.
See handler.go for Runner.Status() usage.
*/
package main

import (
	"flag"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/go-sarah/workers"
	"golang.org/x/net/context"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Parse and check command line flags
	var path = flag.String("config", "", "path to application configuration file.")
	flag.Parse()
	if *path == "" {
		panic("./path/to/executable -config=/path/to/config/app.yml")
	}

	// Initialize config with given file content
	cfg, err := readConfig(*path)
	if err != nil {
		panic(err)
	}

	// Setup a bot
	nullBot := &nullBot{}
	sarah.RegisterBot(nullBot)

	// Setup another bot
	slackBot, err := setupSlackBot(cfg)
	if err != nil {
		panic(err)
	}
	sarah.RegisterBot(slackBot)

	// Setup worker
	workerReporter := &workerStats{}
	reporterOpt := workers.WithReporter(workerReporter)
	worker, err := workers.Run(ctx, cfg.Worker, reporterOpt)
	if err != nil {
		panic(err)
	}
	sarah.RegisterWorker(worker)

	// Setup a Runner to run and supervise above bots
	err = sarah.Run(ctx, cfg.Runner)
	if err != nil {
		panic(err)
	}

	// Run HTTP server that reports current status
	server := newServer(workerReporter)
	go server.Run(ctx)

	// Wait til signal reception
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	// Stop
	log.Info("Stopping due to signal reception.")
	cancel()
	time.Sleep(1 * time.Second) // Wait a bit til things finish
}

func setupSlackBot(cfg *config) (sarah.Bot, error) {
	storage := sarah.NewUserContextStorage(cfg.ContextCache)
	slackAdapter, err := slack.NewAdapter(cfg.Slack)
	if err != nil {
		return nil, err
	}
	slackBot, err := sarah.NewBot(slackAdapter, sarah.BotWithStorage(storage))
	if err != nil {
		return nil, err
	}
	return slackBot, nil
}
