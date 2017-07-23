package sarah

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/watchers"
	"github.com/oklahomer/go-sarah/workers"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Config contains some configuration variables for Runner.
type Config struct {
	PluginConfigRoot string `json:"plugin_config_root" yaml:"plugin_config_root"`
	TimeZone         string `json:"timezone" yaml:"timezone"`
}

// NewConfig creates and returns new Config instance with default settings.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to override default values.
func NewConfig() *Config {
	return &Config{
		// PluginConfigRoot defines the root directory for each Command and ScheduledTask.
		// File path for each plugin is defined as PluginConfigRoot + "/" + BotType + "/" + (Command|ScheduledTask).Identifier.
		PluginConfigRoot: "",
		TimeZone:         time.Now().Location().String(),
	}
}

// Runner is the core of sarah.
//
// This is responsible for each Bot implementation's lifecycle and plugin execution;
// Bot is responsible for bot-specific implementation such as connection handling, message reception and sending.
//
// Developers can register desired number of Bots and Commands to create own bot experience.
type Runner struct {
	config            *Config
	bots              []Bot
	worker            workers.Worker
	watcher           watchers.Watcher
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
		worker:            nil,
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

// RunnerOption defines a function signature that Runner's functional option must satisfy.
type RunnerOption func(*Runner) error

// RunnerOptions stashes group of RunnerOption for later use with NewRunner().
//
// On typical setup, especially when a process consists of multiple Bots and Commands, each construction step requires more lines of codes.
// Each step ends with creating new RunnerOption instance to be fed to NewRunner(), but as code gets longer it gets harder to keep track of each RunnerOption.
// In that case RunnerOptions becomes a handy helper to temporary stash RunnerOption.
//
//  options := NewRunnerOptions()
//
//  // 5-10 lines of codes to configure Slack bot.
//  slackBot, _ := sarah.NewBot(slack.NewAdapter(slackConfig), sarah.BotWithStorage(storage))
//  options.Append(sarah.WithBot(slackBot))
//
//  // Here comes other 5-10 codes to configure another bot.
//  myBot, _ := NewMyBot(...)
//  optionsAppend(sarah.WithBot(myBot))
//
//  // Some more codes to register Commands/ScheduledTasks.
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
// When more than two RunnerOption instances are stashed, they are executed in the order of addition.
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

// WithBot creates RunnerOption that feeds given Bot implementation to Runner.
func WithBot(bot Bot) RunnerOption {
	return func(runner *Runner) error {
		runner.bots = append(runner.bots, bot)
		return nil
	}
}

// WithCommandProps creates RunnerOption that feeds given CommandProps to Runner.
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

// WithScheduledTaskProps creates RunnerOption that feeds given ScheduledTaskProps to Runner.
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

// WithScheduledTask creates RunnerOperation that feeds given ScheduledTask to Runner.
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

// WithAlerter creates RunnerOperation that feeds given Alerter implementation to Runner.
func WithAlerter(alerter Alerter) RunnerOption {
	return func(runner *Runner) error {
		runner.alerters.appendAlerter(alerter)
		return nil
	}
}

// WithWorker creates RunnerOperation that feeds given Worker implementation to Runner.
// If no WithWorker is supplied, Runner creates worker with default configuration on Runner.Run.
func WithWorker(worker workers.Worker) RunnerOption {
	return func(runner *Runner) error {
		runner.worker = worker
		return nil
	}
}

// WithWatcher creates RunnerOption that feeds given Watcher implementation to Runner.
// If Config.PluginConfigRoot is set without WithWatcher option, Runner creates Watcher with default configuration on Runner.Run.
func WithWatcher(watcher watchers.Watcher) RunnerOption {
	return func(runner *Runner) error {
		runner.watcher = watcher
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
	if runner.worker == nil {
		w, e := workers.Run(ctx, workers.NewConfig())
		if e != nil {
			panic(fmt.Sprintf("worker could not run: %s", e.Error()))
		}

		runner.worker = w
	}

	if runner.config.PluginConfigRoot != "" && runner.watcher == nil {
		w, e := watchers.Run(ctx)
		if e != nil {
			panic(fmt.Sprintf("watcher could not run: %s", e.Error()))
		}

		runner.watcher = w
	}

	loc, locErr := time.LoadLocation(runner.config.TimeZone)
	if locErr != nil {
		panic(fmt.Sprintf("given timezone can't be converted to time.Location: %s", locErr.Error()))
	}
	taskScheduler := runScheduler(ctx, loc)

	var wg sync.WaitGroup
	for _, bot := range runner.bots {
		wg.Add(1)

		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// Each Bot has its own context propagating Runner's lifecycle.
		botCtx, errNotifier := botSupervisor(ctx, botType, runner.alerters)

		// Prepare function that receives Input.
		receiveInput := setupInputReceiver(botCtx, bot, runner.worker)

		// Run Bot
		go runBot(botCtx, bot, receiveInput, errNotifier)

		// Setup config directory.
		var configDir string
		if runner.config.PluginConfigRoot != "" {
			configDir = filepath.Join(runner.config.PluginConfigRoot, strings.ToLower(bot.BotType().String()))
		}

		// Build commands with stashed CommandProps.
		commandProps := runner.botCommandProps(botType)
		registerCommands(bot, commandProps, configDir)

		// Register scheduled tasks.
		tasks := runner.botScheduledTasks(botType)
		taskProps := runner.botScheduledTaskProps(botType)
		registerScheduledTasks(botCtx, bot, tasks, taskProps, taskScheduler, configDir)

		// Supervise configuration files' directory for Command/ScheduledTask.
		if configDir != "" {
			callback := runner.configUpdateCallback(botCtx, bot, taskScheduler)
			err := runner.watcher.Subscribe(botType.String(), configDir, callback)
			if err != nil {
				log.Errorf("Failed to watch %s: %s", configDir, err.Error())
			}
		}

		go func(c context.Context, b Bot) {
			select {
			case <-c.Done():
				wg.Done()

				// When Bot stops, stop subscription for config file changes.
				err := runner.watcher.Unsubscribe(b.BotType().String())
				if err != nil {
					// Probably because Runner context is canceled, and its derived contexts are canceled simultaneously.
					// In that case this warning is harmless since Watcher itself is canceled at this point.
					log.Warnf("Failed to unsubscribe %s", err.Error())
				}
			}
		}(botCtx, bot)
	}

	wg.Wait()
}

func (runner *Runner) configUpdateCallback(botCtx context.Context, bot Bot, taskScheduler scheduler) func(string) {
	return func(path string) {
		file, err := plainPathToFile(path)
		if err == errUnableToDetermineConfigFileFormat {
			log.Warnf("File under Config.PluginConfigRoot is updated, but file format can not be determined from its extension: %s.", path)
			return
		} else if err == errUnableToDetermineConfigFileFormat {
			log.Warnf("File under Config.PluginConfigRoot is updated, but file format is not supported: %s.", path)
			return
		} else if err != nil {
			log.Warnf("Failed to locate %s: %s", path, err.Error())
			return
		}

		// TODO Consider wrapping below function calls with goroutine.
		// Developer may update bunch of files under PluginConfigRoot at once. e.g. rsync whole all files under the directory.
		// That makes series of callback function calls while each Command/ScheduledTask blocks config file while its execution.
		// See if that block is critical to watcher implementation.
		commandProps := runner.botCommandProps(bot.BotType())
		if e := updateCommandConfig(bot, commandProps, file); e != nil {
			log.Errorf("Failed to update Command config: %s.", e.Error())
		}

		taskProps := runner.botScheduledTaskProps(bot.BotType())
		if e := updateScheduledTaskConfig(botCtx, bot, taskProps, taskScheduler, file); e != nil {
			log.Errorf("Failed to update ScheduledTask config: %s", e.Error())
		}
	}
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

func registerScheduledTask(botCtx context.Context, bot Bot, task ScheduledTask, taskScheduler scheduler) {
	err := taskScheduler.update(bot.BotType(), task, func() {
		executeScheduledTask(botCtx, bot, task)
	})
	if err != nil {
		log.Errorf("failed to schedule a task. id: %s. reason: %s.", task.Identifier(), err.Error())
	}
}

func registerCommands(bot Bot, props []*CommandProps, configDir string) {
	for _, p := range props {
		var file *pluginConfigFile
		if configDir != "" && p.config != nil {
			file = findPluginConfigFile(configDir, p.identifier)
		}

		command, err := buildCommand(p, file)
		if err != nil {
			log.Errorf("Failed to configure command. %s. %#v", err.Error(), p)
			continue
		}
		bot.AppendCommand(command)
	}
}

func registerScheduledTasks(botCtx context.Context, bot Bot, tasks []ScheduledTask, props []*ScheduledTaskProps, taskScheduler scheduler, configDir string) {
	for _, p := range props {
		var file *pluginConfigFile
		if configDir != "" && p.config != nil {
			file = findPluginConfigFile(configDir, p.identifier)
		}

		task, err := buildScheduledTask(p, file)
		if err != nil {
			log.Errorf("Failed to configure scheduled task: %s. %#v.", err.Error(), p)
			continue
		}
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		// Make sure schedule is given. Especially those pre-registered tasks.
		if task.Schedule() == "" {
			log.Errorf("Failed to schedule a task. id: %s. reason: %s.", task.Identifier(), "No schedule given.")
			continue
		}

		registerScheduledTask(botCtx, bot, task, taskScheduler)
	}
}

func updateCommandConfig(bot Bot, props []*CommandProps, file *pluginConfigFile) error {
	for _, p := range props {
		if p.config == nil {
			continue
		}

		if p.identifier != file.id {
			continue
		}

		log.Infof("Start updating config due to config file change: %s.", file.id)
		rv := reflect.ValueOf(p.config)
		if rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Map {
			// p.config is a pointer to config struct or a map
			// Just update p.config and the same instance is shared by currently registered Command.
			err := func() error {
				// https://github.com/oklahomer/go-sarah/issues/44
				locker := configLocker.get(p.botType, p.identifier)
				locker.Lock()
				defer locker.Unlock()

				return updatePluginConfig(file, p.config)
			}()
			if err != nil {
				return fmt.Errorf("failed to update config for %s: %s", p.identifier, err.Error())
			}
		} else {
			// p.config is not pointer or map, but an actual struct value.
			// Simply updating p.config can not update the behaviour of currently registered Command.
			// Proceed to re-build Command and replace with old one.
			rebuiltCmd, err := buildCommand(p, file)
			if err != nil {
				return fmt.Errorf("failed to rebuild Command for %s: %s", p.identifier, err.Error())
			}
			bot.AppendCommand(rebuiltCmd) // Replaces the old one.
		}
		log.Infof("End updating config due to config file change: %s.", file.id)

		return nil
	}

	return nil
}

func updateScheduledTaskConfig(botCtx context.Context, bot Bot, taskProps []*ScheduledTaskProps, taskScheduler scheduler, file *pluginConfigFile) error {
	for _, p := range taskProps {
		if p.config == nil {
			continue
		}

		if p.identifier != file.id {
			continue
		}

		// TaskConfig update may involve re-scheduling and other miscellaneous tasks
		// so no matter config type is a pointer or actual value, always re-build ScheduledTask and register again.
		// See updateCommandConfig.
		log.Infof("Start rebuilding scheduled task due to config file change: %s.", file.id)
		task, err := buildScheduledTask(p, file)
		if err != nil {
			// When rebuild is failed, unregister corresponding ScheduledTask.
			// This is to avoid a scenario that config struct itself is successfully updated,
			// but the props' values and new config do not provide some required settings such as schedule, etc...
			e := taskScheduler.remove(bot.BotType(), p.identifier)
			if e != nil {
				return fmt.Errorf("tried to remove ScheduledTask because rebuild failed, but removal also failed: %s", e.Error())
			} else {
				return fmt.Errorf("failed to re-build scheduled task id: %s error: %s", p.identifier, err.Error())
			}
		}

		registerScheduledTask(botCtx, bot, task, taskScheduler)

		log.Infof("End rebuilding scheduled task due to config file change: %s.", file.id)

		return nil
	}

	return nil
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
			// Useful when destination can be preset.
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
	// Bot itself MUST NOT kill itself, but the Runner does. Beware that Runner takes care of all related components' lifecycle.
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
					// Call to cancel() causes Bot context cancellation, and hence below botCtx.Done case works.
					// Until then let this case statement listen to other errors during Bot stopping stage, so that desired logging may work.
				}

			case <-botCtx.Done():
				// The context.CancelFunc is locally stored in this goroutine and is completely under control,
				// but botCtx can also be cancelled by upper level context, runner context.
				// So be sure to subscribe to botCtx.Done().
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
		// Try notifying critical error state, but give up if the corresponding Bot is already stopped or is being stopped.
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

func setupInputReceiver(botCtx context.Context, bot Bot, worker workers.Worker) func(Input) error {
	continuousEnqueueErrCnt := 0
	return func(input Input) error {
		err := worker.Enqueue(func() {
			err := bot.Respond(botCtx, input)
			if err != nil {
				log.Errorf("error on message handling. input: %#v. error: %s.", input, err.Error())
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

type fileType uint

const (
	_ fileType = iota
	yaml_file
	json_file
)

type pluginConfigFile struct {
	id       string
	path     string
	fileType fileType
}

func updatePluginConfig(file *pluginConfigFile, configPtr interface{}) error {
	buf, err := ioutil.ReadFile(file.path)
	if err != nil {
		return err
	}

	switch file.fileType {
	case yaml_file:
		return yaml.Unmarshal(buf, configPtr)

	case json_file:
		return json.Unmarshal(buf, configPtr)

	default:
		return fmt.Errorf("Unsupported file type: %s.", file.path)

	}
}

var (
	errUnableToDetermineConfigFileFormat = errors.New("can not determine file format")
	errUnsupportedConfigFileFormat       = errors.New("unsupported file format")
)

func plainPathToFile(path string) (*pluginConfigFile, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(path)
	if ext == "" {
		return nil, errUnableToDetermineConfigFileFormat
	}

	candidates := []struct {
		ext      string
		fileType fileType
	}{
		{
			ext:      ".yaml",
			fileType: yaml_file,
		},
		{
			ext:      ".yml",
			fileType: yaml_file,
		},
		{
			ext:      ".json",
			fileType: json_file,
		},
	}

	_, filename := filepath.Split(path)
	id := strings.TrimSuffix(filename, ext) // buzz.yaml to buzz

	for _, c := range candidates {
		if ext != c.ext {
			continue
		}

		return &pluginConfigFile{
			id:       id,
			path:     path,
			fileType: c.fileType,
		}, nil
	}

	return nil, errUnsupportedConfigFileFormat
}

func findPluginConfigFile(configDir, id string) *pluginConfigFile {
	candidates := []struct {
		ext      string
		fileType fileType
	}{
		{
			ext:      ".yaml",
			fileType: yaml_file,
		},
		{
			ext:      ".yml",
			fileType: yaml_file,
		},
		{
			ext:      ".json",
			fileType: json_file,
		},
	}

	for _, c := range candidates {
		configPath := filepath.Join(configDir, fmt.Sprintf("%s%s", id, c.ext))
		_, err := os.Stat(configPath)
		if err == nil {
			// File exists.
			return &pluginConfigFile{
				id:       id,
				path:     configPath,
				fileType: c.fileType,
			}
		}
	}

	return nil
}
