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
	SLACK sarah.BotType = "slack"
)

type Slacker struct {
	WebAPIClient        *webapi.Client
	RtmAPIClient        *rtmapi.Client
	tryPing             chan bool
	Events              chan rtmapi.DecodedEvent
	outgoingEventId     *rtmapi.OutgoingEventID
	OutgoingMessages    chan *rtmapi.TextMessage
	StartNewRtm         chan bool
	webSocketConnection *websocket.Conn
	Stopper             chan bool
	stopAll             chan bool
	pluginConfigDir     string
}

func NewSlacker(token, pluginConfigDir string) *Slacker {
	slacker := &Slacker{
		WebAPIClient:     webapi.NewClient(&webapi.Config{Token: token}),
		RtmAPIClient:     rtmapi.NewClient(),
		tryPing:          make(chan bool),
		Events:           make(chan rtmapi.DecodedEvent, 100),
		outgoingEventId:  rtmapi.NewOutgoingEventID(),
		OutgoingMessages: make(chan *rtmapi.TextMessage, 100),
		StartNewRtm:      make(chan bool),
		Stopper:          make(chan bool),
		stopAll:          make(chan bool),
		pluginConfigDir:  pluginConfigDir,
	}
	return slacker
}

func (slacker *Slacker) GetPluginConfigDir() string {
	return slacker.pluginConfigDir
}

func (slacker *Slacker) GetBotType() sarah.BotType {
	return SLACK
}

func (slacker *Slacker) Run(receiver chan<- sarah.BotInput) {
	go slacker.supervise()
	go slacker.sendEnqueuedMessage()
	go slacker.receiveEvent(receiver)

	slacker.StartNewRtm <- true
}

func (slacker *Slacker) Stop() {
	slacker.Stopper <- true
}

func (slacker *Slacker) supervise() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-slacker.StartNewRtm:
			slacker.disconnect()
			if err := slacker.connect(); err != nil {
				logrus.Errorf("can't establish connection. %s", err.Error())
				slacker.Stopper <- true
			}
		case <-slacker.Stopper:
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

func (slacker *Slacker) connect() error {
	rtmInfo, err := slacker.fetchRtmInfo()
	if err != nil {
		return err
	}

	conn, err := slacker.connectRtm(rtmInfo)
	if err != nil {
		return err
	}

	slacker.webSocketConnection = conn
	return nil
}

func (slacker *Slacker) disconnect() {
	if slacker.webSocketConnection == nil {
		return
	}

	if err := slacker.webSocketConnection.Close(); err != nil {
		logrus.Errorf("error on connection close. type %T. value: %+v.", err, err)
	}
}

func (slacker *Slacker) receiveEvent(receiver chan<- sarah.BotInput) {
	for {
		select {
		case <-slacker.stopAll:
			logrus.Info("stop receiving events due to stop queue")
			return
		default:
			if slacker.webSocketConnection == nil {
				continue
			}

			payload, err := rtmapi.ReceivePayload(slacker.webSocketConnection)
			if err == io.EOF {
				slacker.tryPing <- true
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

func (slacker *Slacker) sendEnqueuedMessage() {
	for {
		select {
		case <-slacker.stopAll:
			return
		case message := <-slacker.OutgoingMessages:
			if slacker.webSocketConnection == nil {
				continue
			}

			event := rtmapi.NewOutgoingMessage(slacker.outgoingEventId, message)
			if err := websocket.JSON.Send(slacker.webSocketConnection, event); err != nil {
				logrus.Error("failed to send event", event)
			}
		}
	}
}

func (slacker *Slacker) checkConnection() {
	logrus.Debug("checking connection status with Ping payload.")
	ping := rtmapi.NewPing(slacker.outgoingEventId)
	if err := websocket.JSON.Send(slacker.webSocketConnection, ping); err != nil {
		logrus.Error("failed sending Ping payload", err)
		slacker.StartNewRtm <- true
	}
}

func (slacker *Slacker) fetchRtmInfo() (*webapi.RtmStart, error) {
	var rtmStart *webapi.RtmStart
	err := retry.RetryInterval(10, func() error {
		r, e := slacker.WebAPIClient.RtmStart()
		rtmStart = r
		return e
	}, 500*time.Millisecond)

	return rtmStart, err
}

func (slacker *Slacker) connectRtm(rtm *webapi.RtmStart) (*websocket.Conn, error) {
	var conn *websocket.Conn
	err := retry.RetryInterval(10, func() error {
		c, e := slacker.RtmAPIClient.Connect(rtm.URL)
		conn = c
		return e
	}, 500*time.Millisecond)

	return conn, err
}

func NewStringCommandResponse(responseContent string) *sarah.CommandResponse {
	return &sarah.CommandResponse{ResponseContent: responseContent}
}

func NewPostMessageCommandResponse(responseContent *webapi.PostMessage) *sarah.CommandResponse {
	return &sarah.CommandResponse{ResponseContent: responseContent}
}
