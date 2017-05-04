package slack

import (
	"errors"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/golack/rtmapi"
	"github.com/oklahomer/golack/webapi"
	"golang.org/x/net/context"
	"reflect"
	"testing"
	"time"
)

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
	SendFunc    func(rtmapi.ChannelID, string) error
	PingFunc    func() error
	CloseFunc   func() error
}

func (conn *DummyConnection) Receive() (rtmapi.DecodedPayload, error) {
	return conn.ReceiveFunc()
}

func (conn *DummyConnection) Send(channel rtmapi.ChannelID, content string) error {
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

func TestNewAdapter_WithOptionError(t *testing.T) {
	config := &Config{}
	expectedErr := errors.New("dummy")

	adapter, err := NewAdapter(config, func(_ *Adapter) error {
		return expectedErr
	})

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if err != expectedErr {
		t.Errorf("Unexpected error is returned: %s.", err.Error())
	}

	if adapter != nil {
		t.Error("Adapter should not be returned.")
	}
}

func TestAdapter_BotType(t *testing.T) {
	adapter := &Adapter{}

	if adapter.BotType() != SLACK {
		t.Errorf("Unexpected BotType is returned: %s.", adapter.BotType())
	}
}

func TestAdapter_startRTMSession_WithError(t *testing.T) {
	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return nil, errors.New("connection error")
		},
	}

	retrialCnt := 3
	rtmStart, err := startRTMSession(context.TODO(), client, uint(retrialCnt), time.Duration(0))

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if e, ok := err.(*retry.Errors); ok {
		if len(*e) != retrialCnt {
			t.Errorf("# of error should be equal to that of retrial: %d.", len(*e))
		}

	} else {
		t.Fatalf("Returned error is not instance of retry.Errors: %#v.", err)

	}

	if rtmStart != nil {
		t.Errorf("RTMStart instant should not be returned: %#v.", rtmStart)
	}
}

func TestAdapter_startRTMSession(t *testing.T) {
	client := &DummyClient{
		StartRTMSessionFunc: func(_ context.Context) (*webapi.RTMStart, error) {
			return &webapi.RTMStart{}, nil
		},
	}

	rtmStart, err := startRTMSession(context.TODO(), client, 3, time.Duration(0))

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if rtmStart == nil {
		t.Fatal("RTMStart instance should be returned.")
	}
}

func TestAdapter_connectRTM_WithError(t *testing.T) {
	client := &DummyClient{
		ConnectRTMFunc: func(_ context.Context, _ string) (rtmapi.Connection, error) {
			return nil, errors.New("connection error.")
		},
	}
	rtmStart := &webapi.RTMStart{
		URL: "http://localhsot/",
	}

	retrialCnt := 3
	conn, err := connectRTM(context.TODO(), client, rtmStart, uint(retrialCnt), time.Duration(0))

	if err == nil {
		t.Fatal("Expected error is not returned.")
	}

	if e, ok := err.(*retry.Errors); ok {
		if len(*e) != retrialCnt {
			t.Errorf("# of error should be equal to that of retrial: %d.", len(*e))
		}

	} else {
		t.Fatalf("Returned error is not instance of retry.Errors: %#v.", err)

	}

	if conn != nil {
		t.Errorf("Connection instant should not be returned: %#v.", conn)
	}
}

func TestMessageInput(t *testing.T) {
	channelID := "id"
	senderID := "who"
	content := "Hello, 世界"
	timestamp := time.Now()
	rtmMessage := &rtmapi.Message{
		CommonEvent: rtmapi.CommonEvent{
			Type: rtmapi.MessageEvent,
		},
		ChannelID: rtmapi.ChannelID(channelID),
		Sender:    rtmapi.UserID(senderID),
		Text:      content,
		TimeStamp: &rtmapi.TimeStamp{
			Time:          timestamp,
			OriginalValue: timestamp.String() + ".99999",
		},
	}

	input := &MessageInput{event: rtmMessage}

	if input == nil {
		t.Fatal("MessageInput instance is not returned.")
	}

	if input.SenderKey() != channelID+"|"+senderID {
		t.Errorf("Unexpected SenderKey is retuned: %s.", input.SenderKey())
	}

	if input.Message() != content {
		t.Errorf("Unexpected Message is returned: %s.", input.Message())
	}

	if string(input.ReplyTo().(rtmapi.ChannelID)) != channelID {
		t.Errorf("Unexpected ReplyTo is returned: %s.", input.ReplyTo())
	}

	if input.SentAt() != timestamp {
		t.Errorf("Unexpected SentAt is returned: %s.", input.SentAt().String())
	}
}

func TestAdapter_connectRTM(t *testing.T) {
	client := &DummyClient{
		ConnectRTMFunc: func(_ context.Context, _ string) (rtmapi.Connection, error) {
			return &DummyConnection{}, nil
		},
	}
	rtmStart := &webapi.RTMStart{
		URL: "http://localhsot/",
	}

	conn, err := connectRTM(context.TODO(), client, rtmStart, 3, time.Duration(0))

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if conn == nil {
		t.Fatal("Connection instance should be returned.")
	}
}

func TestNewStringResponse(t *testing.T) {
	str := "abc"
	res := NewStringResponse(str)

	if res.Content != str {
		t.Errorf("expected content is not returned: %s.", res.Content)
	}

	if res.UserContext != nil {
		t.Errorf("UserContext should not be returned: %#v.", res.UserContext)
	}
}

func TestNewStringResponseWithNext(t *testing.T) {
	str := "abc"
	next := func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}
	res := NewStringResponseWithNext(str, next)

	if res.Content != str {
		t.Errorf("expected content is not returned: %s.", res.Content)
	}

	if res.UserContext == nil {
		t.Fatal("Expected UserContxt is not stored.")
	}

	if reflect.ValueOf(res.UserContext.Next).Pointer() != reflect.ValueOf(next).Pointer() {
		t.Fatalf("expected next step is not returned: %#v.", res.UserContext.Next)
	}
}

func TestNewPostMessageResponse(t *testing.T) {
	channelID := "id"
	input := NewMessageInput(
		&rtmapi.Message{
			CommonEvent: rtmapi.CommonEvent{
				Type: rtmapi.MessageEvent,
			},
			ChannelID: rtmapi.ChannelID(channelID),
			Sender:    rtmapi.UserID("who"),
			Text:      ".echo foo",
			TimeStamp: &rtmapi.TimeStamp{
				Time:          time.Now(),
				OriginalValue: time.Now().String() + ".99999",
			},
		},
	)
	message := "this  is my message."
	attachments := []*webapi.MessageAttachment{
		{},
	}

	res := NewPostMessageResponse(input, message, attachments)

	if postMessage, ok := res.Content.(*webapi.PostMessage); ok {
		if len(postMessage.Attachments) != 1 {
			t.Errorf("One attachment should exists: %d.", len(postMessage.Attachments))
		}

		if postMessage.Channel != channelID {
			t.Errorf("Unexpected Channel value is given: %s.", postMessage.Channel)
		}

	} else {
		t.Errorf("Unexpected response content is set: %#v.", res.Content)

	}

	if res.UserContext != nil {
		t.Errorf("Unexpected UserContext is returned: %#v.", res.UserContext)
	}
}

func TestNewPostMessageResponseWithNext(t *testing.T) {
	channelID := "id"
	input := NewMessageInput(
		&rtmapi.Message{
			CommonEvent: rtmapi.CommonEvent{
				Type: rtmapi.MessageEvent,
			},
			ChannelID: rtmapi.ChannelID(channelID),
			Sender:    rtmapi.UserID("who"),
			Text:      ".echo foo",
			TimeStamp: &rtmapi.TimeStamp{
				Time:          time.Now(),
				OriginalValue: time.Now().String() + ".99999",
			},
		},
	)
	message := "this  is my message."
	attachments := []*webapi.MessageAttachment{
		{},
	}
	next := func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}

	res := NewPostMessageResponseWithNext(input, message, attachments, next)

	if postMessage, ok := res.Content.(*webapi.PostMessage); ok {
		if len(postMessage.Attachments) != 1 {
			t.Errorf("One attachment should exists: %d.", len(postMessage.Attachments))
		}

		if postMessage.Channel != channelID {
			t.Errorf("Unexpected Channel value is given: %s.", postMessage.Channel)
		}

	} else {
		t.Errorf("Unexpected response content is set: %#v.", res.Content)

	}

	if res.UserContext == nil {
		t.Fatal("Expected UserContext is not set")
	}

	if reflect.ValueOf(res.UserContext.Next).Pointer() != reflect.ValueOf(next).Pointer() {
		t.Fatalf("expected next step is not returned: %#v.", res.UserContext.Next)
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
			payload: &rtmapi.WebSocketReply{
				OK:   false,
				Text: "no good",
			},
			inputType: nil,
		},
		{
			payload: &rtmapi.Message{
				ChannelID: rtmapi.ChannelID("abc"),
				Sender:    rtmapi.UserID("cde"),
				Text:      helpCommand,
				TimeStamp: &rtmapi.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.HelpInput{}).Type(),
		},
		{
			payload: &rtmapi.Message{
				ChannelID: rtmapi.ChannelID("abc"),
				Sender:    rtmapi.UserID("cde"),
				Text:      abortCommand,
				TimeStamp: &rtmapi.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.AbortInput{}).Type(),
		},
		{
			payload: &rtmapi.Message{
				ChannelID: rtmapi.ChannelID("abc"),
				Sender:    rtmapi.UserID("cde"),
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
