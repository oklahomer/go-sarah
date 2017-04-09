package sarah

import (
	"errors"
	"golang.org/x/net/context"
	"reflect"
	"testing"
	"time"
)

type DummyBot struct {
	BotTypeValue      BotType
	RespondFunc       func(context.Context, Input) error
	SendMessageFunc   func(context.Context, Output)
	AppendCommandFunc func(Command)
	RunFunc           func(context.Context, func(Input) error, func(error))
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

func (bot *DummyBot) Run(ctx context.Context, enqueueInput func(Input) error, notifyErr func(error)) {
	bot.RunFunc(ctx, enqueueInput, notifyErr)
}

func Test_NewBot(t *testing.T) {
	adapter := &DummyAdapter{}
	myBot := NewBot(adapter, NewCacheConfig())
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

func TestDefaultBot_AppendCommand(t *testing.T) {
	myBot := &defaultBot{commands: NewCommands()}

	command := &DummyCommand{}
	myBot.AppendCommand(command)

	registeredCommands := myBot.commands
	if len(*registeredCommands) != 1 {
		t.Errorf("1 registered command should exists: %#v.", registeredCommands)
	}
}

func TestDefaultBot_Respond_CacheAcquisitionError(t *testing.T) {
	cacheError := errors.New("cache error")
	dummyCache := &DummyCachedUserContexts{
		GetFunc: func(_ string) (*UserContext, error) {
			return nil, cacheError
		},
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
	}

	dummyInput := &DummyInput{
		SenderKeyValue: "senderKey",
	}

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != cacheError {
		t.Errorf("Expected error was not returned: %#v.", err)
	}
}

func TestDefaultBot_Respond_WithoutContext(t *testing.T) {
	dummyCache := &DummyCachedUserContexts{
		GetFunc: func(_ string) (*UserContext, error) {
			return nil, nil
		},
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
		commands:         NewCommands(),
	}

	dummyInput := &DummyInput{
		SenderKeyValue: "senderKey",
		MessageValue:   ".echo foo",
	}

	err := myBot.Respond(context.TODO(), dummyInput)
	if err != nil {
		t.Errorf("Unexpected error is returned: %#v.", err)
	}
}

func TestDefaultBot_Respond_WithContextButMessage(t *testing.T) {
	var givenNext ContextualFunc
	dummyCache := &DummyCachedUserContexts{
		GetFunc: func(_ string) (*UserContext, error) {
			return nil, nil
		},
		SetFunc: func(_ string, userContext *UserContext) error {
			givenNext = userContext.Next
			return nil
		},
	}

	nextFunc := func(_ context.Context, input Input) (*CommandResponse, error) {
		return nil, nil
	}
	command := &DummyCommand{
		MatchFunc: func(_ string) bool {
			return true
		},
		ExecuteFunc: func(_ context.Context, _ Input) (*CommandResponse, error) {
			return &CommandResponse{
				Content: nil,
				Next:    nextFunc,
			}, nil
		},
	}

	isSent := false
	myBot := &defaultBot{
		userContextCache: dummyCache,
		commands:         &Commands{command},
		sendMessageFunc: func(_ context.Context, output Output) {
			isSent = true
		},
	}
	err := myBot.Respond(context.TODO(), &DummyInput{})

	if err != nil {
		t.Fatalf("Unexpected error is returned: %#v.", err)
	}

	if reflect.ValueOf(givenNext).Pointer() != reflect.ValueOf(nextFunc).Pointer() {
		t.Errorf("Unexpected ContextualFunc is set %#v.", givenNext)
	}

	if isSent == true {
		t.Error("Unexpected call to Bot.SendMessage.")
	}
}

func TestDefaultBot_Respond_WithContext(t *testing.T) {
	nextFunc := func(_ context.Context, input Input) (*CommandResponse, error) {
		return nil, nil
	}
	responseContent := &struct{}{}
	var givenNext ContextualFunc
	dummyCache := &DummyCachedUserContexts{
		DeleteFunc: func(_ string) error {
			return nil
		},
		GetFunc: func(_ string) (*UserContext, error) {
			return NewUserContext(func(_ context.Context, input Input) (*CommandResponse, error) {
				return &CommandResponse{
					Content: responseContent,
					Next:    nextFunc,
				}, nil
			}), nil
		},
		SetFunc: func(_ string, userContext *UserContext) error {
			givenNext = userContext.Next
			return nil
		},
	}

	var passedContent interface{}
	var passedDestination OutputDestination
	myBot := &defaultBot{
		sendMessageFunc: func(_ context.Context, output Output) {
			passedContent = output.Content()
			passedDestination = output.Destination()
		},
		userContextCache: dummyCache,
		commands:         NewCommands(),
	}

	dummyInput := &DummyInput{
		SenderKeyValue: "senderKey",
		MessageValue:   ".echo foo",
		ReplyToValue:   "replyTo",
	}

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

	if passedDestination != dummyInput.ReplyToValue {
		t.Errorf("Expected reply destination is not passed: %#v.", passedDestination)
	}
}

func TestDefaultBot_Respond_Abort(t *testing.T) {
	isCacheDeleted := false
	dummyCache := &DummyCachedUserContexts{
		DeleteFunc: func(_ string) error {
			isCacheDeleted = true
			return nil
		},
		GetFunc: func(_ string) (*UserContext, error) {
			return NewUserContext(func(_ context.Context, input Input) (*CommandResponse, error) {
				panic("Don't call me!!!")
			}), nil
		},
	}

	myBot := &defaultBot{
		userContextCache: dummyCache,
	}

	err := myBot.Respond(context.TODO(), &AbortInput{})
	if err != nil {
		t.Errorf("Unexpected error returned: %#v.", err)
	}
	if isCacheDeleted == false {
		t.Error("Cached context is not deleted.")
	}
}

func TestDefaultBot_Respond_Help(t *testing.T) {
	commandID := "id"
	example := "e.g."
	cmd := &DummyCommand{
		IdentifierValue: commandID,
		InputExampleFunc: func() string {
			return example
		},
	}

	var givenOutput Output
	dummyCache := &DummyCachedUserContexts{
		GetFunc: func(_ string) (*UserContext, error) {
			return nil, nil
		},
	}
	myBot := &defaultBot{
		userContextCache: dummyCache,
		commands:         &Commands{cmd},
		sendMessageFunc: func(_ context.Context, output Output) {
			givenOutput = output
		},
	}

	dest := "destination"
	dummyInput := NewHelpInput("sender", "message", time.Now(), dest)
	err := myBot.Respond(context.TODO(), dummyInput)
	if err != nil {
		t.Errorf("Unexpected error is returned: %#v.", err)
	}

	if givenOutput == nil {
		t.Fatal("Passed output is nil")
	}
	helps := givenOutput.Content().(*CommandHelps)
	if len(*helps) != 1 {
		t.Fatalf("Expectnig one help to be given, but was %d.", len(*helps))
	}
	if (*helps)[0].Identifier != commandID {
		t.Errorf("Expected ID was not returned: %s.", (*helps)[0].Identifier)
	}
	if (*helps)[0].InputExample != example {
		t.Errorf("Expected example was not returned: %s.", (*helps)[0].InputExample)
	}
}

func TestDefaultBot_Run(t *testing.T) {
	adapterProcessed := false
	bot := &defaultBot{
		runFunc: func(_ context.Context, _ func(Input) error, _ func(error)) {
			adapterProcessed = true
		},
	}

	rootCtx := context.Background()
	botCtx, cancelBot := context.WithCancel(rootCtx)
	defer cancelBot()
	bot.Run(botCtx, func(_ Input) error { return nil }, func(_ error) {})

	if adapterProcessed == false {
		t.Error("Adapter.Run is not called.")
	}
}

func TestDefaultBot_SendMessage(t *testing.T) {
	adapterProcessed := false
	bot := &defaultBot{
		sendMessageFunc: func(_ context.Context, _ Output) {
			adapterProcessed = true
		},
	}

	output := NewOutputMessage(struct{}{}, struct{}{})
	bot.SendMessage(context.TODO(), output)

	if adapterProcessed == false {
		t.Error("Adapter.SendMessage is not called.")
	}
}

func TestNewSuppressedResponseWithNext(t *testing.T) {
	nextFunc := func(_ context.Context, input Input) (*CommandResponse, error) {
		return nil, nil
	}
	res := NewSuppressedResponseWithNext(nextFunc)

	if res == nil {
		t.Fatal("CommandResponse is not initialized.")
	}

	if reflect.ValueOf(res.Next).Pointer() != reflect.ValueOf(nextFunc).Pointer() {
		t.Errorf("Unexpected ContextualFunc is set %#v.", res.Next)
	}
}
