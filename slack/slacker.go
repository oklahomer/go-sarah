package slack

import (
	"github.com/Sirupsen/logrus"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/retry"
	"github.com/oklahomer/go-sarah/slack/rtmapi"
	"github.com/oklahomer/go-sarah/slack/webapi"
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

	// A channel that receive outgoing messages.
	OutgoingMessages chan *rtmapi.TextMessage

	// An instance that dispense unique id for outgoing messages.
	// IDs must be unique per-connection.
	outgoingEventID *rtmapi.OutgoingEventID

	// Some channels to handle its life-cycle.
	startNewRtm chan struct{}
	tryPing     chan struct{}
	stopper     chan struct{}
	stopAll     chan struct{}

	// WebSocket connection that is set and updated on each connection (re)establishment.
	webSocketConnection *websocket.Conn
}

// NewSlacker creates new Slacker instance with given settings.
func NewSlacker(token string) *Slacker {
	return &Slacker{
		WebAPIClient:     webapi.NewClient(&webapi.Config{Token: token}),
		RtmAPIClient:     rtmapi.NewClient(),
		OutgoingMessages: make(chan *rtmapi.TextMessage, 100),
		outgoingEventID:  rtmapi.NewOutgoingEventID(),
		startNewRtm:      make(chan struct{}),
		tryPing:          make(chan struct{}),
		stopper:          make(chan struct{}),
		stopAll:          make(chan struct{}),
	}
}

// GetBotType returns BotType of this particular instance.
func (slacker *Slacker) GetBotType() sarah.BotType {
	return SLACK
}

// Run starts Slack interaction including WebSocket connection management and message receiving/sending.
func (slacker *Slacker) Run(receiver chan<- sarah.BotInput) {
	go slacker.supervise()
	go slacker.sendEnqueuedMessage()
	go slacker.receiveEvent(receiver)

	slacker.startNewRtm <- struct{}{}
}

// Stop stops Slack interaction and cleans up all belonging goroutines.
func (slacker *Slacker) Stop() {
	slacker.stopper <- struct{}{}
}

/*
supervise takes care of Slacker instance's life cycle-related channels.

This supervises receives channels and triggers some tasks below:
 - establish new WebSocket connection
 - stop Slack interaction
 - check connection status with ping
*/
func (slacker *Slacker) supervise() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-slacker.startNewRtm:
			slacker.disconnect()
			if err := slacker.connect(); err != nil {
				logrus.Errorf("can't establish connection. %s", err.Error())
				slacker.Stop()
			}
		case <-slacker.stopper:
			close(slacker.stopAll)
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
func (slacker *Slacker) connect() error {
	rtmInfo, err := fetchRtmInfo(slacker.WebAPIClient)
	if err != nil {
		return err
	}

	conn, err := connectRtm(slacker.RtmAPIClient, rtmInfo)
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
func (slacker *Slacker) receiveEvent(receiver chan<- sarah.BotInput) {
	for {
		select {
		case <-slacker.stopAll:
			logrus.Info("stop receiving events due to stop queue")
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
				receiver <- botInput
			} else {
				// Miscellaneous events to support operation
			}
		}
	}
}

/*
SendResponse sends message to Slack via Rest API or existing WebSocket connection depending on what message type is
given; Rest API for *webapi.PostMessage, WebSocket connection for string.
*/
func (slacker *Slacker) SendResponse(response *sarah.CommandResponse) {
	switch content := response.ResponseContent.(type) {
	case string:
		sendingMessage := rtmapi.NewTextMessage(response.Input.GetRoomID(), content)
		slacker.OutgoingMessages <- sendingMessage
	case *webapi.PostMessage:
		message := response.ResponseContent.(*webapi.PostMessage)
		if _, err := slacker.WebAPIClient.PostMessage(message); err != nil {
			logrus.Error("something went wrong with Web API posting", err)
		}
	default:
		logrus.Warnf("unexpected command response %v", reflect.TypeOf(response).Name())
	}
}

func (slacker *Slacker) SendMessage(message *sarah.Message) {
	switch content := message.Content.(type) {
	case string:
		sendingMessage := rtmapi.NewTextMessage(message.GetRoomID(), content)
		slacker.OutgoingMessages <- sendingMessage
	case *webapi.PostMessage:
		message := message.Content.(*webapi.PostMessage)
		if _, err := slacker.WebAPIClient.PostMessage(message); err != nil {
			logrus.Error("something went wrong with Web API posting", err)
		}
	default:
		logrus.Warnf("unexpected command response %v", reflect.TypeOf(message).Name())
	}
}

/*
sendEnqueuedMessage receives messages via Slacker.OutgoingMessages, and send them over WebSocket connection.
This method is meant to be run in a single goroutine per Slacker instance.

Message sending is done in a serial manner one after another, so don't worry about sending multiple payloads over
WebSocket connection simultaneously. websocket.JSON.Send itself is goroutine-safe, though.
*/
func (slacker *Slacker) sendEnqueuedMessage() {
	for {
		select {
		case <-slacker.stopAll:
			return
		case message := <-slacker.OutgoingMessages:
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
func fetchRtmInfo(client *webapi.Client) (*webapi.RtmStart, error) {
	var rtmStart *webapi.RtmStart
	err := retry.RetryInterval(10, func() error {
		r, e := client.RtmStart()
		rtmStart = r
		return e
	}, 500*time.Millisecond)

	return rtmStart, err
}

// connectRtm establishes WebSocket connection with retries.
func connectRtm(client *rtmapi.Client, rtm *webapi.RtmStart) (*websocket.Conn, error) {
	var conn *websocket.Conn
	err := retry.RetryInterval(10, func() error {
		c, e := client.Connect(rtm.URL)
		conn = c
		return e
	}, 500*time.Millisecond)

	return conn, err
}

// NewStringCommandResponse creates new sarah.CommandResponse instance with given string response.
func NewStringCommandResponse(responseContent string) *sarah.CommandResponse {
	return &sarah.CommandResponse{ResponseContent: responseContent}
}

// NewPostMessageCommandResponse creates new sarah.CommandResponse instance with given *webapi.PostMessage response.
func NewPostMessageCommandResponse(responseContent *webapi.PostMessage) *sarah.CommandResponse {
	return &sarah.CommandResponse{ResponseContent: responseContent}
}
