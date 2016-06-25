package rtmapi

import (
	"encoding/json"
	"errors"
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
	Channel string `json:"channel"`
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
GetSenderId returns sender's identifier.
*/
func (message *Message) GetSenderId() string {
	return message.User
}

/*
GetMessage returns sent message.
*/
func (message *Message) GetMessage() string {
	return message.Text
}

/*
GetSentAt returns message event's timestamp.
*/
func (message *Message) GetSentAt() time.Time {
	return message.TimeStamp.Time
}

/*
GetRoomID returns room identifier.
*/
func (message *Message) GetRoomID() string {
	return message.Channel
}

/*
DecodedEvent is just an empty interface that marks decoded event.
This can be used to define method signature or type of returning value.
*/
type DecodedEvent interface {
}

/*
DecodeEvent decodes given event input and converts this to corresponding event structure.
*/
func DecodeEvent(input json.RawMessage) (DecodedEvent, error) {
	event := &CommonEvent{}
	if err := json.Unmarshal(input, event); err != nil {
		return nil, err
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
	default:
		return nil, NewUnknownEventTypeError("received unknown event. " + string(input))
	}

	if err := json.Unmarshal(input, mapping); err != nil {
		return nil, errors.New("error on JSON deserializing to mapped event. " + string(input))
	}

	return mapping, nil
}

/*
UnknownEventTypeError is returned when given event's type is undefined.
*/
type UnknownEventTypeError struct {
	error string
}

/*
NewUnknownEventTypeError creates new instance of UnknownEventTypeError with given error string.
*/
func NewUnknownEventTypeError(e string) error {
	return &UnknownEventTypeError{error: e}
}

/*
Error returns its error string.
*/
func (e UnknownEventTypeError) Error() string {
	return e.error
}
