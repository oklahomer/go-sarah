package slack

import (
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/golack"
	"github.com/oklahomer/golack/rtmapi"
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

// Adapter internally calls Slack Rest API and Real Time Messaging API to offer clients easy way to communicate with Slack.
//
// This implements sarah.Adapter interface, so this instance can be fed to sarah.Runner instance as below.
//
//  runner := sarah.NewRunner(sarah.NewConfig())
//  runner.RegisterAdapter(slack.NewAdapter(slack.NewConfig(token)), "/path/to/plugin/config.yml")
//  runner.Run()
type Adapter struct {
	config       *Config
	client       *golack.Golack
	messageQueue chan *textMessage
}

// NewAdapter creates new Adapter with given *Config, and returns it.
func NewAdapter(config *Config) *Adapter {
	golackConfig := golack.NewConfig()
	golackConfig.Token = config.Token
	golackConfig.RequestTimeout = config.RequestTimeout

	return &Adapter{
		config:       config,
		client:       golack.New(golackConfig),
		messageQueue: make(chan *textMessage, config.SendingQueueSize),
	}
}

// BotType returns BotType of this particular instance.
func (adapter *Adapter) BotType() sarah.BotType {
	return SLACK
}

// Run establishes connection with Slack, supervise it, and tries to reconnect when current connection is gone.
// Connection will be
//
// When message is sent from slack server, the payload is passed to Runner via the function given as 2nd argument, enqueueInput.
// This function simply wraps a channel to prevent blocking situation. When workers are too busy and channel blocks, this function returns BlockedInputError.
//
// When critical situation such as reconnection trial fails for specified times, this critical situation is notified to Runner via 3rd argument function, notifyErr.
// Runner cancels this Bot/Adapter and related resources when BotNonContinuableError is given to this function.
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
		conn.Close() // TODO may return net.OpError with "use of closed network connection" if called with closed connection
		connCancel()
		if connErr == nil {
			// Connection is intentionally closed by caller.
			// No more interaction follows.
			return
		}
		log.Error(connErr.Error())
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
					return pingErr
				}
			}
		case <-ticker.C:
			nonBlockSignal(pingSignalChannelID, tryPing)
		case <-tryPing:
			log.Debug("send ping")
			if err := payloadSender.Ping(); err != nil {
				log.Errorf("error on ping: %#v.", err.Error())
				return err
			}
		}
	}
}

// connect fetches WebSocket endpoint information via Rest API and establishes WebSocket connection.
func (adapter *Adapter) connect(ctx context.Context) (rtmapi.Connection, error) {
	rtmSession, err := startRTMSession(ctx, adapter.client, adapter.config.RetryLimit, adapter.config.RetryInterval)
	if err != nil {
		return nil, err
	}

	return connectRTM(ctx, adapter.client, rtmSession, adapter.config.RetryLimit, adapter.config.RetryInterval)
}

func (adapter *Adapter) receivePayload(connCtx context.Context, payloadReceiver rtmapi.PayloadReceiver, tryPing chan<- struct{}, enqueueInput func(sarah.Input) error) {
	for {
		select {
		case <-connCtx.Done():
			log.Info("stop receiving payload due to context cancel")
			return
		default:
			payload, err := payloadReceiver.Receive()
			// TODO should io.EOF and io.ErrUnexpectedEOF treated differently than other errors?
			if err == rtmapi.ErrEmptyPayload {
				continue
			} else if _, ok := err.(*rtmapi.MalformedPayloadError); ok {
				// Malformed payload was passed, but there is no programmable way to handle this error.
				// Leave log and proceed.
				log.Warnf("ignoring malformed paylaod: %s.", err.Error())
			} else if err != nil {
				// Connection might not be stable or is closed already.
				log.Debugf("ping caused by '%s'", err.Error())
				nonBlockSignal(pingSignalChannelID, tryPing)
				continue
			}

			switch p := payload.(type) {
			case *rtmapi.WebSocketReply:
				if !p.OK {
					log.Errorf("something was wrong with previous message sending. id: %d. text: %s.", p.ReplyTo, p.Text)
				}
			case *rtmapi.Message:
				trimmed := strings.TrimSpace(p.Text)
				if adapter.config.HelpCommand != "" && trimmed == adapter.config.HelpCommand {
					// Help command
					help := sarah.NewHelpInput(p.Sender, p.Text, p.TimeStamp.Time, p.Channel)
					enqueueInput(help)
				} else if adapter.config.AbortCommand != "" && trimmed == adapter.config.AbortCommand {
					// Abort command
					abort := sarah.NewAbortInput(p.Sender, p.Text, p.TimeStamp.Time, p.Channel)
					enqueueInput(abort)
				} else {
					// Regular input
					enqueueInput(&MessageInput{event: p})
				}
			case *rtmapi.Pong:
				continue
			case nil:
				continue
			default:
				log.Debugf("payload given, but no corresponding action is defined. %#v", p)
			}
		}
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
	default:
		// couldn't send because no goroutine is receiving channel or is busy.
		log.Infof("not sending signal to channel: %s", id)
	}
}

type textMessage struct {
	channel *rtmapi.Channel
	text    string
}

// SendMessage let Bot send message to Slack.
func (adapter *Adapter) SendMessage(ctx context.Context, output sarah.Output) {
	switch content := output.Content().(type) {
	case string:
		channel, ok := output.Destination().(*rtmapi.Channel)
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
		channel, ok := output.Destination().(*rtmapi.Channel)
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
		postMessage := webapi.NewPostMessageWithAttachments(channel.Name, "", attachments)

		if _, err := adapter.client.PostMessage(ctx, postMessage); err != nil {
			log.Error("something went wrong with Web API posting", err)
		}

	default:
		log.Warnf("unexpected output %#v", output)
	}
}

// RTMSessionStarter is an interface that is used to ease web API tests with retrials.
type RTMSessionStarter interface {
	StartRTMSession(context.Context) (*webapi.RTMStart, error)
}

// startRTMSession starts Real Time Messaging session and returns initial state.
func startRTMSession(ctx context.Context, starter RTMSessionStarter, retrial uint, interval time.Duration) (*webapi.RTMStart, error) {
	var rtmStart *webapi.RTMStart
	err := retry.WithInterval(retrial, func() (e error) {
		rtmStart, e = starter.StartRTMSession(ctx)
		return e
	}, interval)

	return rtmStart, err
}

// RTMConnector is an interface that is used to ease connection tests with retrials
type RTMConnector interface {
	ConnectRTM(context.Context, string) (rtmapi.Connection, error)
}

// connectRTM establishes WebSocket connection with retries.
func connectRTM(ctx context.Context, connector RTMConnector, rtm *webapi.RTMStart, retrial uint, interval time.Duration) (rtmapi.Connection, error) {
	var conn rtmapi.Connection
	err := retry.WithInterval(retrial, func() (e error) {
		conn, e = connector.ConnectRTM(ctx, rtm.URL)
		return e
	}, interval)

	return conn, err
}

// MessageInput satisfies Input interface
type MessageInput struct {
	event *rtmapi.Message
}

// SenderKey returns string representing message sender.
func (message *MessageInput) SenderKey() string {
	return fmt.Sprintf("%s|%s", message.event.Channel.Name, message.event.Sender)
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
	return message.event.Channel
}

// NewStringResponse creates new sarah.CommandResponse instance with given string.
func NewStringResponse(responseContent string) *sarah.CommandResponse {
	return NewStringResponseWithNext(responseContent, nil)
}

// NewStringResponseWithNext creates new sarah.CommandResponse instance with given string and next function to continue
func NewStringResponseWithNext(responseContent string, next sarah.ContextualFunc) *sarah.CommandResponse {
	return &sarah.CommandResponse{
		Content: responseContent,
		Next:    next,
	}
}

// NewPostMessageResponse can be used by plugin command to send message with customizable attachments.
// Use NewStringResponse for simple text response.
func NewPostMessageResponse(input sarah.Input, message string, attachments []*webapi.MessageAttachment) *sarah.CommandResponse {
	return NewPostMessageResponseWithNext(input, message, attachments, nil)
}

// NewPostMessageResponseWithNext can be used by plugin command to send message with customizable attachments, and keep the user in the middle of conversation.
// Use NewStringResponse for simple text response.
func NewPostMessageResponseWithNext(input sarah.Input, message string, attachments []*webapi.MessageAttachment, next sarah.ContextualFunc) *sarah.CommandResponse {
	inputMessage, _ := input.(*MessageInput)
	return &sarah.CommandResponse{
		Content: webapi.NewPostMessageWithAttachments(inputMessage.event.Channel.Name, message, attachments),
		Next:    next,
	}
}
