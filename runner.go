package sarah

import (
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/worker"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
)

var (
	stashedCommandBuilder       = map[BotType][]*commandBuilder{}
	stashedScheduledTaskBuilder = map[BotType][]*scheduledTaskBuilder{}
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
		if builders, ok := stashedCommandBuilder[botType]; ok {
			commands := buildCommands(builders, bot.PluginConfigDir())
			for _, command := range commands {
				bot.AppendCommand(command)
			}
		}

		// build scheduled task with stashed builder settings
		if builders, ok := stashedScheduledTaskBuilder[botType]; ok {
			tasks := buildScheduledTasks(builders, bot.PluginConfigDir())
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
		case botInput := <-inputReceiver:
			log.Debugf("responding to %#v", botInput)

			runner.EnqueueJob(func() {
				res, err := bot.Respond(adapterCtx, botInput)
				if err != nil {
					log.Errorf("error on message handling. botInput: %s. error: %#v.", botInput, err.Error())
					return
				} else if res == nil {
					return
				}

				message := NewOutputMessage(botInput.ReplyTo(), res.Content)
				bot.SendMessage(adapterCtx, message)
			})
		}
	}
}

// EnqueueJob can be used to enqueue task to Runner's internal workers.
func (runner *Runner) EnqueueJob(job func()) {
	runner.worker.EnqueueJob(job)
}

/*
AppendCommandBuilder appends given commandBuilder to internal stash.
Stashed builder is used to configure and build Command instance on Runner's initialization.
*/
func AppendCommandBuilder(botType BotType, builder *commandBuilder) {
	log.Infof("appending command builder for %s. builder %#v.", botType, builder)
	_, ok := stashedCommandBuilder[botType]
	if !ok {
		stashedCommandBuilder[botType] = make([]*commandBuilder, 0)
	}

	stashedCommandBuilder[botType] = append(stashedCommandBuilder[botType], builder)
}

func AppendScheduledTaskBuilder(botType BotType, builder *scheduledTaskBuilder) {
	log.Infof("appending scheduled task builder for %s. builder %#v.", botType, builder)
	_, ok := stashedScheduledTaskBuilder[botType]
	if !ok {
		stashedScheduledTaskBuilder[botType] = make([]*scheduledTaskBuilder, 0)
	}

	stashedScheduledTaskBuilder[botType] = append(stashedScheduledTaskBuilder[botType], builder)
}

/*
buildCommands configures and creates Command instances with given stashed CommandBuilders
*/
func buildCommands(builders []*commandBuilder, configDir string) []Command {
	commands := []Command{}
	for _, builder := range builders {
		command, err := builder.build(configDir)
		if err != nil {
			log.Errorf(fmt.Sprintf("can't configure plugin: %s. error: %s.", builder.identifier, err.Error()))
			continue
		}
		commands = append(commands, command)
	}

	return commands
}

func buildScheduledTasks(builders []*scheduledTaskBuilder, configDir string) []*scheduledTask {
	scheduledTasks := []*scheduledTask{}
	for _, builder := range builders {
		task, err := builder.build(configDir)
		if err != nil {
			log.Errorf(fmt.Sprintf("can't configure plugin: %s. error: %s.", builder.identifier, err.Error()))
			continue
		}
		scheduledTasks = append(scheduledTasks, task)
	}

	return scheduledTasks
}
