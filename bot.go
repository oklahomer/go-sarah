package sarah

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/oklahomer/go-sarah/worker"
	"time"
)

var (
	stashedCommandBuilder = map[BotType][]*commandBuilder{}
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
	GetPluginConfigDir() string
	GetBotType() BotType
	Run(chan<- BotInput)
	SendResponse(*CommandResponse)
	Stop()
}

/*
Bot is the core of sarah.
Developers can register desired BotAdapter and Commands to create own bot.
*/
type Bot struct {
	adapters   map[BotType]BotAdapter
	commands   map[BotType]*Commands
	workerPool *worker.Pool
	stopAll    chan bool
}

// NewBot creates and return new Bot instance.
func NewBot() *Bot {
	return &Bot{
		adapters:   map[BotType]BotAdapter{},
		commands:   map[BotType]*Commands{},
		workerPool: worker.NewPool(10),
		stopAll:    make(chan bool),
	}
}

/*
AddAdapter allows developer to register desired BotAdapter implementation.
Bot and each adapter mainly communicate via designated channels to pass incoming message and outgoing response.
*/
func (bot *Bot) AddAdapter(adapter BotAdapter) {
	botType := adapter.GetBotType()
	bot.adapters[botType] = adapter
	bot.commands[botType] = NewCommands()
}

/*
Run starts Bot interaction.

At this point bot starts its internal workers, runs each BotAdapter, and starts listening to incoming messages.
*/
func (bot *Bot) Run() {
	go bot.runWorkers()
	for botType := range bot.adapters {
		bot.ConfigureCommands(botType)
		receiver := make(chan BotInput)
		bot.adapters[botType].Run(receiver)
		go bot.respondMessage(botType, receiver)
	}
}

/*
respondMessage listens to incoming messages via channel.

Each BotAdapter enqueues incoming messages to Bot's listening channel, and respondMessage receives them.
When corresponding command is found, command is executed and the result can be passed to BotAdapter's SendResponse method.
*/
func (bot *Bot) respondMessage(botType BotType, receiver <-chan BotInput) {
	for {
		select {
		case <-bot.stopAll:
			return
		case botInput := <-receiver:
			logrus.Debugf("responding to %#v", botInput)
			bot.EnqueueJob(func() {
				res, err := bot.commands[botType].ExecuteFirstMatched(botInput)
				if err != nil {
					logrus.Errorf("error on message handling. botType: %s. error: %#v.", botInput, err.Error())
				}

				if res != nil {
					bot.adapters[botType].SendResponse(res)
				}
			})
		}
	}
}

// Stop can be called to stop all bot interaction including each BotAdapter.
func (bot *Bot) Stop() {
	close(bot.stopAll)
	for botType := range bot.adapters {
		bot.adapters[botType].Stop()
	}
}

// runWorkers starts Bot internal workers.
func (bot *Bot) runWorkers() {
	bot.workerPool.Run()
	defer bot.workerPool.Stop()

	<-bot.stopAll
}

// EnqueueJob can be used to enqueue task to Bot internal workers.
func (bot *Bot) EnqueueJob(job func()) {
	bot.workerPool.EnqueueJob(job)
}

/*
AppendCommandBuilder appends given commandBuilder to Bot's internal stash.
Stashed builder is used to configure and build Command instance on Bot's initialization.
*/
func AppendCommandBuilder(botType BotType, builder *commandBuilder) {
	logrus.Infof("appending command builder for %s. builder %#v.", botType, builder)
	_, ok := stashedCommandBuilder[botType]
	if !ok {
		stashedCommandBuilder[botType] = make([]*commandBuilder, 0)
	}

	stashedCommandBuilder[botType] = append(stashedCommandBuilder[botType], builder)
}

/*
ConfigureCommands configures and creates Command instances for given BotType.
*/
func (bot *Bot) ConfigureCommands(botType BotType) {
	adapter := bot.adapters[botType]
	for _, builder := range stashedCommandBuilder[botType] {
		command, err := builder.build(adapter.GetPluginConfigDir())
		if err != nil {
			logrus.Errorf(fmt.Sprintf("can't configure plugin: %s. error: %s.", builder.Identifier, err.Error()))
			continue
		}
		bot.commands[botType].Append(command)
	}
}

// BotInput defines interface that each incoming message must satisfy.
type BotInput interface {
	GetSenderID() string

	GetMessage() string

	GetSentAt() time.Time

	GetRoomID() string
}
