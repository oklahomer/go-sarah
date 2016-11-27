package sarah

import (
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/worker"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
)

type Config struct {
	Worker      *worker.Config
	CacheConfig *CacheConfig
}

func NewConfig() *Config {
	return &Config{
		Worker:      worker.NewConfig(),
		CacheConfig: NewCacheConfig(),
	}
}

// Runner is the core of sarah.
//
// This takes care of lifecycle of each Bot implementation and plugin execution;
// Bot is responsible for bot-specific implementation such as connection handling, message reception and sending.
//
// Developers can register desired number of Bot and Commands to create own bot experience.
type Runner struct {
	config *Config
	bots   []Bot
	cron   *cron.Cron
}

// NewRunner creates and return new Runner instance.
func NewRunner(config *Config) *Runner {
	return &Runner{
		config: config,
		bots:   []Bot{},
		cron:   cron.New(),
	}
}

// RegisterBot register given Bot implementation's instance to runner instance
func (runner *Runner) RegisterBot(bot Bot) {
	runner.bots = append(runner.bots, bot)
}

// RegisterAdapter allows developer to register desired Adapter implementation.
// This internally creates an instance of default Bot implementation with given Adapter.
// Created Bot instance is fed to Runner.RegisterBot.
//
//  runner := sarah.NewRunner(sarah.NewConfig())
//  runner.RegisterAdapter(slack.NewAdapter(slack.NewConfig(token)), "/path/to/plugin/config.yml")
//  runner.Run()
func (runner *Runner) RegisterAdapter(adapter Adapter, pluginConfigDir string) {
	for _, bot := range runner.bots {
		if bot.BotType() == adapter.BotType() {
			panic(fmt.Sprintf("BotType (%s) conflicted with stored Adapter.", adapter.BotType()))
		}
	}

	bot := newBot(adapter, runner.config.CacheConfig, pluginConfigDir)
	runner.RegisterBot(bot)
}

// Run starts Bot interaction.
// At this point Runner starts its internal workers, runs each bot, and starts listening to incoming messages.
func (runner *Runner) Run(ctx context.Context) {
	workerJob := worker.Run(ctx, runner.config.Worker)

	for _, bot := range runner.bots {
		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// each Bot has its own context propagating Runner's lifecycle
		botCtx, cancelBot := context.WithCancel(ctx)

		// build commands with stashed builder settings
		commands := stashedCommandBuilders.build(botType, bot.PluginConfigDir())
		for _, command := range commands {
			bot.AppendCommand(command)
		}

		// build scheduled task with stashed builder settings
		tasks := stashedScheduledTaskBuilders.build(botType, bot.PluginConfigDir())
		for _, task := range tasks {
			runner.cron.AddFunc(task.config.Schedule(), func() {
				res, err := task.Execute(botCtx)
				if err != nil {
					log.Errorf("error on scheduled task: %s", task.Identifier())
					return
				} else if res == nil {
					return
				}

				message := NewOutputMessage(task.config.Destination(), res.Content)
				bot.SendMessage(botCtx, message)
			})
		}

		// run Bot
		inputReceiver := make(chan Input)
		errCh := make(chan error)
		go respond(botCtx, bot, inputReceiver, workerJob)
		go stopUnrecoverableBot(errCh, cancelBot)
		go bot.Run(botCtx, inputReceiver, errCh)
	}

	runner.cron.Start()
}

// stopUnrecoverableBot receives error from Bot, check if the error is critical, and stop the bot if required.
func stopUnrecoverableBot(errNotifier <-chan error, stopBot context.CancelFunc) {
	for {
		err := <-errNotifier
		switch err := err.(type) {
		case *BotNonContinuableError:
			log.Errorf("stop unrecoverable bot: %s", err.Error())
			stopBot()
			return
		}
	}
}

// respond listens to incoming messages via channel.
//
// Each Bot enqueues incoming messages to runner's listening channel, and respond() receives them.
// When corresponding command is found, command is executed and the result can be passed to Bot's SendMessage method.
func respond(botCtx context.Context, bot Bot, inputReceiver <-chan Input, workerJob chan<- func()) {
	for {
		select {
		case <-botCtx.Done():
			log.Info("stop responding to message due to context cancel")
			return
		case input := <-inputReceiver:
			log.Debugf("responding to %#v", input)

			workerJob <- func() {
				err := bot.Respond(botCtx, input)
				if err != nil {
					log.Errorf("error on message handling. input: %#v. error: %s.", input, err.Error())
					return
				}
			}
		}
	}
}
