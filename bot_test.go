package sarah

import (
	"errors"
	"golang.org/x/net/context"
	"reflect"
	"testing"
)

type DummyBot struct {
	BotTypeValue BotType

	RespondFunc func(context.Context, Input) error

	SendMessageFunc func(context.Context, Output)

	AppendCommandFunc func(Command)

	RunFunc func(context.Context, chan<- Input, chan<- error)

	PluginConfigDirFunc func() string
}

func (bot *DummyBot) BotType() BotType {
	return bot.BotTypeValue
}

func (bot *DummyBot) Respond(ctx context.Context, input Input) error {
	return bot.RespondFunc(ctx, input)
}

func (bot *DummyBot) SendMessage(ctx context.Context, output Output) {
	bot.SendMessageFunc(ctx, output)
}

func (bot *DummyBot) AppendCommand(command Command) {
	bot.AppendCommandFunc(command)
}

func (bot *DummyBot) Run(ctx context.Context, input chan<- Input, errCh chan<- error) {
	bot.RunFunc(ctx, input, errCh)
}

func (bot *DummyBot) PluginConfigDir() string {
	return bot.PluginConfigDirFunc()
}

func Test_NewBot(t *testing.T) {
	adapter := &DummyAdapter{}
	myBot := NewBot(adapter, NewCacheConfig(), "")
	if _, ok := myBot.(*defaultBot); !ok {
		t.Errorf("newBot did not return bot instance: %#v.", myBot)
	}
}

func TestDefaultBot_BotType(t *testing.T) {
	var botType BotType = "slack"
	myBot := &defaultBot{botType: botType}

	if myBot.BotType() != botType {
		t.Errorf("Bot type is wrong: %s.", myBot.BotType())
	}
}

func TestDefaultBot_PluginConfigDir(t *testing.T) {
	dummyPluginDir := "/dummy/path/to/config"
	myBot := &defaultBot{pluginConfigDir: dummyPluginDir}

	if myBot.PluginConfigDir() != dummyPluginDir {
		t.Errorf("Plugin configuration file's location is wrong: %s.", myBot.PluginConfigDir())
	}
}

func TestDefaultBot_AppendCommand(t *testing.T) {
	myBot := &defaultBot{commands: NewCommands()}

	command := &DummyCommand{}
	myBot.AppendCommand(command)

	registeredCommands := myBot.commands
	if len(registeredCommands.cmd) != 1 {
		t.Errorf("1 registered command should exists: %#v.", registeredCommands)
	}
}

func TestDefaultBot_Respond_CacheAcquisitionError(t *testing.T) {
	cacheError := errors.New("cache error")
	dummyCache := &DummyCachedUserContexts{}
	dummyCache.GetFunc = func(_ string) (*UserContext, error) {
		return nil, cacheError
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
	}

	dummyInput := &DummyInput{}
	dummyInput.SenderKeyValue = "senderKey"

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != cacheError {
		t.Errorf("Expected error was not returned: %#v.", err)
	}
}

func TestDefaultBot_Respond_WithoutContext(t *testing.T) {
	dummyCache := &DummyCachedUserContexts{}
	dummyCache.GetFunc = func(_ string) (*UserContext, error) {
		return nil, nil
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
		commands:         NewCommands(),
	}

	dummyInput := &DummyInput{}
	dummyInput.SenderKeyValue = "senderKey"
	dummyInput.MessageValue = ".echo foo"

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != nil {
		t.Errorf("Unexpected error is returned: %#v.", err)
	}
}

func TestDefaultBot_Respond_WithContext(t *testing.T) {
	dummyCache := &DummyCachedUserContexts{}
	dummyCache.DeleteFunc = func(_ string) {
		return
	}
	nextFunc := func(_ context.Context, input Input) (*CommandResponse, error) {
		return nil, nil
	}
	responseContent := &struct{}{}
	dummyCache.GetFunc = func(_ string) (*UserContext, error) {
		return NewUserContext(func(_ context.Context, input Input) (*CommandResponse, error) {
			return &CommandResponse{
				Content: responseContent,
				Next:    nextFunc,
			}, nil
		}), nil
	}

	var givenNext ContextualFunc
	dummyCache.SetFunc = func(_ string, userContext *UserContext) {
		givenNext = userContext.Next
	}

	var passedContent interface{}
	var passedDestination OutputDestination
	sendMessageFunc := func(_ context.Context, output Output) {
		passedContent = output.Content()
		passedDestination = output.Destination()
	}
	myBot := &defaultBot{
		sendMessageFunc:  sendMessageFunc,
		userContextCache: dummyCache,
		commands:         NewCommands(),
	}

	replyDestination := "replyTo"
	dummyInput := &DummyInput{}
	dummyInput.SenderKeyValue = "senderKey"
	dummyInput.MessageValue = ".echo foo"
	dummyInput.ReplyToValue = replyDestination

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != nil {
		t.Errorf("Unexpected error is returned: %#v.", err)
	}

	if reflect.ValueOf(givenNext).Pointer() != reflect.ValueOf(nextFunc).Pointer() {
		t.Errorf("Expected Next step is not passed: %#v.", givenNext)
	}

	if passedContent != responseContent {
		t.Errorf("Expected message content is not passed: %#v.", passedContent)
	}
	if passedDestination != replyDestination {
		t.Errorf("Expected reply destination is not passed: %#v.", passedDestination)
	}
}

func TestDefaultBot_Respond_Abort(t *testing.T) {
	dummyCache := &DummyCachedUserContexts{}
	isCacheDeleted := false
	dummyCache.DeleteFunc = func(_ string) {
		isCacheDeleted = true
	}
	dummyCache.GetFunc = func(_ string) (*UserContext, error) {
		return NewUserContext(func(_ context.Context, input Input) (*CommandResponse, error) {
			panic("Don't call me!!!")
		}), nil
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
	}

	replyDestination := "replyTo"
	dummyInput := &DummyInput{}
	dummyInput.SenderKeyValue = "senderKey"
	dummyInput.MessageValue = ".abort"
	dummyInput.ReplyToValue = replyDestination

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != nil {
		t.Errorf("Unexpected error returned: %#v.", err)
	}
	if isCacheDeleted == false {
		t.Error("Cached context is not deleted.")
	}
}

func TestDefaultBot_Run(t *testing.T) {
	adapterProcessed := false
	runFunc := func(_ context.Context, _ chan<- Input, _ chan<- error) {
		adapterProcessed = true
	}
	bot := &defaultBot{runFunc: runFunc}

	inputReceiver := make(chan Input)
	errCh := make(chan error)
	rootCtx := context.Background()
	botCtx, cancelBot := context.WithCancel(rootCtx)
	defer cancelBot()
	bot.Run(botCtx, inputReceiver, errCh)

	if adapterProcessed == false {
		t.Error("Adapter.Run is not called.")
	}
}

func TestDefaultBot_SendMessage(t *testing.T) {
	adapterProcessed := false
	sendMessageFunc := func(_ context.Context, _ Output) {
		adapterProcessed = true
	}
	bot := &defaultBot{sendMessageFunc: sendMessageFunc}

	output := NewOutputMessage(struct{}{}, struct{}{})
	bot.SendMessage(context.TODO(), output)

	if adapterProcessed == false {
		t.Error("Adapter.SendMessage is not called.")
	}
}
