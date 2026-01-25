package sarah

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-kasumi/worker"
	"runtime"
	"strings"
	"sync"
	"time"
)

var options = &optionHolder{}

// Config is a serializable struct that contains some configuration variables.
type Config struct {
	// TimeZone tells the scheduler in what timezone the application runs.
	TimeZone string `json:"timezone" yaml:"timezone"`
}

// NewConfig creates and returns a new Config instance with default settings.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to override those default values.
func NewConfig() *Config {
	return &Config{
		TimeZone: time.Now().Location().String(),
	}
}

// optionHolder is a struct that stashes and holds given options until Sarah boots up.
// Those options are applied to Sarah on Run execution to manipulate Sarah's behavior.
// This was formally called RunnerOptions and was provided publicly, but is now private in favor of https://github.com/oklahomer/go-sarah/issues/72 .
// Calls to its methods are thread-safe.
type optionHolder struct {
	mutex   sync.RWMutex
	stashed []func(*runner)
}

func (o *optionHolder) register(opt func(*runner)) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.stashed = append(o.stashed, opt)
}

func (o *optionHolder) apply(r *runner) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	for _, v := range o.stashed {
		v(r)
	}
}

// RegisterAlerter registers a given Alerter implementation to Sarah.
// When Sarah's process or a registered Bot implementation encounters a critical state, Alerter.Alert is called to notify such state.
// A developer may call this method multiple times to register multiple Alerters.
func RegisterAlerter(alerter Alerter) {
	options.register(func(r *runner) {
		r.alerters.appendAlerter(alerter)
	})
}

// RegisterBot registers a given Bot implementation to be run on Run call.
// This may be called multiple times to register as many bot instances as wanted.
func RegisterBot(bot Bot) {
	options.register(func(r *runner) {
		r.bots = append(r.bots, bot)
	})
}

// RegisterCommand registers a given Command implementation.
// On Run, each Command implementation is registered to the corresponding bot via Bot.AppendCommand.
// A Bot is considered to "correspond" when its BotType matches with the botType.
func RegisterCommand(botType BotType, command Command) {
	options.register(func(r *runner) {
		commands, ok := r.commands[botType]
		if !ok {
			commands = []Command{}
		}
		r.commands[botType] = append(commands, command)
	})
}

// RegisterCommandProps registers a given CommandProps to build Command implementation on Run call.
// This instance is reused when a configuration is updated and the corresponding Command needs to be rebuilt to reflect the changes.
func RegisterCommandProps(props *CommandProps) {
	options.register(func(r *runner) {
		stashed, ok := r.commandProps[props.botType]
		if !ok {
			stashed = []*CommandProps{}
		}
		r.commandProps[props.botType] = append(stashed, props)
	})
}

// RegisterScheduledTask registers a given ScheduledTask to Sarah.
// On Run, a schedule is set for this task.
func RegisterScheduledTask(botType BotType, task ScheduledTask) {
	options.register(func(r *runner) {
		tasks, ok := r.scheduledTasks[botType]
		if !ok {
			tasks = []ScheduledTask{}
		}
		r.scheduledTasks[botType] = append(tasks, task)
	})
}

// RegisterScheduledTaskProps registers a given ScheduledTaskProps to build ScheduledTask on Run call.
// This instance is reused when a configuration file is updated and the corresponding ScheduledTask needs to be rebuilt.
func RegisterScheduledTaskProps(props *ScheduledTaskProps) {
	options.register(func(r *runner) {
		stashed, ok := r.scheduledTaskProps[props.botType]
		if !ok {
			stashed = []*ScheduledTaskProps{}
		}
		r.scheduledTaskProps[props.botType] = append(stashed, props)
	})
}

// RegisterConfigWatcher registers a given ConfigWatcher implementation to Sarah.
// If a ConfigWatcher is registered, Sarah's process subscribes to the changes to Command or ScheduledTask's configuration.
// When a configuration is updated, ConfigWatcher reads the new configuration setting and reflects to the corresponding configuration instance
// so Sarah can rebuild the corresponding Command or ScheduledTask with the new setting.
func RegisterConfigWatcher(watcher ConfigWatcher) {
	options.register(func(r *runner) {
		r.configWatcher = watcher
	})
}

// RegisterWorker registers a given worker.Worker implementation to Sarah.
// When one is not registered, a worker instance with default setting is used.
func RegisterWorker(worker worker.Worker) {
	options.register(func(r *runner) {
		r.worker = worker
	})
}

// RegisterBotErrorSupervisor registers a given supervising function that is called when a Bot escalates an error.
// This function judges if the given error is worth being notified to administrators and if the Bot should stop.
// When an action is required, the function may return non-nil *SupervisionDirective to pass the order;
// Return nil when the escalated error can simply be ignored.
//
// Bot and Adapter can escalate an error via a function -- func(error) -- that is passed to Bot.Run as a third argument.
// When BotNonContinuableError is escalated, Sarah cancels the failing Bot's context, and thus the Bot and its related resources stop working.
// If one or more Alerter implementations are registered, such critical error is passed to those Alerters and administrators will be notified.
// When other types of error are escalated, the error is passed to the supervising function registered via RegisterBotErrorSupervisor.
// The function may return *SupervisionDirective to tell how Sarah should react.
//
// Bot and Adapter's implementation should be simple. It should not handle serious errors by itself.
// Instead, they should simply escalate an error every time when a noteworthy error occurs and let Sarah judge how to react.
// For example, if the bot should stop when three reconnection trial fails in ten seconds, the scenario could be somewhat like below:
//	1. Bot escalates reconnection error, FooReconnectionFailureError, each time it fails to reconnect
//	2. The supervising function counts the error and ignores the first two occurrence
// 	3. When the third error comes within ten seconds from the initial error escalation, return *SupervisionDirective with StopBot value of true
//
// Similarly, if there should be a rate limiter to limit the calls to Alerters, the supervising function should take care of this instead of the failing Bot.
// Each Bot or Adapter's implementation can be kept simple in this way; Sarah should always supervise and control its belonging Bots.
func RegisterBotErrorSupervisor(fnc func(BotType, error) *SupervisionDirective) {
	options.register(func(r *runner) {
		r.superviseError = fnc
	})
}

// Run sets up all required resources and initiates Sarah.
// Workers, schedulers, and other required resources for a bot interaction start running on this function call.
// This returns an error when bot interaction cannot start; No error is returned when the process starts successfully.
//
// Call ctx.Done or CurrentStatus to reference current running status.
//
// To control its lifecycle, a developer may cancel ctx and stop Sarah at any moment.
// When bot interaction stops unintentionally without such context cancellation,
// the critical state is notified to administrators via registered Alerter.
// Registering multiple Alerter implementations to ensure successful notification is recommended.
func Run(ctx context.Context, config *Config) error {
	err := runnerStatus.start()
	if err != nil {
		return fmt.Errorf("failed to start bot process: %w", err)
	}

	runner, err := newRunner(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start bot process: %w", err)
	}
	go runner.run(ctx)

	return nil
}

func newRunner(ctx context.Context, config *Config) (*runner, error) {
	loc, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		return nil, fmt.Errorf(`given timezone "%s" cannot be converted to time.Location: %w`, config.TimeZone, err)
	}

	r := &runner{
		config:             config,
		bots:               []Bot{},
		worker:             nil,
		configWatcher:      &nullConfigWatcher{},
		commands:           make(map[BotType][]Command),
		commandProps:       make(map[BotType][]*CommandProps),
		scheduledTasks:     make(map[BotType][]ScheduledTask),
		scheduledTaskProps: make(map[BotType][]*ScheduledTaskProps),
		alerters:           &alerters{},
		scheduler:          runScheduler(ctx, loc),
		superviseError:     nil,
	}

	options.apply(r)

	if r.worker == nil {
		// When the jobs are CPU-intensive, the number of workers can be equal to the number of CPUs.
		// However, in general, bot interaction involves more IO-intensive jobs such as calling external Weather APIs
		// on behalf of the user. This setting expects up to a hundred jobs can work in a concurrent manner based on such premise.
		//
		// The queue size is set to ten, which is relatively small.
		// Instead of allowing larger latency with a bigger queue size, messages will soon be ignored when the worker is busy and the queue is full.
		// Users usually do not expect to have belated responses.
		//
		// Provide a worker.Worker implementation via RegisterWorker to customize the setting.
		workerConfig := worker.NewConfig()
		workerConfig.WorkerNum = 100
		workerConfig.QueueSize = 10
		r.worker = worker.Run(ctx, worker.NewConfig())
	}

	return r, nil
}

type runner struct {
	config             *Config
	bots               []Bot
	worker             worker.Worker
	configWatcher      ConfigWatcher
	commands           map[BotType][]Command
	commandProps       map[BotType][]*CommandProps
	scheduledTasks     map[BotType][]ScheduledTask
	scheduledTaskProps map[BotType][]*ScheduledTaskProps
	alerters           *alerters
	scheduler          scheduler
	superviseError     func(BotType, error) *SupervisionDirective
}

// SupervisionDirective tells Sarah how to react to Bot's escalating error.
//
// A designated supervisor function judges if the error represents a critical state when a bot escalates an error.
// When the bot is in a critical state, the function can return non-nil *SupervisionDirective to tell Sarah how to treat the current state.
// A customized supervisor function can be defined and registered via RegisterBotErrorSupervisor.
type SupervisionDirective struct {
	// StopBot tells if Sarah needs to stop the failing bot and cleanup related resources.
	// When two or more bots are registered and at least one bot is to stay running after the failing bot stops,
	// internal workers and a scheduler keep running.
	//
	// When all bots stop, then Sarah stops all resources.
	StopBot bool

	// AlertingErr is sent registered alerters and administrators will be notified.
	// Set nil when such alert notification is not required.
	AlertingErr error
}

func (r *runner) botCommands(botType BotType) []Command {
	if commands, ok := r.commands[botType]; ok {
		return commands
	}
	return []Command{}
}

func (r *runner) botCommandProps(botType BotType) []*CommandProps {
	if props, ok := r.commandProps[botType]; ok {
		return props
	}
	return []*CommandProps{}
}

func (r *runner) botScheduledTaskProps(botType BotType) []*ScheduledTaskProps {
	if props, ok := r.scheduledTaskProps[botType]; ok {
		return props
	}
	return []*ScheduledTaskProps{}
}

func (r *runner) botScheduledTasks(botType BotType) []ScheduledTask {
	if tasks, ok := r.scheduledTasks[botType]; ok {
		return tasks
	}
	return []ScheduledTask{}
}

func (r *runner) run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, bot := range r.bots {
		wg.Add(1)

		go func() {
			defer func() {
				wg.Done()
				runnerStatus.stopBot(bot)
			}()

			runnerStatus.addBot(bot)
			r.runBot(ctx, bot)
		}()
	}
	wg.Wait()
}

func unsubscribeConfigWatcher(watcher ConfigWatcher, botType BotType) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Failed to unsubscribe ConfigWatcher for %s: %+v", botType, r)
		}
	}()
	err := watcher.Unwatch(botType)
	if err != nil {
		logger.Errorf("Failed to unsubscribe ConfigWatcher for %s: %+v", botType, err)
	}
}

// runBot initiates the given Bot implementation and blocks until the bot stops.
func (r *runner) runBot(runnerCtx context.Context, bot Bot) {
	logger.Infof("Starting %s", bot.BotType())
	botCtx, errNotifier := r.superviseBot(runnerCtx, bot.BotType())

	// Build commands with stashed CommandProps.
	r.registerCommands(botCtx, bot)

	// Register scheduled tasks.
	r.registerScheduledTasks(botCtx, bot)

	inputReceiver := setupInputReceiver(botCtx, bot, r.worker)

	// Run the bot in a panic-proof manner.
	func() {
		defer func() {
			// When the bot panics, recover and tell as much detailed information as possible via the error notification channel.
			// The channel receiver sends an alert to the administrator.
			if r := recover(); r != nil {
				stack := []string{fmt.Sprintf("panic in bot: %s. %#v.", bot.BotType(), r)}

				// Inform stack trace
				for depth := 0; ; depth++ {
					_, src, line, ok := runtime.Caller(depth)
					if !ok {
						break
					}
					stack = append(stack, fmt.Sprintf(" -> depth:%d. file:%s. line:%d.", depth, src, line))
				}

				errNotifier(NewBotNonContinuableError(strings.Join(stack, "\n")))
			}

			// Bot.Run may return without internally sending an error to errNotifier.
			// To ensure the bot's context is canceled by Sarah and administrators are notified, explicitly send *BotNonContinuableError in this defer statement.
			// An error being sent here is simply ignored if the bot context is already canceled by a previous error notification.
			errNotifier(NewBotNonContinuableError(fmt.Sprintf("shutdown bot: %s", bot.BotType())))
		}()

		bot.Run(botCtx, inputReceiver, errNotifier) // Blocks til interaction ends
		unsubscribeConfigWatcher(r.configWatcher, bot.BotType())
	}()
}

func (r *runner) superviseBot(runnerCtx context.Context, botType BotType) (context.Context, func(error)) {
	botCtx, cancel := context.WithCancel(runnerCtx)

	sendAlert := func(err error) {
		e := r.alerters.alertAll(runnerCtx, botType, err)
		if e != nil {
			logger.Errorf("Failed to send alert for %s: %+v", botType, e)
		}
	}

	stopBot := func() {
		cancel()
		logger.Infof("Stop supervising bot's critical error due to its context cancellation: %s.", botType)
	}

	// A function that receives an escalated error from the bot.
	// If a critical error is sent, this cancels the bot's context to finish its lifecycle.
	// The bot MUST NOT kill itself, but Sarah does. Beware that Sarah takes care of all related components' lifecycle.
	handleError := func(err error) {
		switch err.(type) {
		case *BotNonContinuableError:
			logger.Errorf("Stop unrecoverable bot. BotType: %s. Error: %+v", botType, err)

			stopBot()

			go sendAlert(err)

		default:
			if r.superviseError != nil {
				directive := r.superviseError(botType, err)
				if directive == nil {
					return
				}

				if directive.StopBot {
					logger.Errorf("Stop bot due to given directive. BotType: %s. Reason: %+v", botType, err)
					stopBot()
				}

				if directive.AlertingErr != nil {
					go sendAlert(directive.AlertingErr)
				}
			}

		}
	}

	// A function to be exposed to Bot/Adapter developers.
	// When Bot implementation faces a critical state, the failing bot can call this function to let Sarah judge the severity and stop the bot if necessary.
	errNotifier := func(err error) {
		select {
		case <-botCtx.Done():
			// Bot context is already canceled by the preceding error notification. Do nothing.
			return

		default:
			handleError(err)
		}
	}

	return botCtx, errNotifier
}

func (r *runner) registerCommands(botCtx context.Context, bot Bot) {
	props := r.botCommandProps(bot.BotType())

	reg := func(p *CommandProps) {
		command, err := buildCommand(botCtx, p, r.configWatcher)
		if err != nil {
			logger.Errorf("Failed to build command %#v: %+v", p, err)
			return
		}
		bot.AppendCommand(command)
	}

	for _, p := range props {
		reg(p)
		err := r.configWatcher.Watch(botCtx, bot.BotType(), p.identifier, func() {
			logger.Infof("Updating command: %s", p.identifier)
			reg(p)
		})
		if err != nil {
			logger.Errorf("Failed to subscribe configuration for command %s: %+v", p.identifier, err)
			continue
		}
	}

	for _, command := range r.botCommands(bot.BotType()) {
		bot.AppendCommand(command)
	}
}

func (r *runner) registerScheduledTasks(botCtx context.Context, bot Bot) {
	reg := func(p *ScheduledTaskProps) {
		r.scheduler.remove(bot.BotType(), p.identifier)

		task, err := buildScheduledTask(botCtx, p, r.configWatcher)
		if err != nil {
			logger.Errorf("Failed to build scheduled task %s: %+v", p.identifier, err)
			return
		}

		err = r.scheduler.update(bot.BotType(), task, func() {
			executeScheduledTask(botCtx, bot, task)
		})
		if err != nil {
			logger.Errorf("Failed to schedule a task. ID: %s: %+v", task.Identifier(), err)
		}
	}

	for _, p := range r.botScheduledTaskProps(bot.BotType()) {
		reg(p)
		err := r.configWatcher.Watch(botCtx, bot.BotType(), p.identifier, func() {
			logger.Infof("Updating scheduled task: %s", p.identifier)
			reg(p)
		})
		if err != nil {
			logger.Errorf("Failed to subscribe configuration for scheduled task %s: %+v", p.identifier, err)
			continue
		}
	}

	for _, task := range r.botScheduledTasks(bot.BotType()) {
		if task.Schedule() == "" {
			logger.Errorf("Failed to schedule a task. ID: %s. Reason: %s.", task.Identifier(), "No schedule given.")
			continue
		}

		err := r.scheduler.update(bot.BotType(), task, func() {
			executeScheduledTask(botCtx, bot, task)
		})
		if err != nil {
			logger.Errorf("Failed to schedule a task. id: %s: %+v", task.Identifier(), err)
		}
	}
}

func executeScheduledTask(ctx context.Context, bot Bot, task ScheduledTask) {
	results, err := task.Execute(ctx)
	if err != nil {
		logger.Errorf("Error on scheduled task: %s", task.Identifier())
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
			// Useful when destination can be preset.
			// e.g. Weather forecast task always sends weather information to #goodmorning room.
			presetDest := task.DefaultDestination()
			if presetDest == nil {
				logger.Errorf("Task was completed, but destination was not set: %s.", task.Identifier())
				continue
			}
			dest = presetDest
		}

		message := NewOutputMessage(dest, res.Content)
		bot.SendMessage(ctx, message)
	}
}

func setupInputReceiver(botCtx context.Context, bot Bot, wkr worker.Worker) func(Input) error {
	continuousEnqueueErrCnt := 0
	return func(input Input) error {
		err := wkr.Enqueue(func() {
			err := bot.Respond(botCtx, input)
			if err != nil {
				logger.Errorf("Error on message handling. Input: %#v. Error: %+v", input, err)
			}
		})

		if err == nil {
			continuousEnqueueErrCnt = 0
			return nil

		}

		continuousEnqueueErrCnt++
		// Could not send because probably the workers are too busy or the runner context is already canceled.
		return NewBlockedInputError(continuousEnqueueErrCnt)
	}
}
