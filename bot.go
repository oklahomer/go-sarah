package sarah

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/oklahomer/go-sarah/worker"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
	"time"
)

var (
	stashedCommandBuilder       = map[BotType][]*commandBuilder{}
	stashedScheduledTaskBuilder = map[BotType][]*scheduledTaskBuilder{}
)

// BotType indicates what bot implementation a particular BotAdapter/Plugin is corresponding to.
type BotType string

// String returns a stringified form of BotType
func (botType BotType) String() string {
	return string(botType)
}

/*
BotAdapter defines interface that each Bot implementation has to satisfy.
Its instance can be fed to Bot to start bot interaction.
*/
type BotAdapter interface {
	BotType() BotType
	Run(context.Context, chan<- BotInput, chan<- error)
	SendMessage(BotOutput)
}

/*
botProperty stashes some properties for each bot implementation.

Since each bot implementation, BotAdapter, is not responsible for implementing and storing its commands -- this is managed by Bot --
Bot needs to internally store each BotAdapter, corresponding Commands, and miscellaneous properties/attributes at one place.
This is to increase Bot's implementation handiness, so this struct is never meant to be exposed.
*/
type botProperty struct {
	adapter         BotAdapter
	commands        *Commands
	pluginConfigDir string
	cron            *cron.Cron
}

/*
newBotProperty creates and return new botProperty to store each bot implementation.
*/
func newBotProperty(adapter BotAdapter, configDir string) *botProperty {
	return &botProperty{
		adapter:         adapter,
		commands:        NewCommands(),
		pluginConfigDir: configDir,
		cron:            cron.New(),
	}
}

/*
BotRunner is the core of sarah.

This takes care of lifecycle of each bot implementation, internal job worker, and plugin execution;
BotAdapter is responsible for bot-specific implementation such as connection handling, message reception and sending.

Developers can register desired number of BotAdapter and Commands to create own bot.
*/
type BotRunner struct {
	botProperties []*botProperty
	worker        *worker.Worker
}

// NewBotRunner creates and return new Bot instance.
func NewBotRunner() *BotRunner {
	return &BotRunner{
		botProperties: []*botProperty{},
		worker:        worker.New(),
	}
}

/*
AddAdapter allows developer to register desired BotAdapter implementation.
Bot and each adapter mainly communicate via designated channels to pass incoming message and outgoing response.
*/
func (runner *BotRunner) AddAdapter(adapter BotAdapter, pluginConfigDir string) {
	for _, botProperty := range runner.botProperties {
		if botProperty.adapter.BotType() == adapter.BotType() {
			panic(fmt.Sprintf("BotType (%s) conflicted with stored BotAdapter.", adapter.BotType()))
		}
	}

	// New adapter. Append to stored ones.
	runner.botProperties = append(runner.botProperties, newBotProperty(adapter, pluginConfigDir))
}

/*
Run starts Bot interaction.

At this point bot starts its internal workers, runs each BotAdapter, and starts listening to incoming messages.
*/
func (runner *BotRunner) Run(ctx context.Context) {
	runner.worker.Run(ctx.Done(), 10)
	for _, botProperty := range runner.botProperties {
		// build commands with stashed builder settings
		if builders, ok := stashedCommandBuilder[botProperty.adapter.BotType()]; ok {
			commands := buildCommands(builders, botProperty.pluginConfigDir)
			for _, command := range commands {
				botProperty.commands.Append(command)
			}
		}

		// build scheduled task with stashed builder settings
		if builders, ok := stashedScheduledTaskBuilder[botProperty.adapter.BotType()]; ok {
			tasks := buildScheduledTasks(builders, botProperty.pluginConfigDir)
			for _, task := range tasks {
				botProperty.cron.AddFunc(task.config.Schedule(), func() {
					res, err := task.Execute()
					if err != nil {
						logrus.Error(fmt.Sprintf("error on scheduled task: %s", task.Identifier))
						return
					}
					message := NewBotOutputMessage(task.config.Destination(), res.Content)
					botProperty.adapter.SendMessage(message)
				})
			}
		}
		botProperty.cron.Start()

		// run BotAdapter
		receiver := make(chan BotInput)
		errCh := make(chan error)
		botAdapterCtx, cancelAdapter := context.WithCancel(ctx)
		go runner.respondMessage(ctx, botProperty, receiver)
		go stopUnrecoverableAdapter(errCh, cancelAdapter)
		botProperty.adapter.Run(botAdapterCtx, receiver, errCh)
	}
}

/*
stopUnrecoverableAdapter receives error from BotAdapter, check if the error is critical, and stop the adapter if required.
*/
func stopUnrecoverableAdapter(errNotifier <-chan error, stopAdapter context.CancelFunc) {
	for {
		err := <-errNotifier
		switch err := err.(type) {
		case *BotAdapterNonContinuableError:
			logrus.Errorf("stop unrecoverable adapter: %s", err.Error())
			stopAdapter()
			return
		}
	}
}

/*
respondMessage listens to incoming messages via channel.

Each BotAdapter enqueues incoming messages to runner's listening channel, and respondMessage receives them.
When corresponding command is found, command is executed and the result can be passed to BotAdapter's SendMessage method.
*/
func (runner *BotRunner) respondMessage(ctx context.Context, botProperty *botProperty, receiver <-chan BotInput) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("stop responding to message due to context cancel")
			return
		case botInput := <-receiver:
			logrus.Debugf("responding to %#v", botInput)
			runner.EnqueueJob(func() {
				res, err := botProperty.commands.ExecuteFirstMatched(botInput)
				if err != nil {
					logrus.Errorf("error on message handling. botInput: %s. error: %#v.", botInput, err.Error())
				}

				if res != nil {
					message := NewBotOutputMessage(botInput.ReplyTo(), res.Content)
					botProperty.adapter.SendMessage(message)
				}
			})
		}
	}
}

// EnqueueJob can be used to enqueue task to BotRunner's internal workers.
func (runner *BotRunner) EnqueueJob(job func()) {
	runner.worker.EnqueueJob(job)
}

/*
AppendCommandBuilder appends given commandBuilder to internal stash.
Stashed builder is used to configure and build Command instance on BotRunner's initialization.
*/
func AppendCommandBuilder(botType BotType, builder *commandBuilder) {
	logrus.Infof("appending command builder for %s. builder %#v.", botType, builder)
	_, ok := stashedCommandBuilder[botType]
	if !ok {
		stashedCommandBuilder[botType] = make([]*commandBuilder, 0)
	}

	stashedCommandBuilder[botType] = append(stashedCommandBuilder[botType], builder)
}

func AppendScheduledTaskBuilder(botType BotType, builder *scheduledTaskBuilder) {
	logrus.Infof("appending scheduled task builder for %s. builder %#v.", botType, builder)
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
			logrus.Errorf(fmt.Sprintf("can't configure plugin: %s. error: %s.", builder.identifier, err.Error()))
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
			logrus.Errorf(fmt.Sprintf("can't configure plugin: %s. error: %s.", builder.identifier, err.Error()))
			continue
		}
		scheduledTasks = append(scheduledTasks, task)
	}

	return scheduledTasks
}

type OutputDestination interface{}

// BotInput defines interface that each incoming message must satisfy.
type BotInput interface {
	Message() string

	SentAt() time.Time

	ReplyTo() OutputDestination
}

// BotOutput defines interface that each outgoing message must satisfy.
type BotOutput interface {
	Destination() OutputDestination
	Content() interface{}
}

type BotOutputMessage struct {
	destination OutputDestination
	content     interface{}
}

func NewBotOutputMessage(destination OutputDestination, content interface{}) BotOutput {
	return &BotOutputMessage{
		destination: destination,
		content:     content,
	}
}

func (output *BotOutputMessage) Destination() OutputDestination {
	return output.destination
}

func (output *BotOutputMessage) Content() interface{} {
	return output.content
}

/*
BotAdapterNonContinuableError represents critical error that BotAdapter can't continue its operation.
When BotRunner receives this, BotRunner must stop corresponding adapter.
*/
type BotAdapterNonContinuableError struct {
	err string
}

/*
Error returns detailed error about BotAdapter's non-continuable state.
*/
func (e BotAdapterNonContinuableError) Error() string {
	return e.err
}

/*
NewBotAdapterNonContinuableError creates and return new BotAdapterNonContinuableError instance.
*/
func NewBotAdapterNonContinuableError(errorContent string) error {
	return &BotAdapterNonContinuableError{err: errorContent}
}
