package rtmapi

import (
	"encoding/json"
	"errors"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack/common"
	"time"
)

/*
Hello event is sent from slack when WebSocket connection is successfully established.
https://api.slack.com/events/hello
*/
type Hello struct {
	CommonEvent
}

/*
TeamMigrationStarted is sent when chat group is migrated between servers.
"The WebSocket connection will close immediately after it is sent.
*snip* By the time a client has reconnected the process is usually complete, so the impact is minimal."
https://api.slack.com/events/team_migration_started
*/
type TeamMigrationStarted struct {
	CommonEvent
}

/*
Pong is given when client send Ping.
https://api.slack.com/rtm#ping_and_pong
*/
type Pong struct {
	CommonEvent
	ReplyTo uint `json:"reply_to"`
}

/*
IncomingChannelEvent represents any event occurred in a specific channel.
This can be a part of other event such as message.
*/
type IncomingChannelEvent struct {
	CommonEvent
	Channel string `json:"channel"` // TODO common.Channel
}

/*
Message represent message event on RTM.
https://api.slack.com/events/message
{
    "type": "message",
    "channel": "C2147483705",
    "user": "U2147483697",
    "text": "Hello, world!",
    "ts": "1355517523.000005",
    "edited": {
        "user": "U2147483697",
        "ts": "1355517536.000001"
    }
}
*/
type Message struct {
	IncomingChannelEvent
	User      string    `json:"user"`
	Text      string    `json:"text"`
	TimeStamp TimeStamp `json:"ts"`
}

// Let Message implement BotInput

/*
SenderId returns sender's identifier.
*/
func (message *Message) SenderID() string {
	return message.User
}

/*
Message returns sent message.
*/
func (message *Message) Message() string {
	return message.Text
}

/*
SentAt returns message event's timestamp.
*/
func (message *Message) SentAt() time.Time {
	return message.TimeStamp.Time
}

/*
ReplyTo returns slack channel to send reply to.
*/
func (message *Message) ReplyTo() sarah.OutputDestination {
	return common.NewChannel(message.Channel)
}

/*
DecodedEvent is just an empty interface that marks decoded event.
This can be used to define method signature or type of returning value.
*/
type DecodedEvent interface {
}

/*
DecodeEvent decodes given payload and converts this to corresponding event structure.
*/
func DecodeEvent(input json.RawMessage) (DecodedEvent, error) {
	event := &CommonEvent{}
	if err := json.Unmarshal(input, event); err != nil {
		return nil, NewMalformedPayloadError(err.Error())
	}

	var mapping DecodedEvent

	switch event.Type {
	case HELLO:
		mapping = &Hello{}
	case MESSAGE:
		mapping = &Message{}
	case TEAM_MIGRATION_STARTED:
		mapping = &TeamMigrationStarted{}
	case PONG:
		mapping = &Pong{}
	case "":
		// type is not given and is filled with zero-value
		// or empty string is given as type value
		return nil, NewMalformedEventTypeError("type is not given. " + string(input))
	default:
		// What? New event type? Time to check latest update.
		return nil, NewUnknownEventTypeError("received unknown event. " + string(input))
	}

	if err := json.Unmarshal(input, mapping); err != nil {
		return nil, errors.New("error on JSON deserializing to mapped event. " + string(input))
	}

	return mapping, nil
}

/*
MalformedEventTypeError represents an error that given payload can properly parsed as valid JSON string,
but it is missing "type" field.
*/
type MalformedEventTypeError struct {
	Err string
}

/*
NewMalformedEventTypeError creates new MalformedEventTypeError instance with given arguments.
*/
func NewMalformedEventTypeError(e string) *MalformedEventTypeError {
	return &MalformedEventTypeError{Err: e}
}

/*
Error returns its error string.
*/
func (e *MalformedEventTypeError) Error() string {
	return e.Err
}

/*
UnknownEventTypeError is returned when given event's type is undefined.
*/
type UnknownEventTypeError struct {
	Err string
}

/*
NewUnknownEventTypeError creates new instance of UnknownEventTypeError with given error string.
*/
func NewUnknownEventTypeError(e string) *UnknownEventTypeError {
	return &UnknownEventTypeError{Err: e}
}

/*
Error returns its error string.
*/
func (e *UnknownEventTypeError) Error() string {
	return e.Err
}
