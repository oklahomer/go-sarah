package slack

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/log"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/eventsapi"
	"github.com/oklahomer/golack/v2/rtmapi"
	"github.com/oklahomer/golack/v2/webapi"
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
	ConnectRTMFunc  func(context.Context) (rtmapi.Connection, error)
	PostMessageFunc func(context.Context, *webapi.PostMessage) (*webapi.APIResponse, error)
	RunServerFunc   func(context.Context, eventsapi.EventReceiver) <-chan error
}

var _ SlackClient = (*DummyClient)(nil)

func (client *DummyClient) ConnectRTM(ctx context.Context) (rtmapi.Connection, error) {
	return client.ConnectRTMFunc(ctx)
}

func (client *DummyClient) PostMessage(ctx context.Context, message *webapi.PostMessage) (*webapi.APIResponse, error) {
	return client.PostMessageFunc(ctx, message)
}

func (client *DummyClient) RunServer(ctx context.Context, receiver eventsapi.EventReceiver) <-chan error {
	return client.RunServerFunc(ctx, receiver)
}

type DummyApiSpecificAdapter struct {
	RunFunc func(_ context.Context, _ func(sarah.Input) error, _ func(error))
}

var _ apiSpecificAdapter = (*DummyApiSpecificAdapter)(nil)

func (d DummyApiSpecificAdapter) run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	d.RunFunc(ctx, enqueueInput, notifyErr)
}

func TestWithSlackClient(t *testing.T) {
	client := &DummyClient{}
	opt := WithSlackClient(client)
	adapter := &Adapter{}

	opt(adapter)

	if adapter.client == nil {
		t.Fatal("SlackClient is not set.")
	}

	if adapter.client != client {
		t.Fatal("Given SlackClient is not set.")
	}
}

func TestWithRTMPayloadHandler(t *testing.T) {
	fnc := func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {}
	opt := WithRTMPayloadHandler(fnc)
	adapter := &Adapter{}

	opt(adapter)

	if adapter.apiSpecificAdapterBuilder == nil {
		t.Fatal("apiSpecificAdapterBuilder is not set.")
	}

	if adapter.apiSpecificAdapterBuilder(nil, nil) == nil {
		t.Error("apiSpecificAdapter could not be built.")
	}
}

func TestWithEventsPayloadHandler(t *testing.T) {
	fnc := func(_ context.Context, _ *Config, _ *eventsapi.EventWrapper, _ func(sarah.Input) error) {}
	opt := WithEventsPayloadHandler(fnc)
	adapter := &Adapter{}

	opt(adapter)

	if adapter.apiSpecificAdapterBuilder == nil {
		t.Fatal("apiSpecificAdapterBuilder is not set.")
	}

	if adapter.apiSpecificAdapterBuilder(nil, nil) == nil {
		t.Error("apiSpecificAdapter could not be built.")
	}
}

func TestNewAdapter(t *testing.T) {
	t.Run("Minimum option", func(t *testing.T) {
		config := &Config{
			Token:          "dummy",
			RequestTimeout: time.Duration(10),
		}
		adapter, err := NewAdapter(config, WithEventsPayloadHandler(DefaultEventsPayloadHandler))

		if err != nil {
			t.Fatalf("Unexpected error is returned: %s.", err.Error())
		}

		if adapter.config != config {
			t.Errorf("Expected config struct is not set: %#v.", adapter.config)
		}

		if adapter.client == nil {
			t.Error("Golack client instance is not set.")
		}
	})

	t.Run("Missing config or SlackClient", func(t *testing.T) {
		config := &Config{}
		adapter, err := NewAdapter(config, WithRTMPayloadHandler(DefaultRTMPayloadHandler))

		if err == nil {
			t.Error("Expected error is not returned")
		}

		if adapter != nil {
			t.Fatal("Adapter should not be returned.")
		}
	})

	t.Run("Missing apiSpecificAdapter option", func(t *testing.T) {
		config := &Config{
			Token:          "dummy",
			RequestTimeout: time.Duration(10),
		}
		adapter, err := NewAdapter(config)

		if err == nil {
			t.Error("Expected error is not returned")
		}

		if adapter != nil {
			t.Fatal("Adapter should not be returned.")
		}
	})

	t.Run("With SlackClient", func(t *testing.T) {
		config := &Config{}
		client := &DummyClient{}
		opts := []AdapterOption{
			WithSlackClient(client),
			WithEventsPayloadHandler(DefaultEventsPayloadHandler),
		}

		adapter, err := NewAdapter(config, opts...)

		if err != nil {
			t.Fatalf("Unexpected error is returned: %s.", err.Error())
		}

		if adapter == nil {
			t.Fatal("Adapter should be returned.")
		}

		if adapter.client != client {
			t.Error("Provided SlackClient is not set.")
		}
	})
}

func TestAdapter_BotType(t *testing.T) {
	adapter := &Adapter{}

	if adapter.BotType() != SLACK {
		t.Errorf("Unexpected BotType is returned: %s.", adapter.BotType())
	}
}

func TestAdapter_Run(t *testing.T) {
	called := false
	adapter := &Adapter{
		apiSpecificAdapterBuilder: func(_ *Config, _ SlackClient) apiSpecificAdapter {
			return DummyApiSpecificAdapter{
				RunFunc: func(_ context.Context, _ func(sarah.Input) error, _ func(error)) {
					called = true
				},
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter.Run(ctx, func(_ sarah.Input) error { return nil }, func(err error) {})

	if !called {
		t.Error("apiSpecificAdapter is not run.")
	}
}

func TestAdapter_SendMessage(t *testing.T) {
	t.Run("Regular message", func(t *testing.T) {
		tests := []struct {
			channelID event.ChannelID
			err       error
			response  *webapi.APIResponse
		}{
			{
				channelID: "channelID",
				err:       nil,
				response: &webapi.APIResponse{
					OK:    true,
					Error: "",
				},
			},
			{
				channelID: "channelID",
				err:       errors.New("error"),
				response:  nil,
			},
			{
				channelID: "channelID",
				err:       nil,
				response: &webapi.APIResponse{
					OK:    false,
					Error: "error",
				},
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				called := false
				adapter := &Adapter{
					client: &DummyClient{
						PostMessageFunc: func(_ context.Context, _ *webapi.PostMessage) (*webapi.APIResponse, error) {
							called = true
							return tt.response, tt.err
						},
					},
				}

				postMessage := webapi.NewPostMessage(tt.channelID, "test")
				output := sarah.NewOutputMessage(tt.channelID, postMessage)
				adapter.SendMessage(context.TODO(), output)

				if !called {
					t.Fatal("Client.PostMessage is not called.")
				}
			})
		}
	})

	t.Run("String message", func(t *testing.T) {
		called := false
		adapter := &Adapter{
			client: &DummyClient{
				PostMessageFunc: func(_ context.Context, _ *webapi.PostMessage) (*webapi.APIResponse, error) {
					called = true
					return &webapi.APIResponse{
						OK:    true,
						Error: "",
					}, nil
				},
			},
		}

		output := sarah.NewOutputMessage(event.ChannelID("channel"), "message")
		adapter.SendMessage(context.TODO(), output)
		if !called {
			t.Fatal("Client.PostMessage is not called.")
		}
	})

	t.Run("Help command", func(t *testing.T) {
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

		adapter.SendMessage(context.TODO(), sarah.NewOutputMessage(event.ChannelID("test"), helps))
		if !called {
			t.Fatal("Client.PostMessage is not called.")
		}
	})
}

type DummyInput struct {
}

var _ sarah.Input = (*DummyInput)(nil)

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
	now := time.Now()
	tests := []struct {
		input   sarah.Input
		message string
		options []RespOption
		hasErr  bool
	}{
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
			},
			message: "dummy message",
			hasErr:  false,
		},
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
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
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
				timestamp: &event.TimeStamp{
					Time:          now,
					OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
				},
			},
			message: "dummy message",
			options: []RespOption{
				RespAsThreadReply(true),
			},
			hasErr: false,
		},
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
				timestamp: &event.TimeStamp{
					Time:          now,
					OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
				},
			},
			message: "dummy message",
			options: []RespOption{
				RespAsThreadReply(true),
				RespReplyBroadcast(true),
			},
			hasErr: false,
		},
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
				timestamp: &event.TimeStamp{
					Time:          now,
					OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
				},
				threadTimeStamp: &event.TimeStamp{
					Time:          now,
					OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
				},
			},
			message: "dummy message",
			options: []RespOption{},
			hasErr:  false,
		},
		{
			input: &Input{
				Event:     &event.Message{},
				channelID: "dummy",
				timestamp: &event.TimeStamp{
					Time:          time.Now(),
					OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
				},
				threadTimeStamp: &event.TimeStamp{
					Time:          time.Now(),
					OriginalValue: fmt.Sprintf("%d.999999", now.Unix()),
				},
			},
			message: "dummy message",
			options: []RespOption{},
			hasErr:  false,
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

			default:
				t.Errorf("Unexpected type of payload is returned: %T", typed)

			}
		})
	}
}

func TestRespAsThreadReply(t *testing.T) {
	options := &respOptions{}
	opt := RespAsThreadReply(true)

	opt(options)

	if options.asThreadReply == nil || !*options.asThreadReply {
		t.Fatal("Passed value is not set.")
	}
}

func TestRespReplyBroadcast(t *testing.T) {
	options := &respOptions{}
	opt := RespReplyBroadcast(true)

	opt(options)

	if !options.replyBroadcast {
		t.Fatal("Passed value is not set.")
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

func TestIsThreadMessage(t *testing.T) {
	now := time.Now()
	ts := &event.TimeStamp{
		Time:          now,
		OriginalValue: fmt.Sprintf("%d.123", now.Unix()),
	}
	tests := []struct {
		input    *Input
		expected bool
	}{
		{
			input: &Input{
				timestamp: &event.TimeStamp{
					OriginalValue: "1355517536.000001",
				},
			},
			expected: false,
		},
		{
			// A parent message
			input: &Input{
				threadTimeStamp: ts,
				timestamp:       ts,
			},
			expected: false,
		},
		{
			// A reply to a parent message, which is posted in a thread
			// https://api.slack.com/docs/message-threading
			input: &Input{
				threadTimeStamp: ts,
				timestamp: &event.TimeStamp{
					Time:          now,
					OriginalValue: fmt.Sprintf("%d.9999999999", now.Unix()),
				},
			},
			expected: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			isThread := IsThreadMessage(tt.input)
			if isThread != tt.expected {
				t.Errorf("Unexpected value is returned: %t", isThread)
			}
		})
	}
}

func Test_nonBlockSignal(t *testing.T) {
	// Prepare a channel with a buffer of 1.
	target := make(chan struct{}, 1)
	defer close(target)

	// Send twice. This exceed the target channel's cap, but the second call should not block.
	nonBlockSignal("DUMMY ID", target)
	nonBlockSignal("DUMMY ID", target)

	if len(target) != 1 {
		t.Errorf("The target channel should have exactly one signal: %d", len(target))
	}
}
