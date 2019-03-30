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
// While developers may provide own implementation for interfaces in this project to customize behavior,
// this particular interface is not meant to be implemented and replaced.
// See https://github.com/oklahomer/go-sarah/pull/47
type Runner interface {
	// Run starts Bot interaction.
	// At this point Runner starts its internal workers and schedulers, runs each bot, and starts listening to incoming messages.
	Run(context.Context)

	// Status returns the status of Runner and belonging Bots.
	// The returned Status value represents a snapshot of the status when this method is called,
	// which means each field value is not subject to update.
	// To reflect the latest status, this is recommended to call this method whenever the value is needed.
	Status() Status
}

type runner struct {
	config            *Config
	bots              []Bot
	worker            workers.Worker
	watcher           watchers.Watcher
	commandProps      map[BotType][]*CommandProps
	scheduledTaskPrps map[BotType][]*ScheduledTaskProps
	scheduledTasks    map[BotType][]ScheduledTask
	alerters          *alerters
	status            *status
}

// NewRunner creates and return new instance that satisfies Runner interface.
//
// The reason for returning interface instead of concrete implementation
// is to avoid developers from executing RunnerOption outside of NewRunner,
// where sarah can not be aware of and severe side-effect may occur.
//
// Ref. https://github.com/oklahomer/go-sarah/pull/47
//
// So the aim is not to let developers switch its implementations.
func NewRunner(config *Config, options ...RunnerOption) (Runner, error) {
	r := &runner{
		config:            config,
		bots:              []Bot{},
		worker:            nil,
		commandProps:      make(map[BotType][]*CommandProps),
		scheduledTaskPrps: make(map[BotType][]*ScheduledTaskProps),
		scheduledTasks:    make(map[BotType][]ScheduledTask),
		alerters:          &alerters{},
		status:            &status{},
	}

	for _, opt := range options {
		err := opt(r)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// RunnerOption defines a function signature that NewRunner's functional option must satisfy.
type RunnerOption func(*runner) error

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
	return func(r *runner) error {
		for _, opt := range *options {
			err := opt(r)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithBot creates RunnerOption that feeds given Bot implementation to Runner.
func WithBot(bot Bot) RunnerOption {
	return func(r *runner) error {
		r.bots = append(r.bots, bot)
		return nil
	}
}

// WithCommandProps creates RunnerOption that feeds given CommandProps to Runner.
// Command is built on runner.Run with given CommandProps.
// This props is re-used when configuration file is updated and Command needs to be re-built.
func WithCommandProps(props *CommandProps) RunnerOption {
	return func(r *runner) error {
		stashed, ok := r.commandProps[props.botType]
		if !ok {
			stashed = []*CommandProps{}
		}
		r.commandProps[props.botType] = append(stashed, props)
		return nil
	}
}

// WithScheduledTaskProps creates RunnerOption that feeds given ScheduledTaskProps to Runner.
// ScheduledTask is built on runner.Run with given ScheduledTaskProps.
// This props is re-used when configuration file is updated and ScheduledTask needs to be re-built.
func WithScheduledTaskProps(props *ScheduledTaskProps) RunnerOption {
	return func(r *runner) error {
		stashed, ok := r.scheduledTaskPrps[props.botType]
		if !ok {
			stashed = []*ScheduledTaskProps{}
		}
		r.scheduledTaskPrps[props.botType] = append(stashed, props)
		return nil
	}
}

// WithScheduledTask creates RunnerOperation that feeds given ScheduledTask to Runner.
func WithScheduledTask(botType BotType, task ScheduledTask) RunnerOption {
	return func(r *runner) error {
		tasks, ok := r.scheduledTasks[botType]
		if !ok {
			tasks = []ScheduledTask{}
		}
		r.scheduledTasks[botType] = append(tasks, task)
		return nil
	}
}

// WithAlerter creates RunnerOperation that feeds given Alerter implementation to Runner.
func WithAlerter(alerter Alerter) RunnerOption {
	return func(r *runner) error {
		r.alerters.appendAlerter(alerter)
		return nil
	}
}

// WithWorker creates RunnerOperation that feeds given Worker implementation to Runner.
// If no WithWorker is supplied, Runner creates worker with default configuration on runner.Run.
func WithWorker(worker workers.Worker) RunnerOption {
	return func(r *runner) error {
		r.worker = worker
		return nil
	}
}

// WithWatcher creates RunnerOption that feeds given Watcher implementation to Runner.
// If Config.PluginConfigRoot is set without WithWatcher option, Runner creates Watcher with default configuration on Runner.Run.
func WithWatcher(watcher watchers.Watcher) RunnerOption {
	return func(r *runner) error {
		r.watcher = watcher
		return nil
	}
}

func (r *runner) botCommandProps(botType BotType) []*CommandProps {
	if props, ok := r.commandProps[botType]; ok {
		return props
	}
	return []*CommandProps{}
}

func (r *runner) botScheduledTaskProps(botType BotType) []*ScheduledTaskProps {
	if props, ok := r.scheduledTaskPrps[botType]; ok {
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

func (r *runner) Status() Status {
	return r.status.snapshot()
}

func (r *runner) Run(ctx context.Context) {
	r.status.start()

	if r.worker == nil {
		w, e := workers.Run(ctx, workers.NewConfig())
		if e != nil {
			panic(fmt.Sprintf("worker could not run: %s", e.Error()))
		}

		r.worker = w
	}

	if r.config.PluginConfigRoot != "" && r.watcher == nil {
		w, e := watchers.Run(ctx)
		if e != nil {
			panic(fmt.Sprintf("watcher could not run: %s", e.Error()))
		}

		r.watcher = w
	}

	loc, locErr := time.LoadLocation(r.config.TimeZone)
	if locErr != nil {
		panic(fmt.Sprintf("given timezone can't be converted to time.Location: %s", locErr.Error()))
	}
	taskScheduler := runScheduler(ctx, loc)

	var wg sync.WaitGroup
	for _, bot := range r.bots {
		wg.Add(1)

		botType := bot.BotType()
		log.Infof("starting %s", botType.String())

		// Each Bot has its own context propagating Runner's lifecycle.
		botCtx, errNotifier := superviseBot(ctx, botType, r.alerters)

		// Prepare function that receives Input.
		receiveInput := setupInputReceiver(botCtx, bot, r.worker)

		// Run Bot
		go runBot(botCtx, bot, receiveInput, errNotifier)
		r.status.addBot(bot)

		// Setup config directory.
		var configDir string
		if r.config.PluginConfigRoot != "" {
			configDir = filepath.Join(r.config.PluginConfigRoot, strings.ToLower(bot.BotType().String()))
		}

		// Build commands with stashed CommandProps.
		commandProps := r.botCommandProps(botType)
		registerCommands(bot, commandProps, configDir)

		// Register scheduled tasks.
		tasks := r.botScheduledTasks(botType)
		taskProps := r.botScheduledTaskProps(botType)
		registerScheduledTasks(botCtx, bot, tasks, taskProps, taskScheduler, configDir)

		// Supervise configuration files' directory for Command/ScheduledTask.
		if configDir != "" {
			callback := r.configUpdateCallback(botCtx, bot, taskScheduler)
			err := r.watcher.Subscribe(botType.String(), configDir, callback)
			if err != nil {
				log.Errorf("Failed to watch %s: %s", configDir, err.Error())
			}
		}

		go func(c context.Context, b Bot, d string) {
			select {
			case <-c.Done():
				defer wg.Done()

				// When Bot stops, stop subscription for config file changes.
				if d != "" {
					err := r.watcher.Unsubscribe(b.BotType().String())
					if err != nil {
						// Probably because Runner context is canceled, and its derived contexts are canceled simultaneously.
						// In that case this warning is harmless since Watcher itself is canceled at this point.
						log.Warnf("Failed to unsubscribe %s", err.Error())
					}
				}

				r.status.stopBot(b)
			}
		}(botCtx, bot, configDir)
	}

	wg.Wait()
	r.status.stop()
}

func (r *runner) configUpdateCallback(botCtx context.Context, bot Bot, taskScheduler scheduler) func(string) {
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
		commandProps := r.botCommandProps(bot.BotType())
		if e := updateCommandConfig(bot, commandProps, file); e != nil {
			log.Errorf("Failed to update Command config: %s.", e.Error())
		}

		taskProps := r.botScheduledTaskProps(bot.BotType())
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
			}

			return fmt.Errorf("failed to re-build scheduled task id: %s error: %s", p.identifier, err.Error())
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

func superviseBot(runnerCtx context.Context, botType BotType, alerters *alerters) (context.Context, func(error)) {
	botCtx, cancel := context.WithCancel(runnerCtx)

	// A function that receives an escalated error from Bot.
	// If critical error is sent, this cancels Bot context to finish its lifecycle.
	// Bot itself MUST NOT kill itself, but the Runner does. Beware that Runner takes care of all related components' lifecycle.
	handleError := func(err error) {
		switch err.(type) {
		case *BotNonContinuableError:
			log.Errorf("stop unrecoverable bot. BotType: %s. error: %s.", botType.String(), err.Error())
			cancel()

			go func() {
				e := alerters.alertAll(runnerCtx, botType, err)
				if e != nil {
					log.Errorf("failed to send alert for %s: %s", botType.String(), e.Error())
				}
			}()

			log.Infof("stop supervising bot critical error due to context cancellation: %s.", botType.String())

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
	yamlFile
	jsonFile
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
	case yamlFile:
		return yaml.Unmarshal(buf, configPtr)

	case jsonFile:
		return json.Unmarshal(buf, configPtr)

	default:
		return fmt.Errorf("unsupported file type: %s", file.path)

	}
}

var (
	errUnableToDetermineConfigFileFormat = errors.New("can not determine file format")
	errUnsupportedConfigFileFormat       = errors.New("unsupported file format")
	configFileCandidates                 = []struct {
		ext      string
		fileType fileType
	}{
		{
			ext:      ".yaml",
			fileType: yamlFile,
		},
		{
			ext:      ".yml",
			fileType: yamlFile,
		},
		{
			ext:      ".json",
			fileType: jsonFile,
		},
	}
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

	_, filename := filepath.Split(path)
	id := strings.TrimSuffix(filename, ext) // buzz.yaml to buzz

	for _, c := range configFileCandidates {
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
	for _, c := range configFileCandidates {
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
