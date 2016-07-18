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

type BotType string

func (botType BotType) String() string {
	return string(botType)
}

type BotAdapter interface {
	GetPluginConfigDir() string
	GetBotType() BotType
	Run(chan<- BotInput)
	SendResponse(*CommandResponse)
	Stop()
}

type Bot struct {
	adapters   map[BotType]BotAdapter
	commands   map[BotType]*Commands
	workerPool *worker.Pool
	stopAll    chan bool
}

func NewBot() *Bot {
	return &Bot{
		adapters:   map[BotType]BotAdapter{},
		commands:   map[BotType]*Commands{},
		workerPool: worker.NewPool(10),
		stopAll:    make(chan bool),
	}
}

func (bot *Bot) AppendAdapter(adapter BotAdapter) {
	botType := adapter.GetBotType()
	bot.adapters[botType] = adapter
	bot.commands[botType] = NewCommands()
}

func (bot *Bot) Run() {
	go bot.runWorkers()
	for botType := range bot.adapters {
		bot.ConfigureCommands(botType)
		receiver := make(chan BotInput)
		bot.adapters[botType].Run(receiver)
		go bot.respondMessage(botType, receiver)
	}
}

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

func (bot *Bot) Stop() {
	close(bot.stopAll)
	for botType := range bot.adapters {
		bot.adapters[botType].Stop()
	}
}

func (bot *Bot) runWorkers() {
	bot.workerPool.Run()
	defer bot.workerPool.Stop()

	<-bot.stopAll
}

func (bot *Bot) EnqueueJob(job func()) {
	bot.workerPool.EnqueueJob(job)
}

/*
AppendCommandBuilder appends given commandBuilder to Bot's internal stash.
Stashed builder is used to configure and build command instance on Bot's initialization.
*/
func AppendCommandBuilder(botType BotType, builder *commandBuilder) {
	logrus.Infof("appending command builder for %s. builder %#v.", botType, builder)
	_, ok := stashedCommandBuilder[botType]
	if !ok {
		stashedCommandBuilder[botType] = make([]*commandBuilder, 0)
	}

	stashedCommandBuilder[botType] = append(stashedCommandBuilder[botType], builder)
}

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

type BotInput interface {
	GetSenderID() string

	GetMessage() string

	GetSentAt() time.Time

	GetRoomID() string
}
