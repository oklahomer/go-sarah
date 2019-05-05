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

var options = &optionHolder{}

// Config contains some basic configuration variables for go-sarah.
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

// RegisterWatcher registers given watchers.Watcher implementation.
// When this is not called but Config.PluginConfigRoot is still set, Sarah creates watcher with default configuration on sarah.Run().
func RegisterWatcher(watcher watchers.Watcher) {
	options.register(func(r *runner) error {
		r.watcher = watcher
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
		return err
	}

	runner, err := newRunner(ctx, config)
	if err != nil {
		return err
	}
	go runner.run(ctx)

	return nil
}

func newRunner(ctx context.Context, config *Config) (*runner, error) {
	loc, locErr := time.LoadLocation(config.TimeZone)
	if locErr != nil {
		return nil, fmt.Errorf("given timezone can't be converted to time.Location: %s", locErr.Error())
	}

	r := &runner{
		config:             config,
		bots:               []Bot{},
		worker:             nil,
		commandProps:       make(map[BotType][]*CommandProps),
		scheduledTaskProps: make(map[BotType][]*ScheduledTaskProps),
		scheduledTasks:     make(map[BotType][]ScheduledTask),
		alerters:           &alerters{},
		scheduler:          runScheduler(ctx, loc),
	}

	err := options.apply(r)
	if err != nil {
		return nil, fmt.Errorf("failed to apply option: %s", err.Error())
	}

	if r.worker == nil {
		w, e := workers.Run(ctx, workers.NewConfig())
		if e != nil {
			return nil, fmt.Errorf("worker could not run: %s", e.Error())
		}

		r.worker = w
	}

	if r.config.PluginConfigRoot != "" && r.watcher == nil {
		w, e := watchers.Run(ctx)
		if e != nil {
			return nil, fmt.Errorf("watcher could not run: %s", e.Error())
		}

		r.watcher = w
	}

	return r, nil
}

type runner struct {
	config             *Config
	bots               []Bot
	worker             workers.Worker
	watcher            watchers.Watcher
	commandProps       map[BotType][]*CommandProps
	scheduledTaskProps map[BotType][]*ScheduledTaskProps
	scheduledTasks     map[BotType][]ScheduledTask
	alerters           *alerters
	scheduler          scheduler
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

// runBot runs given Bot implementation in a blocking manner.
// This returns when bot stops.
func (r *runner) runBot(runnerCtx context.Context, bot Bot) {
	log.Infof("Starting %s", bot.BotType())
	botCtx, errNotifier := superviseBot(runnerCtx, bot.BotType(), r.alerters)

	// Setup config directory for this particular Bot.
	var configDir string
	if r.config.PluginConfigRoot != "" {
		configDir = filepath.Join(r.config.PluginConfigRoot, strings.ToLower(bot.BotType().String()))
	}

	// Build commands with stashed CommandProps.
	commandProps := r.botCommandProps(bot.BotType())
	registerCommands(bot, commandProps, configDir)

	// Register scheduled tasks.
	tasks := r.botScheduledTasks(bot.BotType())
	taskProps := r.botScheduledTaskProps(bot.BotType())
	registerScheduledTasks(botCtx, bot, tasks, taskProps, r.scheduler, configDir)

	if configDir != "" {
		go r.subscribeConfigDir(botCtx, bot, configDir)
	}

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
	}()
}

// subscribeConfigDir listens to changes of configuration files under configDir.
// When a file is updated, a callback function reads the file content and apply changes to corresponding commands and scheduled tasks.
func (r *runner) subscribeConfigDir(botCtx context.Context, bot Bot, configDir string) {
	callback := func(path string) {
		file, err := plainPathToFile(path)
		if err == errUnableToDetermineConfigFileFormat {
			log.Warnf("File under Config.PluginConfigRoot is updated, but file format can not be determined from its extension: %s.", path)
			return
		} else if err == errUnsupportedConfigFileFormat {
			log.Warnf("File under Config.PluginConfigRoot is updated, but file format is not supported: %s.", path)
			return
		} else if err != nil {
			log.Warnf("Failed to locate %s: %s", path, err.Error())
			return
		}

		// TODO Consider wrapping below function calls with goroutine.
		// A developer may update bunch of files under PluginConfigRoot at once. e.g. rsync all files under the directory.
		// That makes series of callback function calls while each Command/ScheduledTask blocks config file while its execution.
		// See if that block is critical to watcher implementation.
		commandProps := r.botCommandProps(bot.BotType())
		if e := updateCommandConfig(bot, commandProps, file); e != nil {
			log.Errorf("Failed to update Command config: %s.", e.Error())
		}

		taskProps := r.botScheduledTaskProps(bot.BotType())
		if e := updateScheduledTaskConfig(botCtx, bot, taskProps, r.scheduler, file); e != nil {
			log.Errorf("Failed to update ScheduledTask config: %s", e.Error())
		}
	}
	err := r.watcher.Subscribe(bot.BotType().String(), configDir, callback)
	if err != nil {
		log.Errorf("Failed to watch %s: %s", configDir, err.Error())
		return
	}

	// When Bot stops, stop subscription for config file changes.
	<-botCtx.Done()
	err = r.watcher.Unsubscribe(bot.BotType().String())
	if err != nil {
		// Probably because Runner context is canceled, and its derived contexts are canceled simultaneously.
		// In that case this warning is harmless since Watcher itself is canceled at this point.
		log.Warnf("Failed to unsubscribe %s", err.Error())
	}
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
