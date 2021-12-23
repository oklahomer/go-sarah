package sarah

import (
	"context"
	"github.com/oklahomer/go-kasumi/logger"
)

// Bot defines an interface that each interacting bot must satisfy.
// Its implementation can be registered to Sarah with RegisterBot, and the lifecycle will be managed by Sarah.
// Multiple Bot implementations can be registered by multiple RegisterBot calls.
type Bot interface {
	// BotType returns a BotType this Bot implementation represents,
	// which can be used as a unique ID to distinguish one Bot implementation from another.
	BotType() BotType

	// Respond receives a user input, executes a "task" against it, and sends the result back to the user when necessary.
	// A task can be one of the below depending on the user's current state:
	//   - When the user is in the middle of stateful command execution, the given input is treated as part of the instruction given in a user-interactive manner.
	//     - Respond executes a function tied to the current user state and sends the result back to the user.
	//   - When no user context is stored, then the input is treated as a brand new command execution.
	//     - Respond finds a Command, executes it, and sends the result back to the user.
	// In either way, a new user state is set to the storage when the task's result tells to do so.
	Respond(context.Context, Input) error

	// SendMessage sends a given message to the destination depending on the Bot implementation.
	// This is mainly used to send scheduled task's result.
	SendMessage(context.Context, Output)

	// AppendCommand receives a Command to be registered to this Bot implementation.
	// The Bot implementation must append the given Command to its internal stash so the corresponding Command can be found when the user gives an Input.
	//
	// Stashed commands are checked against user input in Bot.Respond.
	// If Command.Match returns true, the Command is considered to "correspond" to the given Input.
	// Then the corresponding Command's Command.Execute is called and the result is sent back to the user.
	//
	// A developer may call this method by oneself to register a Command.
	// Or one can register a Command with sarah package's public function, RegisterCommand.
	// The use of RegisterCommand would be easier because one does not have to carry around the reference to Bot implementation's instance to call its method.
	// A more advanced way is to call sarah package's RegisterCommandProps.
	// In this way, a Command is built and set when Sarah boots up or when the Command's configuration is updated.
	AppendCommand(Command)

	// Run is called when Sarah boots up.
	// On this execution, Bot implementation initiates its interaction with the corresponding chat service.
	// When the chat service sends a message, convert that message payload into Input implementation and send it to inputReceiver.
	// Sarah's internal worker receives the Input and proceeds to find and execute the corresponding command.
	//
	// The initiation of Sarah allocates a new goroutine for each bot so this method can block until when the interaction ends.
	// When this method returns, the interaction is considered finished.
	//
	// The bot lifecycle is entirely managed by Sarah.
	// On critical situation, notify such event via notifyErr and let Sarah handle the error.
	// When the bot is indeed in a critical state and can not proceed further operation, ctx is canceled by Sarah.
	// Bot/Adapter developers may listen to this ctx.Done() to clean up its internal resources.
	Run(ctx context.Context, inputReceiver func(Input) error, notifyErr func(error))
}

type defaultBot struct {
	botType            BotType
	runFunc            func(context.Context, func(Input) error, func(error))
	sendMessageFunc    func(context.Context, Output)
	commands           *Commands
	userContextStorage UserContextStorage
}

// NewBot creates a new defaultBot instance with the given Adapter implementation.
// While an Adapter takes care of actual collaboration with each chat service provider,
// defaultBot takes care of some common tasks including:
//   - receive an Input
//   - see if sending user is in the middle of conversational context
//     - if so, execute the next step with the given Input
//     - if not, find a corresponding Command for the given Input and execute it
//   - call Adapter.SendMessage to send an output
// The purpose of defaultBot is to lessen the tasks of Adapter developers by providing some common tasks' implementations
// and ease the creation of Bot implementation.
// Instead of passing an Adapter implementation to NewBot, Developers can also develop a Bot implementation from scratch to highly customize the behavior.
//
// Some optional settings can be supplied by passing DefaultBotOption values returned by functions including BotWithStorage.
//
//  // Use a storage.
//  storage := sarah.NewUserContextStorage(sarah.NewCacheConfig())
//  opt := sarah.BotWithStorage(storage)
//  bot, err := sarah.NewBot(myAdapter, opt)
//
// It is highly recommended to provide an implementation of UserContextStorage, so the users' conversational context can be stored and executed on the next message reception.
// A reference implementation of UserContextStorage can be initialized with NewUserContextStorage.
// This caches user context information in process memory, so the stored context information is lost on process restart.
func NewBot(adapter Adapter, options ...DefaultBotOption) Bot {
	bot := &defaultBot{
		botType:            adapter.BotType(),
		runFunc:            adapter.Run,
		sendMessageFunc:    adapter.SendMessage,
		commands:           NewCommands(),
		userContextStorage: nil,
	}

	for _, opt := range options {
		opt(bot)
	}

	return bot
}

// DefaultBotOption defines a type that a functional option of NewBot must satisfy.
type DefaultBotOption func(bot *defaultBot)

// BotWithStorage creates and returns a DefaultBotOption to register a preferred UserContextStorage implementation.
// The below example utilizes pre-defined in-memory storage.
//
//  config := sarah.NewCacheConfig()
//  configBuf, _ := ioutil.ReadFile("/path/to/storage/config.yaml")
//  yaml.Unmarshal(configBuf, config)
//  bot, err := sarah.NewBot(myAdapter, storage)
func BotWithStorage(storage UserContextStorage) DefaultBotOption {
	return func(bot *defaultBot) {
		bot.userContextStorage = storage
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
		switch in := input.(type) {
		case *HelpInput:
			res = &CommandResponse{
				Content:     bot.commands.Helps(in),
				UserContext: nil,
			}
		default:
			res, err = bot.commands.ExecuteFirstMatched(ctx, input)
		}
	} else {
		e := bot.userContextStorage.Delete(senderKey)
		if e != nil {
			logger.Warnf("Failed to delete UserContext: BotType: %s. SenderKey: %s. Error: %+v", bot.BotType(), senderKey, e)
		}

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
			logger.Errorf("Failed to store UserContext. BotType: %s. SenderKey: %s. UserContext: %#v. Error: %+v", bot.BotType(), senderKey, res.UserContext, err)
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

// NewSuppressedResponseWithNext creates a new CommandResponse without a returning message but with a next step to continue.
// When this is returned by Command execution, no response is returned to the user but the user context is still set.
func NewSuppressedResponseWithNext(next ContextualFunc) *CommandResponse {
	return &CommandResponse{
		Content:     nil,
		UserContext: NewUserContext(next),
	}
}
