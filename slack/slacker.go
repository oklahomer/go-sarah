package slack

import (
	"github.com/Sirupsen/logrus"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/go-sarah/slack/common"
	"github.com/oklahomer/go-sarah/slack/rtmapi"
	"github.com/oklahomer/go-sarah/slack/webapi"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
	"io"
	"reflect"
	"time"
)

const (
	// SLACK is a designated sara.BotType for Slacker.
	SLACK sarah.BotType = "slack"
)

/*
Slacker internally calls Slack Rest API and Real Time Messaging API to offer clients easy way to communicate with Slack.

This implements sarah.BotAdapter interface, so this instance can be fed to sarah.Bot instance as below.

  bot := sarah.NewBot()
  bot.AddAdapter(NewSlacker("myToken"))
  bot.Run()
*/
type Slacker struct {
	// Clients that directly communicate with slack API
	WebAPIClient *webapi.Client
	RtmAPIClient *rtmapi.Client

	// A channel that receive outgoing messages that are sent via RTM WebSocket connection.
	// BEWARE: RTM messages are queued and handled in a serial manner because of the limitation of single connection;
	// WebAPI messages can be sent in multiple goroutines with each dedicated HTTP requests.
	OutgoingRtmMessages chan *rtmapi.TextMessage

	// An instance that dispense unique id for outgoing messages.
	// IDs must be unique per-connection.
	outgoingEventID *rtmapi.OutgoingEventID

	// Some channels to handle its life-cycle.
	startNewRtm chan struct{}
	tryPing     chan struct{}

	// WebSocket connection that is set and updated on each connection (re)establishment.
	webSocketConnection *websocket.Conn
}

type SendingMessage struct {
	channel *common.Channel
}

func (message *SendingMessage) Destination() *common.Channel {
	return message.channel
}

func (message *SendingMessage) Content() interface{} {
	return "TODO"
}

// NewSlacker creates new Slacker instance with given settings.
func NewSlacker(token string) *Slacker {
	return &Slacker{
		WebAPIClient:        webapi.NewClient(&webapi.Config{Token: token}),
		RtmAPIClient:        rtmapi.NewClient(),
		OutgoingRtmMessages: make(chan *rtmapi.TextMessage, 100),
		outgoingEventID:     rtmapi.NewOutgoingEventID(),
		startNewRtm:         make(chan struct{}),
		tryPing:             make(chan struct{}),
	}
}

// BotType returns BotType of this particular instance.
func (slacker *Slacker) BotType() sarah.BotType {
	return SLACK
}

// Run starts Slack interaction including WebSocket connection management and message receiving/sending.
func (slacker *Slacker) Run(ctx context.Context, receiver chan<- sarah.BotInput, errCh chan<- error) {
	go slacker.supervise(ctx, errCh)
	go slacker.sendEnqueuedMessage(ctx)
	go slacker.receiveEvent(ctx, receiver)

	slacker.startNewRtm <- struct{}{}
}

/*
supervise takes care of Slacker instance's life cycle-related channels.

This supervises receives channels and triggers some tasks below:
 - establish new WebSocket connection
 - stop Slack interaction
 - check connection status with ping
*/
func (slacker *Slacker) supervise(ctx context.Context, errCh chan<- error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-slacker.startNewRtm:
			// make sure current connection is closed.
			slacker.disconnect()

			// establish new connection.
			if err := slacker.connect(ctx); err != nil {
				// can't establish connection with retrial.
				// notify critical condition and let BotRunner stop slacker.
				logrus.Errorf("can't establish connection. %s", err.Error())
				errCh <- sarah.NewBotAdapterNonContinuableError(err.Error())
			}
		case <-ctx.Done():
			logrus.Info("disconnect WebSocket connection due to context cancel")
			slacker.disconnect()
			return
		case <-ticker.C:
			slacker.checkConnection()
		case <-slacker.tryPing:
			slacker.checkConnection()
		}
	}
}

// connect fetches WebSocket endpoint information via Rest API and establishes WebSocket connection.
func (slacker *Slacker) connect(ctx context.Context) error {
	rtmInfo, err := fetchRtmInfo(ctx, slacker.WebAPIClient)
	if err != nil {
		return err
	}

	conn, err := connectRtm(ctx, slacker.RtmAPIClient, rtmInfo)
	if err != nil {
		return err
	}

	slacker.webSocketConnection = conn
	return nil
}

/*
disconnect disconnects existing WebSocket connection. To avoid race condition, this method should only be called from supervise method.
*/
func (slacker *Slacker) disconnect() {
	if slacker.webSocketConnection == nil {
		return
	}

	if err := slacker.webSocketConnection.Close(); err != nil {
		logrus.Errorf("error on connection close. type %T. value: %+v.", err, err)
	}
}

/*
receiveEvent receives payloads via WebSocket connection, decodes them into pre-defined events, and passes them via
channel if they satisfy sarah.BotInput interface.
*/
func (slacker *Slacker) receiveEvent(ctx context.Context, inputReceiver chan<- sarah.BotInput) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("stop receiving events due to context cancel")
			return
		default:
			if slacker.webSocketConnection == nil {
				// May reach during connection (re)establishment
				continue
			}

			// Blocking method to receive payload from WebSocket connection.
			// When connection is closed in the middle of this method call, this immediately returns error.
			payload, err := rtmapi.ReceivePayload(slacker.webSocketConnection)
			if err == io.EOF {
				slacker.tryPing <- struct{}{}
				continue
			} else if err != nil {
				logrus.Error("error on receiving payload", reflect.TypeOf(err), err.Error())
				continue
			}

			event, err := slacker.RtmAPIClient.DecodePayload(payload)
			if err != nil {
				switch err.(type) {
				case *rtmapi.MalformedEventTypeError:
					logrus.Warnf("malformed payload was passed. %s", string(payload))
				case *rtmapi.ReplyStatusError:
					logrus.Errorf("something was wrong with previous posted message. %#v", err)
				default:
					logrus.Errorf("unhandled error occured on payload decode. %#v", err)
				}
			}

			if event == nil {
				continue
			}

			if botInput, ok := event.(sarah.BotInput); ok {
				inputReceiver <- botInput
			} else {
				// Miscellaneous events to support operation
				logrus.Debugf("received non-message event. %#v.", event)
			}
		}
	}
}

func (slacker *Slacker) SendMessage(ctx context.Context, output sarah.BotOutput) {
	switch content := output.Content().(type) {
	case string:
		channel, ok := output.Destination().(*common.Channel)
		if !ok {
			logrus.Error("Destination is not instance of Channel")
			return
		}
		sendingMessage := rtmapi.NewTextMessage(channel, content)
		slacker.OutgoingRtmMessages <- sendingMessage
	case *webapi.PostMessage:
		message := output.Content().(*webapi.PostMessage)
		if _, err := slacker.WebAPIClient.PostMessage(ctx, message); err != nil {
			logrus.Error("something went wrong with Web API posting", err)
		}
	default:
		logrus.Warnf("unexpected output %v", reflect.TypeOf(output).Name())
	}
}

/*
sendEnqueuedMessage receives messages via Slacker.OutgoingMessages, and send them over WebSocket connection.
This method is meant to be run in a single goroutine per Slacker instance.

Message sending is done in a serial manner one after another, so don't worry about sending multiple payloads over
WebSocket connection simultaneously. websocket.JSON.Send itself is goroutine-safe, though.
*/
func (slacker *Slacker) sendEnqueuedMessage(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("stop sending message context cancel")
			return
		case message := <-slacker.OutgoingRtmMessages:
			if slacker.webSocketConnection == nil {
				continue
			}

			event := rtmapi.NewOutgoingMessage(slacker.outgoingEventID, message)
			if err := websocket.JSON.Send(slacker.webSocketConnection, event); err != nil {
				logrus.Error("failed to send event", event)
			}
		}
	}
}

/*
checkConnection checks connection status by sending ping over existing WebSocket connection.
If the connection seems stale, this tells supervising method to re-connect via corresponding channel.
*/
func (slacker *Slacker) checkConnection() {
	logrus.Debug("checking connection status with Ping payload.")
	ping := rtmapi.NewPing(slacker.outgoingEventID)
	if err := websocket.JSON.Send(slacker.webSocketConnection, ping); err != nil {
		logrus.Error("failed sending Ping payload", err)
		slacker.startNewRtm <- struct{}{}
	}
}

// fetchRtmInfo fetches Real Time Messaging API information via Rest API endpoint with retries.
func fetchRtmInfo(ctx context.Context, client *webapi.Client) (*webapi.RtmStart, error) {
	var rtmStart *webapi.RtmStart
	err := retry.RetryInterval(10, func() error {
		r, e := client.RtmStart(ctx)
		rtmStart = r
		return e
	}, 500*time.Millisecond)

	return rtmStart, err
}

// connectRtm establishes WebSocket connection with retries.
func connectRtm(ctx context.Context, client *rtmapi.Client, rtm *webapi.RtmStart) (*websocket.Conn, error) {
	var conn *websocket.Conn
	err := retry.RetryInterval(10, func() error {
		c, e := client.Connect(ctx, rtm.URL)
		conn = c
		return e
	}, 500*time.Millisecond)

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

func NewPostMessageResponse(input sarah.BotInput, message string, attachments []*webapi.MessageAttachment) *sarah.PluginResponse {
	return NewPostMessageResponseWithNext(input, message, attachments, nil)
}

func NewPostMessageResponseWithNext(input sarah.BotInput, message string, attachments []*webapi.MessageAttachment, next sarah.ContextualFunc) *sarah.PluginResponse {
	inputMessage, _ := input.(*rtmapi.Message)
	return &sarah.PluginResponse{
		Content: webapi.NewPostMessageWithAttachments(inputMessage.Channel.Name, message, attachments),
		Next:    next,
	}
}
