package rtmapi

import (
	"strings"
)

/*
EventType represents the type of event sent from slack.
Event is passed to client in a form of JSON string, which has a field named "type."
*/
type EventType string

const (
	UNKNOWN                EventType = "unknown"
	HELLO                            = "hello"
	MESSAGE                          = "message"
	TEAM_MIGRATION_STARTED           = "team_migration_started"
	PING                             = "ping"
	PONG                             = "pong"
)

var (
	possibleEvents = [...]EventType{HELLO, MESSAGE, TEAM_MIGRATION_STARTED, PING, PONG}
)

/*
UnmarshalText parses a given event value to EventType.
This method is mainly used by encode/json.
*/
func (eventType *EventType) UnmarshalText(b []byte) error {
	str := string(b)
	for _, val := range possibleEvents {
		if str == val.String() {
			*eventType = val
			return nil
		}
	}
	*eventType = UNKNOWN
	return nil
}

/*
String returns the stringified event name, which corresponds to the one sent from/to slack RTM endpoint.
*/
func (eventType EventType) String() string {
	return string(eventType)
}

/*
MarshalText returns the stringified value of slack event.
This method is mainly used by encode/json.
*/
func (eventType *EventType) MarshalText() ([]byte, error) {
	if str := eventType.String(); strings.Compare(str, "") == 0 {
		return []byte(UNKNOWN), nil
	} else {
		return []byte(str), nil
	}
}

/*
CommonEvent takes care of some common fields all incoming/outgoing event MUST have.
https://api.slack.com/rtm#events
*/
type CommonEvent struct {
	Type EventType `json:"type,omitempty"`
}
