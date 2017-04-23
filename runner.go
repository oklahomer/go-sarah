package sarah

import (
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/worker"
	"golang.org/x/net/context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Config contains some configuration variables for Runner and its underlying structs.
type Config struct {
	Worker           *worker.Config `json:"worker" yaml:"worker"`
	PluginConfigRoot string         `json:"plugin_config_root" yaml:"plugin_config_root"`
	TimeZone         string         `json:"timezone" yaml:"timezone"`
}

// NewConfig creates and returns new Config instance with default settings.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to overload default values.
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
	config            *Config
	bots              []Bot
	commandProps      map[BotType][]*CommandProps
	scheduledTaskPrps map[BotType][]*ScheduledTaskProps
	scheduledTasks    map[BotType][]ScheduledTask
	alerters          *alerters
}

// NewRunner creates and return new Runner instance.
func NewRunner(config *Config, options ...RunnerOption) (*Runner, error) {
	runner := &Runner{
		config:            config,
		bots:              []Bot{},
		commandProps:      make(map[BotType][]*CommandProps),
		scheduledTaskPrps: make(map[BotType][]*ScheduledTaskProps),
		scheduledTasks:    make(map[BotType][]ScheduledTask),
		alerters:          &alerters{},
	}

	for _, opt := range options {
		err := opt(runner)
		if err != nil {
			return nil, err
		}
	}

	return runner, nil
}

// RunnerOption defines function that Runner's functional option must satisfy.
type RunnerOption func(runner *Runner) error

// RunnerOptions stashes RunnerOption for later use with NewRunner().
//
// On typical setup, especially when a process consists of multiple Bots and Commands, each construction step requires more lines of codes.
// Each step ends with creating new RunnerOption instance to be fed to NewRunner(), but as code gets longer it gets hard to keep track of each RunnerOption.
// In that case RunnerOptions becomes a handy helper to temporary stash RunnerOption.
//
//  options := NewRunnerOptions()
//
//  // 5-10 lines of codes to configure Slack bot.
//  slackBot, _ := sarah.NewBot(slack.NewAdapter(slackConfig), sarah.BotWithStorage(storage))
//  options.Append(sarah.WithBot(slackBot))
//
//  // Here comes other 5-10 codes to configure another bot
//  myBot, _ := NewMyBot(...)
//  optionsAppend(sarah.WithBot(myBot))
//
//  // Some more codes to register Commands / ScheduledTasks
//  myTask := customizedTask()
//  options.Append(sarah.WithScheduledTask(myTask))
//
//  // Finally feed stashed options to NewRunner at once
//  runner, _ := NewRunner(sarah.NewConfig(), options.Arg())
//  runner.Run(ctx)
type RunnerOptions []RunnerOption

// NewRunnerOptions creates and returns new RunnerOptions instance.
func NewRunnerOptions() *RunnerOptions {
	return &RunnerOptions{}
}

// Append adds given RunnerOption to internal stash.
func (options *RunnerOptions) Append(opt RunnerOption) {
	*options = append(*options, opt)
}

// Arg returns stashed RunnerOptions in a form that can be directly fed to NewRunner's second argument.
func (options *RunnerOptions) Arg() RunnerOption {
	return func(runner *Runner) error {
		for _, opt := range *options {
			err := opt(runner)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithBot creates RunnerOption with given Bot implementation.
func WithBot(bot Bot) RunnerOption {
	return func(runner *Runner) error {
		runner.bots = append(runner.bots, bot)
		return nil
	}
}

// WithCommandProps creates RunnerOption with given CommandProps.
// Command is built on Runner.Run with given CommandProps.
// This props is re-used when configuration file is updated and Command needs to be re-built.
func WithCommandProps(props *CommandProps) RunnerOption {
	return func(runner *Runner) error {
		stashed, ok := runner.commandProps[props.botType]
		if !ok {
			stashed = []*CommandProps{}
		}
		runner.commandProps[props.botType] = append(stashed, props)
		return nil
	}
}

// WithScheduledTaskProps creates RunnerOption with given ScheduledTaskProps.
// ScheduledTask is built on Runner.Run with given ScheduledTaskProps.
// This props is re-used when configuration file is updated and ScheduledTask needs to be re-built.
func WithScheduledTaskProps(props *ScheduledTaskProps) RunnerOption {
	return func(runner *Runner) error {
		stashed, ok := runner.scheduledTaskPrps[props.botType]
		if !ok {
			stashed = []*ScheduledTaskProps{}
		}
		runner.scheduledTaskPrps[props.botType] = append(stashed, props)
		return nil
	}
}

// WithScheduledTask creates RunnerOperation with given ScheduledTask.
func WithScheduledTask(botType BotType, task ScheduledTask) RunnerOption {
	return func(runner *Runner) error {
		tasks, ok := runner.scheduledTasks[botType]
		if !ok {
			tasks = []ScheduledTask{}
		}
		runner.scheduledTasks[botType] = append(tasks, task)
		return nil
	}
}

// WithAlerter creates RunnerOperation with given Alerter.
func WithAlerter(alerter Alerter) RunnerOption {
	return func(runner *Runner) error {
		runner.alerters.appendAlerter(alerter)
		return nil
	}
}

func (runner *Runner) botCommandProps(botType BotType) []*CommandProps {
	if props, ok := runner.commandProps[botType]; ok {
		return props
	}
	return []*CommandProps{}
}

func (runner *Runner) botScheduledTaskProps(botType BotType) []*ScheduledTaskProps {
	if props, ok := runner.scheduledTaskPrps[botType]; ok {
		return props
	}
	return []*ScheduledTaskProps{}
}

func (runner *Runner) botScheduledTasks(botType BotType) []ScheduledTask {
	if tasks, ok := runner.scheduledTasks[botType]; ok {
		return tasks
	}
	return []ScheduledTask{}
}

// Run starts Bot interaction.
// At this point Runner starts its internal workers and schedulers, runs each bot, and starts listening to incoming messages.
func (runner *Runner) Run(ctx context.Context) {
	workerJob := worker.Run(ctx, runner.config.Worker)

	loc, locErr := time.LoadLocation(runner.config.TimeZone)
	if locErr != nil {
		panic(fmt.Sprintf("Given timezone can't be converted to time.Location: %s.", locErr.Error()))
	}
	taskScheduler := runScheduler(ctx, loc)

	watcher, err := runConfigWatcher(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to run watcher: %s.", err.Error()))
	}

	var wg sync.WaitGroup
	for _, bot := range runner.bots {
		wg.Add(1)

		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// Each Bot has its own context propagating Runner's lifecycle.
		botCtx, errNotifier := botSupervisor(ctx, botType, runner.alerters)

		// Prepare function  that receives Input.
		receiveInput := setupInputReceiver(ctx, bot, workerJob)

		// Run Bot
		go runBot(botCtx, bot, receiveInput, errNotifier)

		// Setup config directory.
		var configDir string
		if runner.config.PluginConfigRoot != "" {
			configDir = filepath.Join(runner.config.PluginConfigRoot, strings.ToLower(bot.BotType().String()))
		}

		// Build commands with stashed CommandProps
		commandProps := runner.botCommandProps(botType)
		registerCommands(bot, commandProps, configDir)

		// supervise configuration files' directory for commands
		if configDir != "" {
			callback := commandUpdaterFunc(bot, commandProps)
			err := watcher.watch(botCtx, botType, configDir, callback)
			if err != nil {
				log.Errorf("failed to watch %s: %s", configDir, err.Error())
			}
		}

		// Register scheduled tasks.
		tasks := runner.botScheduledTasks(botType)
		taskProps := runner.botScheduledTaskProps(botType)
		registerScheduledTasks(botCtx, bot, tasks, taskProps, taskScheduler, configDir)

		// supervise configuration files' directory for scheduled tasks
		if configDir != "" {
			callback := scheduledTaskUpdaterFunc(botCtx, bot, taskProps, taskScheduler)
			err := watcher.watch(botCtx, botType, configDir, callback)
			if err != nil {
				log.Errorf("failed to watch %s: %s", configDir, err.Error())
			}
		}

		go func(c context.Context) {
			select {
			case <-c.Done():
				wg.Done()
			}
		}(botCtx)
	}

	wg.Wait()
}

func runBot(ctx context.Context, bot Bot, receiveInput func(Input) error, errNotifier func(error)) {
	// When bot panics, recover and tell as much detailed information as possible via error notification channel.
	// Notified channel sends alert to notify administrator.
	defer func() {
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
	}()

	bot.Run(ctx, receiveInput, errNotifier)
}

func registerCommand(bot Bot, command Command) {
	bot.AppendCommand(command)
}

func registerScheduledTask(botCtx context.Context, bot Bot, task ScheduledTask, taskScheduler scheduler) {
	err := taskScheduler.update(bot.BotType(), task, func() {
		executeScheduledTask(botCtx, bot, task)
	})
	if err != nil {
		log.Errorf("failed to schedule a task. id: %s. reason: %s.", task.Identifier(), err.Error())
	}
}

func registerCommands(bot Bot, props []*CommandProps, configDir string) {
	for _, props := range props {
		command, err := newCommand(props, configDir)
		if err != nil {
			log.Errorf("can't configure command. %s. %#v", err.Error(), props)
			continue
		}

		registerCommand(bot, command)
	}
}

func registerScheduledTasks(botCtx context.Context, bot Bot, tasks []ScheduledTask, props []*ScheduledTaskProps, taskScheduler scheduler, configDir string) {
	for _, props := range props {
		task, err := newScheduledTask(props, configDir)
		if err != nil {
			log.Errorf("can't configure scheduled task: %s. %#v.", err.Error(), props)
			continue
		}
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		// Make sure schedule is given. Especially those pre-registered tasks.
		if task.Schedule() == "" {
			log.Errorf("failed to schedule a task. id: %s. reason: %s.", task.Identifier(), "No schedule given.")
			continue
		}

		registerScheduledTask(botCtx, bot, task, taskScheduler)
	}
}

func commandUpdaterFunc(bot Bot, props []*CommandProps) func(string) {
	return func(path string) {
		dir, filename := filepath.Split(path)
		id := strings.TrimSuffix(filename, filepath.Ext(filename)) // buzz.yaml to buzz

		for _, p := range props {
			if p.identifier != id {
				continue
			}
			log.Infof("start rebuilding command due to config file change: %s.", id)
			command, err := newCommand(p, dir)
			if err != nil {
				log.Errorf("can't configure command. id: %s. %s", p.identifier, err.Error())
				return
			}

			registerCommand(bot, command) // replaces the old one.
			return
		}
	}
}

func scheduledTaskUpdaterFunc(botCtx context.Context, bot Bot, taskProps []*ScheduledTaskProps, taskScheduler scheduler) func(string) {
	return func(path string) {
		dir, filename := filepath.Split(path)
		id := strings.TrimSuffix(filename, filepath.Ext(filename)) // buzz.yaml to buzz

		for _, p := range taskProps {
			if p.identifier != id {
				continue
			}

			log.Infof("start rebuilding scheduled task due to config file change: %s.", id)
			task, err := newScheduledTask(p, dir)
			if err != nil {
				log.Errorf("can't configure scheduled task. id: %s. %s", p.identifier, err.Error())
				return
			}

			registerScheduledTask(botCtx, bot, task, taskScheduler)
			return
		}
	}
}

func executeScheduledTask(ctx context.Context, bot Bot, task ScheduledTask) {
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
			presetDest := task.DefaultDestination()
			if presetDest == nil {
				log.Errorf("task was completed, but destination was not set: %s.", task.Identifier())
				continue
			}
			dest = presetDest
		}

		message := NewOutputMessage(dest, res.Content)
		bot.SendMessage(ctx, message)
	}
}

func botSupervisor(runnerCtx context.Context, botType BotType, alerters *alerters) (context.Context, func(error)) {
	botCtx, cancel := context.WithCancel(runnerCtx)
	errCh := make(chan error)

	// Run a goroutine that supervises Bot's critical state.
	// If critical error is sent from Bot, this cancels Bot context to finish its lifecycle.
	// Bot itself MUST not kill itself, but the Runner does. Beware that Runner takes care of all related components' lifecycle.
	activated := make(chan struct{})
	go func() {
		signalVal := struct{}{} // avoid multiple construction
		for {
			select {
			case activated <- signalVal:
				// Send sentinel value to make sure this goroutine is all ready by the end of this method call.
				// This blocks once the value is sent because of the nature of non-buffered channel and one-time subscription.

			case e := <-errCh:
				switch e.(type) {
				case *BotNonContinuableError:
					log.Errorf("stop unrecoverable bot. BotType: %s. error: %s.", botType.String(), e.Error())
					cancel()
					err := alerters.alertAll(runnerCtx, botType, e)
					if err != nil {
						log.Errorf("failed to send alert for %s: %s", botType.String(), err.Error())
					}

					// Doesn't require return statement at this point.
					// Call to cancel() causes Bot context cancellation, and hence below botCtx.Done block works.
					// Until then let this case statement listen to other errors during Bot stopping stage, so that desired logging may work.
				}

			case <-botCtx.Done():
				// Since the cancel() is locally stored in this botSupervisor function and is completely handled inside of this function,
				// but botCtx can also be cancelled by upper level context: runner context.
				// So be sure to subscribe to botCtx.Done()
				log.Infof("stop supervising bot critical error due to context cancelation: %s.", botType.String())
				return
			}
		}
	}()
	// Wait til above goroutine is ready.
	// Test shows there is a chance that goroutine is not fully activated right after this method call,
	// so if critical error is notified soon after this setup, the error may fall into default case in the below select statement.
	<-activated

	// Instead of simply returning a channel to receive error, return a function that receive error.
	// This function takes care of channel blocking, so the calling Bot implementation does not have to worry about it.
	errNotifier := func(err error) {
		// Try notifying critical error state to runner, gives up if the runner is already stopping the corresponding Bot.
		// This may occur when multiple parts of Bot/Adapter are notifying critical state and the first one caused Bot stop.
		select {
		case errCh <- err:
			// Successfully sent without blocking.
		default:
			// Could not send because probably the bot context is already cancelled by preceding error notification.
		}
	}

	return botCtx, errNotifier
}

func setupInputReceiver(botCtx context.Context, bot Bot, workerJob chan<- func()) func(Input) error {
	incomingInput := make(chan Input)

	activated := make(chan struct{})
	go func() {
		signalVal := struct{}{} // avoid multiple construction
		for {
			select {
			case activated <- signalVal:
				// Send sentinel value to make sure this goroutine is all ready by the end of this method call.
				// This blocks once the value is sent because of the nature of non-buffered channel and one-time subscription.

			case input := <-incomingInput:
				log.Debugf("responding to %#v", input)

				workerJob <- func() {
					err := bot.Respond(botCtx, input)
					if err != nil {
						log.Errorf("error on message handling. input: %#v. error: %s.", input, err.Error())
					}
				}
				log.Debugf("enqueued %#v", input)

			case <-botCtx.Done():
				log.Infof("stop receiving bot input due to context cancelation: %s.", bot.BotType().String())
				return
			}
		}
	}()
	// Wait til above goroutine is ready.
	// Test shows there is a chance that goroutine is not fully activated right after this method call.
	<-activated

	continuousEnqueueErrCnt := 0
	return func(input Input) error {
		select {
		case incomingInput <- input:
			continuousEnqueueErrCnt = 0
			// Successfully sent without blocking
			return nil
		default:
			continuousEnqueueErrCnt++
			// Could not send because probably the workers are too busy or the bot context is already cancecled.
			return NewBlockedInputError(continuousEnqueueErrCnt)
		}
	}
}
