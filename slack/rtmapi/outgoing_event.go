package rtmapi

import "github.com/oklahomer/go-sarah/slack/common"

/*
OutgoingEvent is just an empty interface that marks outgoing event.
This can be used to define method signature or type of returning value.
*/
type OutgoingEvent interface {
}

/*
OutgoingCommonEvent takes care of some common fields all outgoing event MUST have.
https://api.slack.com/rtm#events
*/
type OutgoingCommonEvent struct {
	CommonEvent
	ID uint `json:"id"`
}

/*
OutgoingMessage represents a simple message sent from client to Slack server via WebSocket connection.
This is the only format RTM API supports. To send more richly formatted message, use Web API.
https://api.slack.com/rtm#sending_messages
*/
type OutgoingMessage struct {
	OutgoingCommonEvent
	ID      uint            `json:"id"`
	Channel *common.Channel `json:"channel"`
	Text    string          `json:"text"`
}

/*
NewOutgoingMessage is a constructor to create new OutgoingMessage instance with given arguments.
*/
func NewOutgoingMessage(eventID *OutgoingEventID, message *TextMessage) *OutgoingMessage {
	return &OutgoingMessage{
		Channel: message.channel,
		Text:    message.text,
		OutgoingCommonEvent: OutgoingCommonEvent{
			ID: eventID.Next(),
			CommonEvent: CommonEvent{
				Type: MESSAGE,
			},
		},
	}
}

/*
Ping is an event that can be sent to slack endpoint via WebSocket to see if the connection is alive.
Slack sends back Pong event if connection is O.K.
*/
type Ping struct {
	OutgoingCommonEvent
}

/*
NewPing creates new Ping instance with given arguments.
*/
func NewPing(eventID *OutgoingEventID) *Ping {
	return &Ping{
		OutgoingCommonEvent: OutgoingCommonEvent{
			ID:          eventID.Next(),
			CommonEvent: CommonEvent{Type: PING},
		},
	}
}
