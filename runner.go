package sarah

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/workers"
	"golang.org/x/xerrors"
	"runtime"
	"strings"
	"sync"
	"time"
)

var options = &optionHolder{}

// Config contains some basic configuration variables for go-sarah.
type Config struct {
	TimeZone string `json:"timezone" yaml:"timezone"`
}

// NewConfig creates and returns new Config instance with default settings.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to override default values.
func NewConfig() *Config {
	return &Config{
		TimeZone: time.Now().Location().String(),
	}
}

// optionHolder is a struct that stashes given options before go-sarah's initialization.
// This was formally called RunnerOptions and was provided publicly, but is now private in favor of https://github.com/oklahomer/go-sarah/issues/72
// Calls to its methods are thread-safe.
type optionHolder struct {
	mutex   sync.RWMutex
	stashed []func(*runner) error
}

func (o *optionHolder) register(opt func(*runner) error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.stashed = append(o.stashed, opt)
}

func (o *optionHolder) apply(r *runner) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	for _, v := range o.stashed {
		e := v(r)
		if e != nil {
			return e
		}
	}

	return nil
}

// RegisterAlerter registers given sarah.Alerter implementation.
// When registered sarah.Bot implementation encounters critical state, given alerter is called to notify such state.
func RegisterAlerter(alerter Alerter) {
	options.register(func(r *runner) error {
		r.alerters.appendAlerter(alerter)
		return nil
	})
}

// RegisterBot registers given sarah.Bot implementation to be run on sarah.Run().
// This may be called multiple times to register as many bot instances as wanted.
// When a Bot with same sarah.BotType is already registered, this returns error on sarah.Run().
func RegisterBot(bot Bot) {
	options.register(func(r *runner) error {
		r.bots = append(r.bots, bot)
		return nil
	})
}

// RegisterCommand registers given sarah.Command.
// On sarah.Run(), Commands are registered to corresponding bot via Bot.AppendCommand().
func RegisterCommand(botType BotType, command Command) {
	options.register(func(r *runner) error {
		commands, ok := r.commands[botType]
		if !ok {
			commands = []Command{}
		}
		r.commands[botType] = append(commands, command)
		return nil
	})
}

// RegisterCommandProps registers given sarah.CommandProps to build sarah.Command on sarah.Run().
// This props is re-used when configuration file is updated and a corresponding sarah.Command needs to be re-built.
func RegisterCommandProps(props *CommandProps) {
	options.register(func(r *runner) error {
		stashed, ok := r.commandProps[props.botType]
		if !ok {
			stashed = []*CommandProps{}
		}
		r.commandProps[props.botType] = append(stashed, props)
		return nil
	})
}

// RegisterScheduledTask registers given sarah.ScheduledTask.
// On sarah.Run(), schedule is set for this task.
func RegisterScheduledTask(botType BotType, task ScheduledTask) {
	options.register(func(r *runner) error {
		tasks, ok := r.scheduledTasks[botType]
		if !ok {
			tasks = []ScheduledTask{}
		}
		r.scheduledTasks[botType] = append(tasks, task)
		return nil
	})
}

// RegisterScheduledTaskProps registers given sarah.ScheduledTaskProps to build sarah.ScheduledTask on sarah.Run().
// This props is re-used when configuration file is updated and a corresponding sarah.ScheduledTask needs to be re-built.
func RegisterScheduledTaskProps(props *ScheduledTaskProps) {
	options.register(func(r *runner) error {
		stashed, ok := r.scheduledTaskProps[props.botType]
		if !ok {
			stashed = []*ScheduledTaskProps{}
		}
		r.scheduledTaskProps[props.botType] = append(stashed, props)
		return nil
	})
}

// RegisterConfigWatcher registers given ConfigWatcher implementation.
func RegisterConfigWatcher(watcher ConfigWatcher) {
	options.register(func(r *runner) error {
		r.configWatcher = watcher
		return nil
	})
}

// RegisterWorker registers given workers.Worker implementation.
// When this is not called, a worker instance with default setting is used.
func RegisterWorker(worker workers.Worker) {
	options.register(func(r *runner) error {
		r.worker = worker
		return nil
	})
}

// RegisterBotErrorSupervisor registers a given supervising function that is called when a Bot escalates an error.
// This function judges if the given error is worth being notified to administrators and if the Bot should stop.
// A developer may return *SupervisionDirective to tell such order.
// If the escalated error can simply be ignored, a nil value can be returned.
//
// Bot/Adapter can escalate an error via a function, func(error), that is passed to Run() as a third argument.
// When BotNonContinuableError is escalated, go-sarah's core cancels failing Bot's context and thus the Bot and related resources stop working.
// If one or more sarah.Alerters implementations are registered, such critical error is passed to the alerters and administrators will be notified.
// When other types of error are escalated, the error is passed to the supervising function registered via sarah.RegisterBotErrorSupervisor().
// The function may return *SupervisionDirective to tell how go-sarah's core should react.
//
// Bot/Adapter's implementation should be simple. It should not handle serious errors by itself.
// Instead, it should simply escalate an error every time when a noteworthy error occurs and let core judge how to react.
// For example, if the bot should stop when three reconnection trial fails in ten seconds, the scenario could be somewhat like below:
//   1. Bot escalates reconnection error, FooReconnectionFailureError, each time it fails to reconnect
//   2. Supervising function counts the error and ignores the first two occurrence
//   3. When the third error comes within ten seconds from the initial error escalation, return *SupervisionDirective with StopBot value of true
//
// Similarly, if there should be a rate limiter to limit the calls to alerters, the supervising function should take care of this instead of the failing Bot.
// Each Bot/Adapter's implementation can be kept simple in this way.
// go-sarah's core should always supervise and control its belonging Bots.
func RegisterBotErrorSupervisor(fnc func(BotType, error) *SupervisionDirective) {
	options.register(func(r *runner) error {
		r.superviseError = fnc
		return nil
	})
}

// Run is a non-blocking function that starts running go-sarah's process with pre-registered options.
// Workers, schedulers and other required resources for bot interaction starts running on this function call.
// This returns error when bot interaction cannot start; No error is returned when process starts successfully.
//
// Refer to ctx.Done() or sarah.CurrentStatus() to reference current running status.
//
// To control its lifecycle, a developer may cancel ctx to stop go-sarah at any moment.
// When bot interaction stops unintentionally without such context cancellation,
// the critical state is notified to administrators via registered sarah.Alerter.
// This is recommended to register multiple sarah.Alerter implementations to make sure critical states are notified.
func Run(ctx context.Context, config *Config) error {
	err := runnerStatus.start()
	if err != nil {
		return xerrors.Errorf("failed to start bot process: %w", err)
	}

	runner, err := newRunner(ctx, config)
	if err != nil {
		return xerrors.Errorf("failed to start bot process: %w", err)
	}
	go runner.run(ctx)

	return nil
}

func newRunner(ctx context.Context, config *Config) (*runner, error) {
	loc, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		return nil, xerrors.Errorf(`given timezone "%s" cannot be converted to time.Location: %w`, config.TimeZone, err)
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

	err = options.apply(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to apply option: %w", err)
	}

	if r.worker == nil {
		w, e := workers.Run(ctx, workers.NewConfig())
		if e != nil {
			return nil, xerrors.Errorf("worker could not run: %w", e)
		}

		r.worker = w
	}

	return r, nil
}

type runner struct {
	config             *Config
	bots               []Bot
	worker             workers.Worker
	configWatcher      ConfigWatcher
	commands           map[BotType][]Command
	commandProps       map[BotType][]*CommandProps
	scheduledTasks     map[BotType][]ScheduledTask
	scheduledTaskProps map[BotType][]*ScheduledTaskProps
	alerters           *alerters
	scheduler          scheduler
	superviseError     func(BotType, error) *SupervisionDirective
}

// SupervisionDirective tells go-sarah's core how to react when a Bot escalates an error.
// A customized supervisor can be defined and registered via RegisterBotErrorSupervisor().
type SupervisionDirective struct {
	// StopBot tells the core to stop the failing Bot and cleanup related resources.
	// When two or more Bots are registered and one or more Bots are to be still running after the failing Bot stops,
	// internal workers and scheduler keep running.
	//
	// When all Bots stop, then the core stops all resources.
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

		go func(b Bot) {
			defer func() {
				wg.Done()
				runnerStatus.stopBot(b)
			}()

			runnerStatus.addBot(b)
			r.runBot(ctx, b)
		}(bot)

	}
	wg.Wait()
}

func unsubscribeConfigWatcher(watcher ConfigWatcher, botType BotType) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Failed to unsubscribe ConfigWatcher for %s: %+v", botType, r)
		}
	}()
	err := watcher.Unwatch(botType)
	if err != nil {
		log.Errorf("Failed to unsubscribe ConfigWatcher for %s: %+v", botType, err)
	}
}

// runBot runs given Bot implementation in a blocking manner.
// This returns when bot stops.
func (r *runner) runBot(runnerCtx context.Context, bot Bot) {
	log.Infof("Starting %s", bot.BotType())
	botCtx, errNotifier := r.superviseBot(runnerCtx, bot.BotType())

	// Build commands with stashed CommandProps.
	r.registerCommands(botCtx, bot)

	// Register scheduled tasks.
	r.registerScheduledTasks(botCtx, bot)

	inputReceiver := setupInputReceiver(botCtx, bot, r.worker)

	// Run Bot in a panic-proof manner
	func() {
		defer func() {
			// When Bot panics, recover and tell as much detailed information as possible via error notification channel.
			// Notified channel sends alert to notify administrator.
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

			// Explicitly send *BotNonContinuableError to make sure bot context is canceled and administrators are notified.
			// This is effective when Bot implementation stops running without notifying its critical state by sending *BotNonContinuableError to errNotifier.
			// Error sent here is simply ignored when Bot context is already canceled by previous *BotNonContinuableError notification.
			errNotifier(NewBotNonContinuableError(fmt.Sprintf("shutdown bot: %s", bot.BotType())))
		}()

		bot.Run(botCtx, inputReceiver, errNotifier)
		unsubscribeConfigWatcher(r.configWatcher, bot.BotType())
	}()
}

func (r *runner) superviseBot(runnerCtx context.Context, botType BotType) (context.Context, func(error)) {
	botCtx, cancel := context.WithCancel(runnerCtx)

	sendAlert := func(err error) {
		e := r.alerters.alertAll(runnerCtx, botType, err)
		if e != nil {
			log.Errorf("Failed to send alert for %s: %+v", botType, e)
		}
	}

	stopBot := func() {
		cancel()
		log.Infof("Stop supervising bot's critical error due to its context cancellation: %s.", botType)
	}

	// A function that receives an escalated error from Bot.
	// If critical error is sent, this cancels Bot context to finish its lifecycle.
	// Bot itself MUST NOT kill itself, but the Runner does. Beware that Runner takes care of all related components' lifecycle.
	handleError := func(err error) {
		switch err.(type) {
		case *BotNonContinuableError:
			log.Errorf("Stop unrecoverable bot. BotType: %s. Error: %+v", botType, err)

			stopBot()

			go sendAlert(err)

		default:
			if r.superviseError != nil {
				directive := r.superviseError(botType, err)
				if directive == nil {
					return
				}

				if directive.StopBot {
					log.Errorf("Stop bot due to given directive. BotType: %s. Reason: %+v", botType, err)
					stopBot()
				}

				if directive.AlertingErr != nil {
					go sendAlert(directive.AlertingErr)
				}
			}

		}
	}

	// A function to be exposed to Bot/Adapter developers.
	// When Bot/Adapter faces a critical state, it can call this function to let Runner judge the severity and stop Bot if necessary.
	errNotifier := func(err error) {
		select {
		case <-botCtx.Done():
			// Bot context is already canceled by preceding error notification. Do nothing.
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
			log.Errorf("Failed to build command %#v: %+v", p, err)
			return
		}
		bot.AppendCommand(command)
	}

	callback := func(p *CommandProps) func() {
		return func() {
			log.Infof("Updating command: %s", p.identifier)
			reg(p)
			return
		}
	}

	for _, p := range props {
		reg(p)
		err := r.configWatcher.Watch(botCtx, bot.BotType(), p.identifier, callback(p))
		if err != nil {
			log.Errorf("Failed to subscribe configuration for command %s: %+v", p.identifier, err)
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
			log.Errorf("Failed to build scheduled task %s: %+v", p.identifier, err)
			return
		}

		err = r.scheduler.update(bot.BotType(), task, func() {
			executeScheduledTask(botCtx, bot, task)
		})
		if err != nil {
			log.Errorf("Failed to schedule a task. ID: %s: %+v", task.Identifier(), err)
		}
	}

	callback := func(p *ScheduledTaskProps) func() {
		return func() {
			log.Infof("Updating scheduled task: %s", p.identifier)
			reg(p)
			return
		}
	}

	for _, p := range r.botScheduledTaskProps(bot.BotType()) {
		reg(p)
		err := r.configWatcher.Watch(botCtx, bot.BotType(), p.identifier, callback(p))
		if err != nil {
			log.Errorf("Failed to subscribe configuration for scheduled task %s: %+v", p.identifier, err)
			continue
		}
	}

	for _, task := range r.botScheduledTasks(bot.BotType()) {
		if task.Schedule() == "" {
			log.Errorf("Failed to schedule a task. ID: %s. Reason: %s.", task.Identifier(), "No schedule given.")
			continue
		}

		err := r.scheduler.update(bot.BotType(), task, func() {
			executeScheduledTask(botCtx, bot, task)
		})
		if err != nil {
			log.Errorf("Failed to schedule a task. id: %s: %+v", task.Identifier(), err)
		}
	}
}

func executeScheduledTask(ctx context.Context, bot Bot, task ScheduledTask) {
	results, err := task.Execute(ctx)
	if err != nil {
		log.Errorf("Error on scheduled task: %s", task.Identifier())
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
				log.Errorf("Task was completed, but destination was not set: %s.", task.Identifier())
				continue
			}
			dest = presetDest
		}

		message := NewOutputMessage(dest, res.Content)
		bot.SendMessage(ctx, message)
	}
}

func setupInputReceiver(botCtx context.Context, bot Bot, worker workers.Worker) func(Input) error {
	continuousEnqueueErrCnt := 0
	return func(input Input) error {
		err := worker.Enqueue(func() {
			err := bot.Respond(botCtx, input)
			if err != nil {
				log.Errorf("Error on message handling. Input: %#v. Error: %+v", input, err)
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
