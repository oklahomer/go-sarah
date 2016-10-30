package slack

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/go-sarah/slack/rtmapi"
	"github.com/oklahomer/go-sarah/slack/webapi"
	"golang.org/x/net/context"
	"time"
)

const (
	// SLACK is a designated sara.BotType for Slack.
	SLACK sarah.BotType = "slack"
)

var pingSignalChannelId string = "ping"

// Adapter internally calls Slack Rest API and Real Time Messaging API to offer clients easy way to communicate with Slack.
//
// This implements sarah.Adapter interface, so this instance can be fed to sarah.Runner instance as below.
//
//  runner := sarah.NewRunner(sarah.NewConfig())
//  runner.RegisterAdapter(slack.NewAdapter(slack.NewConfig(token)), "/path/to/plugin/config.yml")
//  runner.Run()
type Adapter struct {
	config       *Config
	WebAPIClient *webapi.Client
	RtmAPIClient *rtmapi.Client
	messageQueue chan *textMessage
}

func NewAdapter(config *Config) *Adapter {
	return &Adapter{
		config:       config,
		WebAPIClient: webapi.NewClient(&webapi.Config{Token: config.token}),
		RtmAPIClient: rtmapi.NewClient(),
		messageQueue: make(chan *textMessage, config.sendingQueueSize),
	}
}

// BotType returns BotType of this particular instance.
func (adapter *Adapter) BotType() sarah.BotType {
	return SLACK
}

func (adapter *Adapter) Run(ctx context.Context, receivedMessage chan<- sarah.Input, errCh chan<- error) {
	for {
		conn, err := adapter.connect(ctx)
		if err != nil {
			errCh <- sarah.NewBotNonContinuableError(err.Error())
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

		go adapter.receivePayload(connCtx, conn, tryPing, receivedMessage)

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
		} else {
			log.Error(connErr.Error())
		}
	}

}

func (adapter *Adapter) superviseConnection(connCtx context.Context, payloadSender rtmapi.PayloadSender, tryPing chan struct{}) error {
	ticker := time.NewTicker(adapter.config.pingInterval)
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
			nonBlockSignal(pingSignalChannelId, tryPing)
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
	rtmInfo, err := fetchRtmInfo(ctx, adapter.WebAPIClient, adapter.config.retryLimit, adapter.config.retryInterval)
	if err != nil {
		return nil, err
	}

	return connectRtm(ctx, adapter.RtmAPIClient, rtmInfo, adapter.config.retryLimit, adapter.config.retryInterval)
}

func (adapter *Adapter) receivePayload(connCtx context.Context, payloadReceiver rtmapi.PayloadReceiver, tryPing chan<- struct{}, receivedMessage chan<- sarah.Input) {
	for {
		select {
		case <-connCtx.Done():
			log.Info("stop receiving payload due to context cancel")
			return
		default:
			payload, err := payloadReceiver.Receive()
			// TODO should io.EOF and io.ErrUnexpectedEOF treated differently than other errors?
			if err == rtmapi.EmptyPayloadError {
				continue
			} else if err == rtmapi.UnsupportedEventTypeError {
				continue
			} else if err != nil {
				// connection might not be stable or is closed already.
				log.Debugf("ping caused by '%s'", err.Error())
				nonBlockSignal(pingSignalChannelId, tryPing)
				continue
			}

			switch p := payload.(type) {
			case *rtmapi.WebSocketReply:
				if !*p.OK {
					// Something wrong with previous payload sending.
					log.Errorf("something was wrong with previous message sending. id: %d. text: %s.", p.ReplyTo, p.Text)
				}
			case *rtmapi.Message:
				receivedMessage <- p
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
		if _, err := adapter.WebAPIClient.PostMessage(ctx, message); err != nil {
			log.Error("something went wrong with Web API posting", err)
		}
	default:
		log.Warnf("unexpected output %#v", output)
	}
}

// fetchRtmInfo fetches Real Time Messaging API information via Rest API endpoint with retries.
func fetchRtmInfo(ctx context.Context, starter webapi.RtmStarter, retrial uint, interval time.Duration) (*webapi.RtmStart, error) {
	var rtmStart *webapi.RtmStart
	err := retry.RetryInterval(retrial, func() error {
		r, e := starter.RtmStart(ctx)
		rtmStart = r
		return e
	}, interval)

	return rtmStart, err
}

// connectRtm establishes WebSocket connection with retries.
func connectRtm(ctx context.Context, connector rtmapi.Connector, rtm *webapi.RtmStart, retrial uint, interval time.Duration) (rtmapi.Connection, error) {
	var conn rtmapi.Connection
	err := retry.RetryInterval(retrial, func() error {
		c, e := connector.Connect(ctx, rtm.URL)
		conn = c
		return e
	}, interval)

	return conn, err
}

// NewStringResponse creates new sarah.PluginResponse instance with given string.
func NewStringResponse(responseContent string) *sarah.PluginResponse {
	return NewStringResponseWithNext(responseContent, nil)
}

// NewStringResponseWithNext creates new sarah.PluginResponse instance with given string and next function to continue
func NewStringResponseWithNext(responseContent string, next sarah.ContextualFunc) *sarah.PluginResponse {
	return &sarah.PluginResponse{
		Content: responseContent,
		Next:    next,
	}
}

// NewPostMessageResponse can be used by plugin command to send message with customizable attachments.
func NewPostMessageResponse(input sarah.Input, message string, attachments []*webapi.MessageAttachment) *sarah.PluginResponse {
	return NewPostMessageResponseWithNext(input, message, attachments, nil)
}

// NewPostMessageResponseWithNext can be used by plugin command to send message with customizable attachments, and keep the user in the middle of conversation.
func NewPostMessageResponseWithNext(input sarah.Input, message string, attachments []*webapi.MessageAttachment, next sarah.ContextualFunc) *sarah.PluginResponse {
	inputMessage, _ := input.(*rtmapi.Message)
	return &sarah.PluginResponse{
		Content: webapi.NewPostMessageWithAttachments(inputMessage.Channel.Name, message, attachments),
		Next:    next,
	}
}
