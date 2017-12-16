package sarah

import (
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
)

// Bot provides interface for each bot implementation.
// Instance of concrete type can be fed to sarah.Runner to have its lifecycle under control.
// Multiple Bot implementation may be registered to single Runner.
type Bot interface {
	// BotType represents what this Bot implements. e.g. slack, gitter, cli, etc...
	// This can be used as a unique ID to distinguish one from another.
	BotType() BotType

	// Respond receives user input, look for corresponding command, execute it, and send result back to user if possible.
	Respond(context.Context, Input) error

	// SendMessage sends message to destination depending on the Bot implementation.
	// This is mainly used to send scheduled task's result.
	// Be advised: this method may be called simultaneously from multiple workers.
	SendMessage(context.Context, Output)

	// AppendCommand appends given Command implementation to Bot internal stash.
	// Stashed commands are checked against user input in Bot.Respond, and if Command.Match returns true, the
	// Command is considered as "corresponds" to the input, hence its Command.Execute is called and the result is
	// sent back to user.
	AppendCommand(Command)

	// Run is called on Runner.Run to let this Bot interact with corresponding service provider.
	// For example, this is where Bot or Bot's corresponding Adapter initiates connection with service provider.
	// This may run in a blocking manner til given context is canceled since a new goroutine is allocated for this task.
	// When the service provider sends message to us, convert that message payload to Input and send to Input channel.
	// Runner will receive the Input instance and proceed to find and execute corresponding command.
	Run(context.Context, func(Input) error, func(error))
}

type defaultBot struct {
	botType            BotType
	runFunc            func(context.Context, func(Input) error, func(error))
	sendMessageFunc    func(context.Context, Output)
	commands           *Commands
	userContextStorage UserContextStorage
}

// NewBot creates and returns new defaultBot instance with given Adapter.
// While Adapter takes care of actual collaboration with each chat service provider,
// defaultBot takes care of some common tasks including:
//   - receive Input
//   - see if sending user is in the middle of conversational context
//     - if so, execute the next step with given Input
//     - if not, find corresponding Command for given Input and execute it
//   - call Adapter.SendMessage to send output
// The aim of defaultBot is to lessen the tasks of Adapter developer by providing some common tasks' implementations, and achieve easier creation of Bot implementation.
// Hence this method returns Bot interface instead of any concrete instance so this can be ONLY treated as Bot implementation to be fed to Runner.RegisterBot.
//
// Some optional settings can be supplied by passing sarah.WithStorage and others that return DefaultBotOption.
//
//  // Use pre-defined storage.
//  storage := sarah.NewUserContextStorage(sarah.NewCacheConfig())
//  bot, err := sarah.NewBot(myAdapter, sarah.WithStorage(sarah.NewUserContextStorage(sarah.NewCacheConfig())))
//
// It is highly recommended to provide concrete implementation of sarah.UserContextStorage, so the users' conversational context can be stored and executed on next Input.
// sarah.userContextStorage is provided by default to store user context in memory. This storage can be initialized by sarah.NewUserContextStorage like above example.
func NewBot(adapter Adapter, options ...DefaultBotOption) (Bot, error) {
	bot := &defaultBot{
		botType:            adapter.BotType(),
		runFunc:            adapter.Run,
		sendMessageFunc:    adapter.SendMessage,
		commands:           NewCommands(),
		userContextStorage: nil,
	}

	for _, opt := range options {
		err := opt(bot)
		if err != nil {
			return nil, err
		}
	}

	return bot, nil
}

// DefaultBotOption defines function that defaultBot's functional option must satisfy.
type DefaultBotOption func(bot *defaultBot) error

// BotWithStorage creates and returns DefaultBotOption to set preferred UserContextStorage implementation.
// Below example utilizes pre-defined in-memory storage.
//
//  config := sarah.NewCacheConfig()
//  configBuf, _ := ioutil.ReadFile("/path/to/storage/config.yaml")
//  yaml.Unmarshal(configBuf, config)
//  bot, err := sarah.NewBot(myAdapter, storage)
func BotWithStorage(storage UserContextStorage) DefaultBotOption {
	return func(bot *defaultBot) error {
		bot.userContextStorage = storage
		return nil
	}
}

func (bot *defaultBot) BotType() BotType {
	return bot.botType
}

func (bot *defaultBot) Respond(ctx context.Context, input Input) error {
	senderKey := input.SenderKey()

	// See if any conversational context is stored.
	var nextFunc ContextualFunc
	if bot.userContextStorage != nil {
		var storageErr error
		nextFunc, storageErr = bot.userContextStorage.Get(senderKey)
		if storageErr != nil {
			return storageErr
		}
	}

	var res *CommandResponse
	var err error
	if nextFunc == nil {
		// If no conversational context is stored, simply search for corresponding command.
		switch input.(type) {
		case *HelpInput:
			res = &CommandResponse{
				Content:     bot.commands.Helps(),
				UserContext: nil,
			}
		default:
			res, err = bot.commands.ExecuteFirstMatched(ctx, input)
		}
	} else {
		bot.userContextStorage.Delete(senderKey)
		switch input.(type) {
		case *AbortInput:
			return nil
		default:
			res, err = nextFunc(ctx, input)
		}
	}

	if err != nil {
		return err
	}

	if res == nil {
		return nil
	}

	// https://github.com/oklahomer/go-sarah/issues/7
	// Bot may return no message to client and still keep the client in the middle of conversational context.
	// This may damage user experience since user is left in conversational context set by CommandResponse without any sort of notification.
	if res.UserContext != nil && bot.userContextStorage != nil {
		if err := bot.userContextStorage.Set(senderKey, res.UserContext); err != nil {
			log.Errorf("Failed to store UserContext. BotType: %s. SenderKey: %s. UserContext: %#v.", bot.BotType(), senderKey, res.UserContext)
		}
	}
	if res.Content != nil {
		message := NewOutputMessage(input.ReplyTo(), res.Content)
		bot.SendMessage(ctx, message)
	}

	return nil
}

func (bot *defaultBot) SendMessage(ctx context.Context, output Output) {
	bot.sendMessageFunc(ctx, output)
}

func (bot *defaultBot) AppendCommand(command Command) {
	bot.commands.Append(command)
}

func (bot *defaultBot) Run(ctx context.Context, enqueueInput func(Input) error, notifyErr func(error)) {
	bot.runFunc(ctx, enqueueInput, notifyErr)
}

// NewSuppressedResponseWithNext creates new sarah.CommandResponse instance with no message and next function to continue
func NewSuppressedResponseWithNext(next ContextualFunc) *CommandResponse {
	return &CommandResponse{
		Content:     nil,
		UserContext: NewUserContext(next),
	}
}
