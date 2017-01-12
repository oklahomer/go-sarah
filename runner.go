package sarah

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/worker"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Worker           *worker.Config `json:"worker" yaml:"worker"`
	PluginConfigRoot string         `json:"plugin_config_root" yaml:"plugin_config_root"`
	TimeZone         string         `json:"timezone" yaml:"timezone"`
}

func NewConfig() *Config {
	return &Config{
		Worker: worker.NewConfig(),
		// PluginConfigRoot defines the root directory to be used when searching for plugins' configuration files.
		PluginConfigRoot: "",
		TimeZone:         time.Now().Location().String(),
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
}

// NewRunner creates and return new Runner instance.
func NewRunner(config *Config) *Runner {
	return &Runner{
		config: config,
		bots:   []Bot{},
	}
}

// RegisterBot register given Bot implementation's instance to runner instance
func (runner *Runner) RegisterBot(bot Bot) {
	runner.bots = append(runner.bots, bot)
}

// Run starts Bot interaction.
// At this point Runner starts its internal workers and schedulers, runs each bot, and starts listening to incoming messages.
func (runner *Runner) Run(ctx context.Context) {
	loc, locErr := time.LoadLocation(runner.config.TimeZone)
	if locErr != nil {
		panic(fmt.Sprintf("Given timezone can't be converted to time.Location: %s.", locErr.Error()))
	}

	scheduler := cron.NewWithLocation(loc)
	go func() {
		<-ctx.Done()
		log.Info("stop cron jobs due to context cancel")
		scheduler.Stop()
	}()
	workerJob := worker.Run(ctx, runner.config.Worker)

	for _, bot := range runner.bots {
		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// each Bot has its own context propagating Runner's lifecycle
		botCtx, cancelBot := context.WithCancel(ctx)

		var configDir string
		if runner.config.PluginConfigRoot != "" {
			configDir = filepath.Join(runner.config.PluginConfigRoot, strings.ToLower(bot.BotType().String()))
		}

		// build commands with stashed builder settings
		commands := stashedCommandBuilders.build(botType, configDir)
		for _, command := range commands {
			bot.AppendCommand(command)
		}

		// build scheduled task with stashed builder settings
		tasks := stashedScheduledTaskBuilders.build(botType, configDir)
		for _, task := range tasks {
			setupScheduledTask(botCtx, bot, scheduler, task)
		}

		// supervise configuration files' directory
		if configDir != "" {
			go superviseCommandConfig(botCtx, bot, configDir)
		}

		// run Bot
		inputReceiver := make(chan Input)
		errCh := make(chan error)
		go respond(botCtx, bot, inputReceiver, workerJob)
		go stopUnrecoverableBot(errCh, cancelBot)
		go bot.Run(botCtx, inputReceiver, errCh)
	}

	scheduler.Start()
}

func superviseCommandConfig(botCtx context.Context, bot Bot, configDir string) {
	log.Infof("start watching file change under %s", configDir)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize fsnotifier: %s.", err.Error()))
	}
	watcher.Add(configDir)

	for {
		select {
		case <-botCtx.Done():
			watcher.Close()
			return
		case event := <-watcher.Events:
			switch {
			// When configuration file is created or updated, rebuild command.
			// Remember file may be absent on previous build because one can start with default config struct and later update.
			case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
				log.Infof("%s %s.", event.Op.String(), event.Name)
				dir, filename := filepath.Split(event.Name)
				id := strings.TrimSuffix(filename, filepath.Ext(filename))
				builder := stashedCommandBuilders.find(bot.BotType(), id)
				if builder == nil {
					break
				}

				log.Infof("start rebuilding command due to config file change: %s.", id)
				command, err := builder.Build(dir)
				if err != nil {
					log.Errorf("can't configure command. %s. %#v", err.Error(), builder)
					break
				}
				bot.AppendCommand(command) // replaces the old one.
			case event.Op&fsnotify.Remove == fsnotify.Remove:
			case event.Op&fsnotify.Rename == fsnotify.Rename:
			case event.Op&fsnotify.Chmod == fsnotify.Chmod:
			}
		case err := <-watcher.Errors:
			log.Errorf("error on subscribing to directory change: %s.", err.Error())
		}
	}
}

func setupScheduledTask(botCtx context.Context, bot Bot, c *cron.Cron, task *scheduledTask) {
	schedule := task.config.Schedule()
	err := c.AddFunc(schedule, func() {
		executeScheduledTask(botCtx, bot, task)
	})

	if err != nil {
		log.Errorf("failed to schedule a task. id: %s. schedule: %s. reason: %s.", task.Identifier(), schedule, err.Error())
	}
}

func executeScheduledTask(ctx context.Context, bot Bot, task *scheduledTask) {
	results, err := task.Execute(ctx)
	if err != nil {
		log.Errorf("error on scheduled task: %s", task.Identifier())
		return
	} else if results == nil {
		return
	}

	for _, res := range results {
		// The destination returned by task execution has higher priority.
		// e.g. RSS Reader's task searches for stored feed/destination set, and returns which destination to send.
		dest := res.Destination
		if dest == nil {
			// If no destination is given, see if default destination exists.
			// Useful when destination can be determined prior.
			// e.g. Weather forecast task always sends weather information to #goodmorning room.
			presetDest := task.config.Destination()
			if presetDest == nil {
				log.Errorf("task was completed, but destination was not set: %s.", task.Identifier())
				return
			}
			dest = presetDest
		}

		message := NewOutputMessage(dest, res.Content)
		bot.SendMessage(ctx, message)
	}
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
