package sarah

import (
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/worker"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
)

/*
Runner is the core of sarah.

This takes care of lifecycle of each bot implementation, internal job worker, and plugin execution;
Adapter is responsible for bot-specific implementation such as connection handling, message reception and sending.

Developers can register desired number of Adapter and Commands to create own bot.
*/
type Runner struct {
	bots   []Bot
	worker *worker.Worker
	cron   *cron.Cron
}

// NewRunner creates and return new Bot instance.
func NewRunner() *Runner {
	return &Runner{
		bots:   []Bot{},
		worker: worker.New(),
		cron:   cron.New(),
	}
}

func (runner *Runner) AppendBot(bot Bot) {
	runner.bots = append(runner.bots, bot)
}

/*
AddAdapter allows developer to register desired Adapter implementation.
Bot and each adapter mainly communicate via designated channels to pass incoming and outgoing responses.
*/
func (runner *Runner) AddAdapter(adapter Adapter, pluginConfigDir string) {
	for _, bot := range runner.bots {
		if bot.BotType() == adapter.BotType() {
			panic(fmt.Sprintf("BotType (%s) conflicted with stored Adapter.", adapter.BotType()))
		}
	}

	bot := newBot(adapter, pluginConfigDir)
	runner.AppendBot(bot)
}

/*
Run starts Bot interaction.

At this point Runner starts its internal workers, runs each bot, and starts listening to incoming messages.
*/
func (runner *Runner) Run(ctx context.Context) {
	runner.worker.Run(ctx.Done(), 10)

	for _, bot := range runner.bots {
		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// each Bot has its own context propagating Runner's lifecycle
		adapterCtx, cancelAdapter := context.WithCancel(ctx)

		// build commands with stashed builder settings
		commands := stashedCommandBuilders.build(botType, bot.PluginConfigDir())
		for _, command := range commands {
			bot.AppendCommand(command)
		}

		// build scheduled task with stashed builder settings
		tasks := stashedScheduledTaskBuilders.build(botType, bot.PluginConfigDir())
		for _, task := range tasks {
			runner.cron.AddFunc(task.config.Schedule(), func() {
				res, err := task.Execute(adapterCtx)
				if err != nil {
					log.Error(fmt.Sprintf("error on scheduled task: %s", task.Identifier))
					return
				} else if res == nil {
					return
				}

				message := NewOutputMessage(task.config.Destination(), res.Content)
				bot.SendMessage(adapterCtx, message)
			})
		}

		// run Adapter
		inputReceiver := make(chan Input)
		errCh := make(chan error)
		go runner.respond(adapterCtx, bot, inputReceiver)
		go stopUnrecoverableAdapter(errCh, cancelAdapter)
		go bot.Run(adapterCtx, inputReceiver, errCh)
	}

	runner.cron.Start()
}

/*
stopUnrecoverableAdapter receives error from Adapter, check if the error is critical, and stop the adapter if required.
*/
func stopUnrecoverableAdapter(errNotifier <-chan error, stopAdapter context.CancelFunc) {
	for {
		err := <-errNotifier
		switch err := err.(type) {
		case *AdapterNonContinuableError:
			log.Errorf("stop unrecoverable adapter: %s", err.Error())
			stopAdapter()
			return
		}
	}
}

/*
respond listens to incoming messages via channel.

Each Adapter enqueues incoming messages to runner's listening channel, and respond() receives them.
When corresponding command is found, command is executed and the result can be passed to Bot's SendMessage method.
*/
func (runner *Runner) respond(adapterCtx context.Context, bot Bot, inputReceiver <-chan Input) {
	for {
		select {
		case <-adapterCtx.Done():
			log.Info("stop responding to message due to context cancel")
			return
		case input := <-inputReceiver:
			log.Debugf("responding to %#v", input)

			runner.EnqueueJob(func() {
				res, err := bot.Respond(adapterCtx, input)
				if err != nil {
					log.Errorf("error on message handling. input: %#v. error: %s.", input, err.Error())
					return
				} else if res == nil {
					return
				}

				message := NewOutputMessage(input.ReplyTo(), res.Content)
				bot.SendMessage(adapterCtx, message)
			})
		}
	}
}

// EnqueueJob can be used to enqueue task to Runner's internal workers.
func (runner *Runner) EnqueueJob(job func()) {
	runner.worker.EnqueueJob(job)
}
