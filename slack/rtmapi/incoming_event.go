package rtmapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack/common"
	"time"
)

var (
	ErrUnsupportedEventType = errors.New("given type is not supported.")
	ErrEventTypeNotGiven    = errors.New("type field is not given")
)

// Hello event is sent from slack when WebSocket connection is successfully established.
// https://api.slack.com/events/hello
type Hello struct {
	CommonEvent
}

// TeamMigrationStarted is sent when chat group is migrated between servers.
// "The WebSocket connection will close immediately after it is sent.
// *snip* By the time a client has reconnected the process is usually complete, so the impact is minimal."
// https://api.slack.com/events/team_migration_started
type TeamMigrationStarted struct {
	CommonEvent
}

// Pong is given when client send Ping.
// https://api.slack.com/rtm#ping_and_pong
type Pong struct {
	CommonEvent
	ReplyTo uint `json:"reply_to"`
}

// IncomingChannelEvent represents any event occurred in a specific channel.
// This can be a part of other event such as message.
type IncomingChannelEvent struct {
	CommonEvent
	Channel *Channel `json:"channel"`
}

// Message represent message event on RTM.
// https://api.slack.com/events/message
// This implements Input interface.
//  {
//      "type": "message",
//      "channel": "C2147483705",
//      "user": "U2147483697",
//      "text": "Hello, world!",
//      "ts": "1355517523.000005",
//      "edited": {
//          "user": "U2147483697",
//          "ts": "1355517536.000001"
//      }
//  }
type Message struct {
	IncomingChannelEvent
	Sender    *common.UserIdentifier `json:"user"`
	Text      string                 `json:"text"`
	TimeStamp *TimeStamp             `json:"ts"`
}

// SenderKey returns string representing message sender.
func (message *Message) SenderKey() string {
	return fmt.Sprintf("%s|%s", message.Channel.Name, message.Sender.ID)
}

// Message returns sent message.
func (message *Message) Message() string {
	return message.Text
}

// SentAt returns message event's timestamp.
func (message *Message) SentAt() time.Time {
	return message.TimeStamp.Time
}

// ReplyTo returns slack channel to send reply to.
func (message *Message) ReplyTo() sarah.OutputDestination {
	return message.Channel
}

// MiscMessage represents some minor message events.
// TODO define each one with subtype field. This is just a representation of common subtyped payload
// https://api.slack.com/events/message#message_subtypes
type MiscMessage struct {
	CommonMessage
	TimeStamp *TimeStamp `json:"ts"`
}

// DecodedEvent is just an empty interface that marks decoded event.
// This can be used to define method signature or type of returning value.
type DecodedEvent interface {
}

// DecodeEvent decodes given payload and converts this to corresponding event structure.
func DecodeEvent(input json.RawMessage) (DecodedEvent, error) {
	event := &CommonEvent{}
	if err := json.Unmarshal(input, event); err != nil {
		return nil, NewMalformedPayloadError(err.Error())
	}

	var mapping DecodedEvent

	switch event.Type {
	case UNSUPPORTED:
		return nil, ErrUnsupportedEventType
	case HELLO:
		mapping = &Hello{}
	case MESSAGE:
		subTypedMessage := &CommonMessage{}
		if err := json.Unmarshal(input, subTypedMessage); err != nil {
			return nil, NewMalformedPayloadError(err.Error())
		}
		switch subTypedMessage.SubType {
		case EMPTY:
			mapping = &Message{}
		default:
			mapping = &MiscMessage{}
		}
	case TEAM_MIGRATION_STARTED:
		mapping = &TeamMigrationStarted{}
	case PONG:
		mapping = &Pong{}
	case "":
		// type field is not given so string's zero value, empty string, is set.
		return nil, ErrEventTypeNotGiven
	default:
		// What?? Even if the type field is not given, "" should be set and there for case check for "" should
		// catch that.
		panic(fmt.Sprintf("error on event decode. %s", string(input)))
	}

	if err := json.Unmarshal(input, mapping); err != nil {
		return nil, errors.New("error on JSON deserializing to mapped event. " + string(input))
	}

	return mapping, nil
}
