package slack

import (
	"context"
	"errors"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/golack/rtmapi"
	"github.com/oklahomer/golack/slackobject"
	"github.com/oklahomer/golack/webapi"
	"io/ioutil"
	stdLogger "log"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := log.GetLogger()
	defer log.SetLogger(oldLogger)

	// Suppress log output in test by default
	l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
	logger := log.NewWithStandardLogger(l)
	log.SetLogger(logger)

	code := m.Run()

	os.Exit(code)
}

type DummyClient struct {
	StartRTMSessionFunc func(context.Context) (*webapi.RTMStart, error)
	ConnectRTMFunc      func(context.Context, string) (rtmapi.Connection, error)
	PostMessageFunc     func(context.Context, *webapi.PostMessage) (*webapi.APIResponse, error)
}

func (client *DummyClient) StartRTMSession(ctx context.Context) (*webapi.RTMStart, error) {
	return client.StartRTMSessionFunc(ctx)
}

func (client *DummyClient) ConnectRTM(ctx context.Context, url string) (rtmapi.Connection, error) {
	return client.ConnectRTMFunc(ctx, url)
}

func (client *DummyClient) PostMessage(ctx context.Context, message *webapi.PostMessage) (*webapi.APIResponse, error) {
	return client.PostMessageFunc(ctx, message)
}

type DummyConnection struct {
	ReceiveFunc func() (rtmapi.DecodedPayload, error)
	SendFunc    func(slackobject.ChannelID, string) error
	PingFunc    func() error
	CloseFunc   func() error
}

func (conn *DummyConnection) Receive() (rtmapi.DecodedPayload, error) {
	return conn.ReceiveFunc()
}

func (conn *DummyConnection) Send(channel slackobject.ChannelID, content string) error {
	return conn.SendFunc(channel, content)
}

func (conn *DummyConnection) Ping() error {
	return conn.PingFunc()
}

func (conn *DummyConnection) Close() error {
	return conn.CloseFunc()
}

func TestNewAdapter(t *testing.T) {
	config := &Config{
		Token:          "dummy",
		RequestTimeout: time.Duration(10),
	}
	adapter, err := NewAdapter(config)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if adapter.config != config {
		t.Errorf("Expected config struct is not set: %#v.", adapter.config)
	}

	if adapter.client == nil {
		t.Error("Golack client instance is not set.")
	}

	if adapter.messageQueue == nil {
		t.Error("Message queue channel is nil.")
	}
}

func TestNewAdapter_WithUnConfigurableClient(t *testing.T) {
	config := &Config{}
	adapter, err := NewAdapter(config)

	if err == nil {
		t.Error("Expected error is not returned")
	}

	if adapter != nil {
		t.Fatal("Adapter should not be returned.")
	}
}

func TestNewAdapter_WithSlackClient(t *testing.T) {
	config := &Config{}
	client := &DummyClient{}
	opt := WithSlackClient(client)

	adapter, err := NewAdapter(config, opt)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if adapter == nil {
		t.Fatal("Adapter should be returned.")
	}

	if adapter.client != client {
		t.Error("Provided SlackClient is not set.")
	}
}

func TestNewAdapter_WithPayloadHandler(t *testing.T) {
	fnc := func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {}
	opt := WithPayloadHandler(fnc)
	adapter := &Adapter{}

	opt(adapter)

	if adapter.payloadHandler == nil {
		t.Fatal("PayloadHandler is not set.")
	}

	if reflect.ValueOf(adapter.payloadHandler).Pointer() != reflect.ValueOf(fnc).Pointer() {
		t.Fatal("Provided function is not set.")
	}
}

func TestAdapter_BotType(t *testing.T) {
	adapter := &Adapter{}

	if adapter.BotType() != SLACK {
		t.Errorf("Unexpected BotType is returned: %s.", adapter.BotType())
	}
}

func TestAdapter_superviseConnection(t *testing.T) {
	send := make(chan struct{}, 1)
	ping := make(chan struct{}, 1)
	conn := &DummyConnection{
		SendFunc: func(_ slackobject.ChannelID, _ string) error {
			send <- struct{}{}
			return nil
		},
		PingFunc: func() error {
			select {
			case ping <- struct{}{}:
			default:
				// Duplicate entry. Just ignore.
			}
			return nil
		},
	}
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	pingInterval := 10 * time.Millisecond
	adapter := &Adapter{
		config: &Config{
			PingInterval: pingInterval,
		},
		messageQueue: make(chan *textMessage, 1),
	}

	conErr := make(chan error)
	go func() {
		err := adapter.superviseConnection(ctx, conn, make(chan struct{}, 1))
		conErr <- err
	}()

	adapter.messageQueue <- &textMessage{
		channel: "dummy",
		text:    "Hello, 世界",
	}

	time.Sleep(pingInterval + 10*time.Millisecond) // Give long enough time to check ping.

	cancel()
	select {
	case <-send:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Connection.Send was not called.")
	}

	select {
	case <-ping:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Connection.Ping was not called.")
	}

	select {
	case err := <-conErr:
		if err != nil {
			t.Errorf("Unexpected error was returned: %s.", err.Error())
		}

	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Context was canceled, but superviseConnection did not return.")

	}
}

func TestAdapter_superviseConnection_ConnectionPingError(t *testing.T) {
	conn := &DummyConnection{
		PingFunc: func() error {
			return errors.New("ping error")
		},
	}

	pingInterval := 10 * time.Millisecond
	adapter := &Adapter{
		config: &Config{
			PingInterval: pingInterval,
		},
	}

	conErr := make(chan error)
	go func() {
		err := adapter.superviseConnection(context.TODO(), conn, make(chan struct{}, 1))
		conErr <- err
	}()

	time.Sleep(pingInterval + 10*time.Millisecond) // Give long enough time to check ping.

	select {
	case err := <-conErr:
		if err == nil {
			t.Error("Expected error is not returned.")
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Error is not returned.")
	}
}

func TestAdapter_superviseConnection_ConnectionSendError(t *testing.T) {
	conn := &DummyConnection{
		SendFunc: func(_ slackobject.ChannelID, _ string) error {
			return errors.New("send error")
		},
		PingFunc: func() error {
			return errors.New("ping error")
		},
	}

	adapter := &Adapter{
		config: &Config{
			PingInterval: 100 * time.Second, // not for scheduled ping test
		},
		messageQueue: make(chan *textMessage),
	}

	conErr := make(chan error)
	go func() {
		err := adapter.superviseConnection(context.TODO(), conn, make(chan struct{}, 1))
		conErr <- err
	}()

	adapter.messageQueue <- &textMessage{
		channel: "dummy",
		text:    "Hello, 世界",
	}

	// Connection.Send error should trigger Connection.Ping, and Connection.Ping error triggers supervise failure.
	select {
	case err := <-conErr:
		if err == nil {
			t.Error("Expected error is not returned.")
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Error is not returned.")
	}
}

func TestAdapter_receivePayload(t *testing.T) {
	given := make(chan struct{})
	adapter := &Adapter{
		payloadHandler: func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {
			given <- struct{}{}
		},
	}

	conn := &DummyConnection{
		ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
			return struct{}{}, nil
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	go adapter.receivePayload(ctx, conn, make(chan struct{}), func(_ sarah.Input) error { return nil })

	select {
	case <-given:
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("PayloadHandler is not called.")
	}
}

func TestAdapter_receivePayload_Error(t *testing.T) {
	adapter := &Adapter{
		payloadHandler: func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {
			t.Fatal("PayloadHandler should not be called.")
		},
	}

	i := 0
	errs := []error{
		rtmapi.ErrEmptyPayload,
		rtmapi.NewMalformedPayloadError("dummy"),
		errors.New("random error"),
	}
	conn := &DummyConnection{
		ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
			if i < len(errs) {
				err := errs[i]
				i++
				return nil, err
			}

			i++
			return nil, nil
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	go adapter.receivePayload(ctx, conn, make(chan struct{}), func(_ sarah.Input) error { return nil })

	time.Sleep(100 * time.Millisecond) // Give long enough time to receive all errors.
}

func TestAdapter_Run(t *testing.T) {
	closeCh := make(chan struct{})
	conn := &DummyConnection{
		ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
			return nil, nil
		},
		CloseFunc: func() error {
			closeCh <- struct{}{}
			return nil
		},
	}

	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return &webapi.RTMStart{
				URL: "ws://localhost/dummy",
			}, nil
		},
		ConnectRTMFunc: func(_ context.Context, _ string) (rtmapi.Connection, error) {
			return conn, nil
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	adapter := &Adapter{
		config: &Config{
			PingInterval: 100 * time.Second,
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		client: client,
	}

	go adapter.Run(
		ctx,
		func(_ sarah.Input) error {
			return nil
		},
		func(err error) {
			t.Fatalf("Unexpected errr is returned: %s.", err.Error())
		},
	)

	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case <-closeCh:
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Adapter.Close was not called after Context cancellation.")
	}
}

func TestAdapter_Run_ConnectionInitializationError(t *testing.T) {
	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return nil, errors.New("failed to fetch RTM information")
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	adapter := &Adapter{
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		client: client,
	}

	errCh := make(chan error)
	go adapter.Run(
		ctx,
		func(_ sarah.Input) error {
			return nil
		},
		func(err error) {
			errCh <- err
		},
	)

	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case <-errCh:
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Expected error did not occur.")
	}
}

func TestAdapter_Run_ConnectionAbortionError(t *testing.T) {
	closeCh := make(chan struct{})
	conn := &DummyConnection{
		PingFunc: func() error {
			return errors.New("ping error")
		},
		ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
			return nil, errors.New("message reception error")
		},
		CloseFunc: func() error {
			closeCh <- struct{}{}
			return nil
		},
	}

	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return &webapi.RTMStart{
				URL: "ws://localhost/dummy",
			}, nil
		},
		ConnectRTMFunc: func(_ context.Context, _ string) (rtmapi.Connection, error) {
			return conn, nil
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	adapter := &Adapter{
		config: &Config{
			PingInterval: 100 * time.Second,
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		client: client,
	}

	go adapter.Run(
		ctx,
		func(_ sarah.Input) error {
			return nil
		},
		func(err error) {
			t.Fatalf("Unexpected errr is returned: %s.", err.Error())
		},
	)

	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case <-closeCh:
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("Adapter.Close was not called after Context cancellation.")
	}
}

func TestAdapter_SendMessage_String(t *testing.T) {
	adapter := &Adapter{
		messageQueue: make(chan *textMessage, 1),
	}

	output := sarah.NewOutputMessage(slackobject.ChannelID("ch"), "test")
	adapter.SendMessage(context.TODO(), output)
	select {
	case <-adapter.messageQueue:
		// O.K.
	default:
		t.Fatalf("Valid output was not enqueued.")
	}

	invalid := sarah.NewOutputMessage("invalid", "test")
	adapter.SendMessage(context.TODO(), invalid)
	select {
	case <-adapter.messageQueue:
		t.Fatalf("Invalid output was enqueued.")
	default:
		// O.K.
	}
}

func TestAdapter_SendMessage_PostMessage(t *testing.T) {
	called := false
	adapter := &Adapter{
		client: &DummyClient{
			PostMessageFunc: func(_ context.Context, _ *webapi.PostMessage) (*webapi.APIResponse, error) {
				called = true
				return nil, errors.New("post error") // Should not cause panic.
			},
		},
	}

	postMessage := webapi.NewPostMessage("channelID", "test")
	output := sarah.NewOutputMessage(slackobject.ChannelID("ch"), postMessage)
	adapter.SendMessage(context.TODO(), output)

	if !called {
		t.Fatal("Client.PostMessage is not called.")
	}
}

func TestAdapter_SendMessage_CommandHelps(t *testing.T) {
	called := false
	adapter := &Adapter{
		client: &DummyClient{
			PostMessageFunc: func(_ context.Context, _ *webapi.PostMessage) (*webapi.APIResponse, error) {
				called = true
				return nil, errors.New("post error") // Should not cause panic.
			},
		},
	}

	helps := &sarah.CommandHelps{
		&sarah.CommandHelp{
			Identifier:  "id",
			Instruction: ".help",
		},
	}

	invalid := sarah.NewOutputMessage("invalidID", helps)
	adapter.SendMessage(context.TODO(), invalid)
	if called {
		t.Fatal("Invalid output reached Client.PostMessage.")
	}

	adapter.SendMessage(context.TODO(), sarah.NewOutputMessage(slackobject.ChannelID("test"), helps))
	if !called {
		t.Fatal("Client.PostMessage is not called.")
	}
}

func TestAdapter_SendMessage_IrrelevantType(t *testing.T) {
	postMessageCalled := false
	adapter := &Adapter{
		messageQueue: make(chan *textMessage, 1),
		client: &DummyClient{
			PostMessageFunc: func(_ context.Context, _ *webapi.PostMessage) (*webapi.APIResponse, error) {
				postMessageCalled = true
				return nil, errors.New("post error") // Should not cause panic.
			},
		},
	}

	adapter.SendMessage(context.TODO(), sarah.NewOutputMessage(slackobject.ChannelID("validID"), struct{}{}))

	if postMessageCalled {
		t.Fatal("Invalid content reached Client.PostMessage")
	}

	select {
	case <-adapter.messageQueue:
		t.Fatal("Invalid content is sent as String.")
	case <-time.NewTimer(100 * time.Millisecond).C:
		// O.K.
	}
}

func TestMessageInput(t *testing.T) {
	channelID := "id"
	senderID := "who"
	content := "Hello, 世界"
	timestamp := time.Now()
	rtmMessage := &rtmapi.Message{
		TypedEvent: rtmapi.TypedEvent{
			Type: rtmapi.MessageEvent,
		},
		ChannelID: slackobject.ChannelID(channelID),
		SenderID:  slackobject.UserID(senderID),
		Text:      content,
		TimeStamp: &rtmapi.TimeStamp{
			Time:          timestamp,
			OriginalValue: timestamp.String() + ".99999",
		},
	}

	input := &MessageInput{event: rtmMessage}

	if input.SenderKey() != channelID+"|"+senderID {
		t.Errorf("Unexpected SenderKey is retuned: %s.", input.SenderKey())
	}

	if input.Message() != content {
		t.Errorf("Unexpected Message is returned: %s.", input.Message())
	}

	if string(input.ReplyTo().(slackobject.ChannelID)) != channelID {
		t.Errorf("Unexpected ReplyTo is returned: %s.", input.ReplyTo())
	}

	if input.SentAt() != timestamp {
		t.Errorf("Unexpected SentAt is returned: %s.", input.SentAt().String())
	}
}

func TestAdapter_connect(t *testing.T) {
	client := &DummyClient{
		ConnectRTMFunc: func(_ context.Context, _ string) (rtmapi.Connection, error) {
			return &DummyConnection{}, nil
		},
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return &webapi.RTMStart{}, nil
		},
	}

	adapter := &Adapter{
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		client: client,
	}

	conn, err := adapter.connect(context.TODO())
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if conn == nil {
		t.Error("Connection is not returned.")
	}
}

func TestAdapter_connect_error(t *testing.T) {
	expected := errors.New("expected error")
	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return nil, expected
		},
	}

	adapter := &Adapter{
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		client: client,
	}

	conn, err := adapter.connect(context.TODO())
	if err == nil {
		t.Fatal("Unexpected error is not returned.")
	}

	if conn != nil {
		t.Fatal("Connection should not be returned.")
	}
}

func Test_handlePayload(t *testing.T) {
	helpCommand := ".help"
	abortCommand := ".abort"
	config := &Config{
		HelpCommand:  helpCommand,
		AbortCommand: ".abort",
	}
	inputs := []struct {
		payload   rtmapi.DecodedPayload
		inputType reflect.Type
	}{
		{
			payload: &rtmapi.WebSocketOKReply{
				WebSocketReply: rtmapi.WebSocketReply{
					ReplyTo: 1,
					OK:      true,
				},
				Text: "OK",
			},
			inputType: nil,
		},
		{
			payload: &rtmapi.WebSocketNGReply{
				WebSocketReply: rtmapi.WebSocketReply{
					ReplyTo: 1,
					OK:      false,
				},
				ErrorReason: struct {
					Code    int    `json:"code"`
					Message string `json:"msg"`
				}{
					Code:    404,
					Message: "Not Found",
				},
			},
			inputType: nil,
		},
		{
			payload:   &rtmapi.Pong{},
			inputType: nil,
		},
		{
			payload: &rtmapi.Message{
				ChannelID: slackobject.ChannelID("abc"),
				SenderID:  slackobject.UserID("cde"),
				Text:      helpCommand,
				TimeStamp: &rtmapi.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.HelpInput{}).Type(),
		},
		{
			payload: &rtmapi.Message{
				ChannelID: slackobject.ChannelID("abc"),
				SenderID:  slackobject.UserID("cde"),
				Text:      abortCommand,
				TimeStamp: &rtmapi.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.AbortInput{}).Type(),
		},
		{
			payload: &rtmapi.Message{
				ChannelID: slackobject.ChannelID("abc"),
				SenderID:  slackobject.UserID("cde"),
				Text:      "foo",
				TimeStamp: &rtmapi.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&MessageInput{}).Type(),
		},
		{
			payload:   &rtmapi.PinAdded{},
			inputType: nil,
		},
	}

	for i, input := range inputs {
		var receivedType reflect.Type
		fnc := func(i sarah.Input) error {
			receivedType = reflect.ValueOf(i).Type()
			return nil
		}
		handlePayload(context.TODO(), config, input.payload, fnc)

		if input.inputType == nil && receivedType != nil {
			t.Errorf("Input shuold not be passed this time: %s.", receivedType.String())
		} else if input.inputType == nil {
			// No test
			continue
		}

		if receivedType == nil {
			t.Error("No payload is received")
		} else if receivedType != input.inputType {
			t.Errorf("Unexpected input type is given on %d test: %s.", i, receivedType.String())
		}
	}
}

type DummyInput struct {
}

func (*DummyInput) SenderKey() string {
	return ""
}

func (*DummyInput) Message() string {
	return ""
}

func (*DummyInput) SentAt() time.Time {
	return time.Now()
}

func (*DummyInput) ReplyTo() sarah.OutputDestination {
	return "dummy"
}

func TestNewResponse(t *testing.T) {
	tests := []struct {
		input   sarah.Input
		message string
		options []RespOption
		hasErr  bool
	}{
		{
			input: &MessageInput{
				event: &rtmapi.Message{
					ChannelID: "dummy",
				},
			},
			message: "dummy message",
			hasErr:  false,
		},
		{
			input: &MessageInput{
				event: &rtmapi.Message{
					ChannelID: "dummy",
				},
			},
			message: "dummy message",
			options: []RespOption{
				func(options *respOptions) {
					options.attachments = []*webapi.MessageAttachment{
						{},
						{},
					}
				},
			},
			hasErr: false,
		},
		{
			input:   &DummyInput{},
			message: "dummy message",
			options: []RespOption{
				func(options *respOptions) {
					options.attachments = []*webapi.MessageAttachment{
						{},
						{},
					}
				},
			},
			hasErr: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			response, err := NewResponse(tt.input, tt.message, tt.options...)
			if tt.hasErr {
				if err == nil {
					t.Fatal("Expected error is not returned.")
				}
				return
			}

			if !tt.hasErr && err != nil {
				t.Fatalf("Unexpected error is returned: %s.", err.Error())
			}

			switch typed := response.Content.(type) {
			case string:
				if tt.message != typed {
					t.Errorf("Unxecpected string is set as message: %s", typed)
				}

			case *webapi.PostMessage:
				if tt.message != typed.Text {
					t.Errorf("Unxecpected string is set as message: %s", typed.Text)
				}

			}
		})
	}
}

func TestRespWithAttachments(t *testing.T) {
	options := &respOptions{}
	attachments := []*webapi.MessageAttachment{{}, {}}
	opt := RespWithAttachments(attachments)

	opt(options)

	if len(options.attachments) != len(attachments) {
		t.Fatal("Passed attachments are not set.")
	}
}

func TestRespWithLinkNames(t *testing.T) {
	options := &respOptions{}
	linkNames := 1
	opt := RespWithLinkNames(linkNames)

	opt(options)

	if options.linkNames != linkNames {
		t.Error("Passed linkNames is not set.")
	}
}

func TestRespWithNext(t *testing.T) {
	options := &respOptions{}
	next := func(ctx context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}
	opt := RespWithNext(next)

	opt(options)

	if options.userContext == nil {
		t.Fatal("Passed function is not set.")
	}

	if reflect.ValueOf(options.userContext.Next).Pointer() != reflect.ValueOf(next).Pointer() {
		t.Error("Passed function is not set.")
	}
}

func TestRespWithNextSerializable(t *testing.T) {
	options := &respOptions{}
	arg := &sarah.SerializableArgument{}
	opt := RespWithNextSerializable(arg)

	opt(options)

	if options.userContext == nil {
		t.Fatal("Passed UserContext is not set.")
	}

	if options.userContext.Serializable != arg {
		t.Error("Passed UserContext argument is not set.")
	}
}

func TestRespWithParse(t *testing.T) {
	options := &respOptions{}
	mode := webapi.ParseModeFull
	opt := RespWithParse(mode)

	opt(options)

	if options.parseMode != mode {
		t.Error("Passed parseMode is not set.")
	}
}

func TestRespWithUnfurlLinks(t *testing.T) {
	options := &respOptions{}
	opt := RespWithUnfurlLinks(true)

	opt(options)

	if !options.unfurlLinks {
		t.Error("Passed unfurlLinks is not set.")
	}
}

func TestRespWithUnfurlMedia(t *testing.T) {
	options := &respOptions{}
	opt := RespWithUnfurlMedia(true)

	opt(options)

	if !options.unfurlMedia {
		t.Error("Passed unfurlMedia is not set.")
	}
}
