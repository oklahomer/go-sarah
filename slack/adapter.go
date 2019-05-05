package slack

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/golack"
	"github.com/oklahomer/golack/rtmapi"
	"github.com/oklahomer/golack/slackobject"
	"github.com/oklahomer/golack/webapi"
	"golang.org/x/net/context"
	"strings"
	"time"
)

const (
	// SLACK is a designated sara.BotType for Slack.
	SLACK sarah.BotType = "slack"
)

var pingSignalChannelID = "ping"

// AdapterOption defines function signature that Adapter's functional option must satisfy.
type AdapterOption func(adapter *Adapter) error

// WithSlackClient creates AdapterOption with given SlackClient implementation.
// If this option is not given, NewAdapter() tries to create golack instance with given Config.
func WithSlackClient(client SlackClient) AdapterOption {
	return func(adapter *Adapter) error {
		adapter.client = client
		return nil
	}
}

// WithPayloadHandler creates AdapterOption with given function that is called when payload is sent from Slack via WebSocket connection.
//
// Slack's RTM API defines relatively large amount of payload types.
// To have better user experience, developers may provide customized callback function to handle received payload.
//
// Developer may wish to have direct access to SlackClient to post some sort of message to Slack via Web API.
// In that case, wrap this function like below so the SlackClient can be accessed within its scope.
//
//  // Setup golack instance, which implements SlackClient interface.
//  golackConfig := golack.NewConfig()
//  golackConfig.Token = "XXXXXXX"
//  slackClient := golack.New(golackConfig)
//
//  slackConfig := slack.NewConfig()
//  payloadHandler := func(connCtx context.Context, config *Config, paylad rtmapi.DecodedPayload, enqueueInput func(sarah.Input) error) {
//    switch p := payload.(type) {
//    case *rtmapi.PinAdded:
//      // Do something with pre-defined SlackClient
//      // slackClient.PostMessage(connCtx, ...)
//
//    case *rtmapi.Message:
//      // Convert RTM specific message to one that satisfies sarah.Input interface.
//      input := &MessageInput{event: p}
//
//      trimmed := strings.TrimSpace(input.Message())
//      if config.HelpCommand != "" && trimmed == config.HelpCommand {
//        // Help command
//        help := sarah.NewHelpInput(input.SenderKey(), input.Message(), input.SentAt(), input.ReplyTo())
//        enqueueInput(help)
//      } else if config.AbortCommand != "" && trimmed == config.AbortCommand {
//        // Abort command
//        abort := sarah.NewAbortInput(input.SenderKey(), input.Message(), input.SentAt(), input.ReplyTo())
//        enqueueInput(abort)
//      } else {
//        // Regular input
//        enqueueInput(input)
//      }
//
//    default:
//      log.Debugf("payload given, but no corresponding action is defined. %#v", p)
//
//    }
//  }
//
//  slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithSlackClient(slackClient), slack.WithPayloadHandler(payloadHandler))
//  slackBot, _ := sarah.NewBot(slackAdapter)
func WithPayloadHandler(fnc func(context.Context, *Config, rtmapi.DecodedPayload, func(sarah.Input) error)) AdapterOption {
	return func(adapter *Adapter) error {
		adapter.payloadHandler = fnc
		return nil
	}
}

// Adapter internally calls Slack Rest API and Real Time Messaging API to offer Bot developers easy way to communicate with Slack.
//
// This implements sarah.Adapter interface, so this instance can be fed to sarah.RegisterBot() as below.
//
//  slackConfig := slack.NewConfig()
//  slackConfig.Token = "XXXXXXXXXXXX" // Set token manually or feed slackConfig to json.Unmarshal or yaml.Unmarshal
//  slackAdapter, _ := slack.NewAdapter(slackConfig)
//  slackBot, _ := sarah.NewBot(slackAdapter)
//  sarah.RegisterBot(slackBot)
//
//  sarah.Run(context.TODO(), sarah.NewConfig())
type Adapter struct {
	config         *Config
	client         SlackClient
	messageQueue   chan *textMessage
	payloadHandler func(context.Context, *Config, rtmapi.DecodedPayload, func(sarah.Input) error)
}

// NewAdapter creates new Adapter with given *Config and zero or more AdapterOption.
func NewAdapter(config *Config, options ...AdapterOption) (*Adapter, error) {
	adapter := &Adapter{
		config:         config,
		messageQueue:   make(chan *textMessage, config.SendingQueueSize),
		payloadHandler: handlePayload, // may be replaced with WithPayloadHandler option.
	}

	for _, opt := range options {
		err := opt(adapter)
		if err != nil {
			return nil, err
		}
	}

	// See if client is set by WithSlackClient option.
	// If not, use golack with given configuration.
	if adapter.client == nil {
		if config.Token == "" {
			return nil, errors.New("Slack client must be provided with WithSlackClient option or must be configurable with given *Config.")
		}

		golackConfig := golack.NewConfig()
		golackConfig.Token = config.Token
		if config.RequestTimeout != 0 {
			golackConfig.RequestTimeout = config.RequestTimeout
		}

		adapter.client = golack.New(golackConfig)
	}

	return adapter, nil
}

// BotType returns BotType of this particular instance.
func (adapter *Adapter) BotType() sarah.BotType {
	return SLACK
}

// Run establishes connection with Slack, supervise it, and tries to reconnect when current connection is gone.
// Connection will be
//
// When message is sent from slack server, the payload is passed to go-sarah's core via the function given as 2nd argument, enqueueInput.
// This function simply wraps a channel to prevent blocking situation. When workers are too busy and channel blocks, this function returns BlockedInputError.
//
// When critical situation such as reconnection trial fails for specified times, this critical situation is notified to go-sarah's core via 3rd argument function, notifyErr.
// go-sarah cancels this Bot/Adapter and related resources when BotNonContinuableError is given to this function.
func (adapter *Adapter) Run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	for {
		conn, err := adapter.connect(ctx)
		if err != nil {
			notifyErr(sarah.NewBotNonContinuableError(err.Error()))
			return
		}

		// Create connection specific context so each connection-scoped goroutine can receive connection closing event and eventually return.
		connCtx, connCancel := context.WithCancel(ctx)

		// This channel is not subject to close. This channel can be accessed in parallel manner with nonBlockSignal(),
		// and the receiver is NOT looking for close signal. Let GC run when this channel is no longer referred.
		//
		// http://stackoverflow.com/a/8593986
		// "Note that it is only necessary to close a channel if the receiver is looking for a close.
		// Closing the channel is a control signal on the channel indicating that no more data follows."
		tryPing := make(chan struct{}, 1)

		go adapter.receivePayload(connCtx, conn, tryPing, enqueueInput)

		// payload reception and other connection-related tasks must run in separate goroutines since receivePayload()
		// internally blocks til entire payload is being read and iterates it over and over.
		connErr := adapter.superviseConnection(connCtx, conn, tryPing)

		// superviseConnection returns when parent context is canceled or connection is hopelessly unstable
		// close current connection and do some cleanup
		_ = conn.Close() // TODO may return net.OpError with "use of closed network connection" if called with closed connection
		connCancel()
		if connErr == nil {
			// Connection is intentionally closed by caller.
			// No more interaction follows.
			return
		}

		log.Errorf("Will try re-connection due to previous connection's fatal state: %s.", connErr.Error())
	}
}

func (adapter *Adapter) superviseConnection(connCtx context.Context, payloadSender rtmapi.PayloadSender, tryPing chan struct{}) error {
	ticker := time.NewTicker(adapter.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			return nil

		case message := <-adapter.messageQueue:
			if err := payloadSender.Send(message.channel, message.text); err != nil {
				// Try ping right away when Send() returns error so that following messages stay in the queue
				// while connection status is checked with ping message and optionally reconnect
				if pingErr := payloadSender.Ping(); pingErr != nil {
					// Reconnection requested.
					return fmt.Errorf("error on ping: %s", pingErr.Error())
				}
			}

		case <-ticker.C:
			nonBlockSignal(pingSignalChannelID, tryPing)

		case <-tryPing:
			log.Debug("Send ping")
			if err := payloadSender.Ping(); err != nil {
				return fmt.Errorf("error on ping: %s", err.Error())
			}

		}
	}
}

// connect fetches WebSocket endpoint information via Rest API and establishes WebSocket connection.
func (adapter *Adapter) connect(ctx context.Context) (rtmapi.Connection, error) {
	// Get RTM session via Web API.
	var rtmStart *webapi.RTMStart
	err := retry.WithPolicy(adapter.config.RetryPolicy, func() (e error) {
		rtmStart, e = adapter.client.StartRTMSession(ctx)
		return e
	})
	if err != nil {
		return nil, err
	}

	// Establish WebSocket connection with given RTM session.
	var conn rtmapi.Connection
	err = retry.WithPolicy(adapter.config.RetryPolicy, func() (e error) {
		conn, e = adapter.client.ConnectRTM(ctx, rtmStart.URL)
		return e
	})

	return conn, err
}

func (adapter *Adapter) receivePayload(connCtx context.Context, payloadReceiver rtmapi.PayloadReceiver, tryPing chan<- struct{}, enqueueInput func(sarah.Input) error) {
	for {
		select {
		case <-connCtx.Done():
			log.Info("Stop receiving payload due to context cancel")
			return

		default:
			payload, err := payloadReceiver.Receive()
			// TODO should io.EOF and io.ErrUnexpectedEOF treated differently than other errors?
			if err == rtmapi.ErrEmptyPayload {
				continue
			} else if _, ok := err.(*rtmapi.MalformedPayloadError); ok {
				// Malformed payload was passed, but there is no programmable way to handle this error.
				// Leave log and proceed.
				log.Warnf("Ignore malformed payload: %s.", err.Error())
			} else if _, ok := err.(*rtmapi.UnexpectedMessageTypeError); ok {
				log.Warnf("Ignore a payload with unexpected message type: %s.", err.Error())
			} else if err != nil {
				// Connection might not be stable or is closed already.
				log.Debugf("Ping caused by '%s'", err.Error())
				nonBlockSignal(pingSignalChannelID, tryPing)
				continue
			}

			if payload == nil {
				continue
			}

			adapter.payloadHandler(connCtx, adapter.config, payload, enqueueInput)
		}
	}
}

func handlePayload(_ context.Context, config *Config, payload rtmapi.DecodedPayload, enqueueInput func(sarah.Input) error) {
	switch p := payload.(type) {
	case *rtmapi.WebSocketOKReply:
		log.Debugf("Successfully sent. ID: %d. Text: %s.", p.ReplyTo, p.Text)

	case *rtmapi.WebSocketNGReply:
		log.Errorf(
			"Something was wrong with previous message sending. id: %d. error code: %d. error message: %s.",
			p.ReplyTo, p.ErrorReason.Code, p.ErrorReason.Message)

	case *rtmapi.Pong:
		log.Debug("Pong message received.")

	case *rtmapi.Message:
		// Convert RTM specific message to one that satisfies sarah.Input interface.
		input := NewMessageInput(p)

		trimmed := strings.TrimSpace(input.Message())
		if config.HelpCommand != "" && trimmed == config.HelpCommand {
			// Help command
			help := sarah.NewHelpInput(input.SenderKey(), input.Message(), input.SentAt(), input.ReplyTo())
			_ = enqueueInput(help)
		} else if config.AbortCommand != "" && trimmed == config.AbortCommand {
			// Abort command
			abort := sarah.NewAbortInput(input.SenderKey(), input.Message(), input.SentAt(), input.ReplyTo())
			_ = enqueueInput(abort)
		} else {
			// Regular input
			_ = enqueueInput(input)
		}

	default:
		log.Debugf("Payload given, but no corresponding action is defined. %#v", p)

	}
}

// nonBlockSignal tries to send signal to given channel.
// If no goroutine is listening to the channel or is working on a task triggered by previous signal, this method skips
// signalling rather than blocks til somebody is ready to read channel.
//
// For signalling purpose, empty struct{} should be used.
// http://peter.bourgon.org/go-in-production/
//  "Use struct{} as a sentinel value, rather than bool or interface{}. For example, (snip) a signal channel is chan struct{}.
//  It unambiguously signals an explicit lack of information."
func nonBlockSignal(id string, target chan<- struct{}) {
	select {
	case target <- struct{}{}:
		// O.K

	default:
		// couldn't send because no goroutine is receiving channel or is busy.
		log.Infof("not sending signal to channel: %s", id)

	}
}

type textMessage struct {
	channel slackobject.ChannelID
	text    string
}

// SendMessage let Bot send message to Slack.
func (adapter *Adapter) SendMessage(ctx context.Context, output sarah.Output) {
	switch content := output.Content().(type) {
	case string:
		channel, ok := output.Destination().(slackobject.ChannelID)
		if !ok {
			log.Errorf("Destination is not instance of Channel. %#v.", output.Destination())
			return
		}

		adapter.messageQueue <- &textMessage{
			channel: channel,
			text:    content,
		}

	case *webapi.PostMessage:
		message := output.Content().(*webapi.PostMessage)
		if _, err := adapter.client.PostMessage(ctx, message); err != nil {
			log.Error("something went wrong with Web API posting", err)
		}

	case *sarah.CommandHelps:
		channelID, ok := output.Destination().(slackobject.ChannelID)
		if !ok {
			log.Errorf("Destination is not instance of Channel. %#v.", output.Destination())
			return
		}

		fields := []*webapi.AttachmentField{}
		for _, commandHelp := range *output.Content().(*sarah.CommandHelps) {
			fields = append(fields, &webapi.AttachmentField{
				Title: commandHelp.Identifier,
				Value: commandHelp.InputExample,
				Short: false,
			})
		}
		attachments := []*webapi.MessageAttachment{
			{
				Fallback: "Here are some input examples.", // TODO
				Pretext:  "Help:",
				Title:    "",
				Fields:   fields,
			},
		}
		postMessage := webapi.NewPostMessageWithAttachments(channelID, "", attachments)

		if _, err := adapter.client.PostMessage(ctx, postMessage); err != nil {
			log.Error("something went wrong with Web API posting", err)
		}

	default:
		log.Warnf("unexpected output %#v", output)

	}
}

// MessageInput satisfies Input interface
type MessageInput struct {
	event *rtmapi.Message
}

// SenderKey returns string representing message sender.
func (message *MessageInput) SenderKey() string {
	return fmt.Sprintf("%s|%s", message.event.ChannelID.String(), message.event.Sender.String())
}

// Message returns sent message.
func (message *MessageInput) Message() string {
	return message.event.Text
}

// SentAt returns message event's timestamp.
func (message *MessageInput) SentAt() time.Time {
	return message.event.TimeStamp.Time
}

// ReplyTo returns slack channel to send reply to.
func (message *MessageInput) ReplyTo() sarah.OutputDestination {
	return message.event.ChannelID
}

// NewMessageInput creates and returns MessageInput instance.
func NewMessageInput(message *rtmapi.Message) *MessageInput {
	return &MessageInput{
		event: message,
	}
}

// NewStringResponse creates new sarah.CommandResponse instance with given string.
func NewStringResponse(responseContent string) *sarah.CommandResponse {
	return &sarah.CommandResponse{
		Content:     responseContent,
		UserContext: nil,
	}
}

// NewStringResponseWithNext creates new sarah.CommandResponse instance with given string and next function to continue
//
// With this method user context is directly stored as an anonymous function since Slack Bot works with single WebSocket connection and hence usually works with single process.
// To use external storage to store user context, use go-sarah-rediscontext or similar sarah.UserContextStorage implementation.
func NewStringResponseWithNext(responseContent string, next sarah.ContextualFunc) *sarah.CommandResponse {
	return &sarah.CommandResponse{
		Content:     responseContent,
		UserContext: sarah.NewUserContext(next),
	}
}

// NewPostMessageResponse can be used by plugin command to send message with customizable attachments.
// Use NewStringResponse for simple text response.
func NewPostMessageResponse(input sarah.Input, message string, attachments []*webapi.MessageAttachment) *sarah.CommandResponse {
	inputMessage, _ := input.(*MessageInput)
	return &sarah.CommandResponse{
		Content:     webapi.NewPostMessageWithAttachments(inputMessage.event.ChannelID, message, attachments),
		UserContext: nil,
	}
}

// NewPostMessageResponseWithNext can be used by plugin command to send message with customizable attachments, and keep the user in the middle of conversation.
// Use NewStringResponse for simple text response.
//
// With this method user context is directly stored as an anonymous function since Slack Bot works with single WebSocket connection and hence usually works with single process.
// To use external storage to store user context, use go-sarah-rediscontext or similar sarah.UserContextStorage implementation.
func NewPostMessageResponseWithNext(input sarah.Input, message string, attachments []*webapi.MessageAttachment, next sarah.ContextualFunc) *sarah.CommandResponse {
	inputMessage, _ := input.(*MessageInput)
	return &sarah.CommandResponse{
		Content:     webapi.NewPostMessageWithAttachments(inputMessage.event.ChannelID, message, attachments),
		UserContext: sarah.NewUserContext(next),
	}
}

// SlackClient is an interface that covers golack's public methods.
type SlackClient interface {
	StartRTMSession(context.Context) (*webapi.RTMStart, error)
	ConnectRTM(context.Context, string) (rtmapi.Connection, error)
	PostMessage(context.Context, *webapi.PostMessage) (*webapi.APIResponse, error)
}
