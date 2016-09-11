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

type SubType string

const (
	EMPTY             SubType = "" // can be absent
	BOT_MESSAGE               = "bot_message"
	CHANNEL_ARCHIVE           = "channel_archive"
	CHANNEL_JOIN              = "channel_join"
	CHANNEL_LEAVE             = "channel_leave"
	CHANNEL_NAME              = "channel_name"
	CHANNEL_PURPOSE           = "channel_purpose"
	CHANNEL_TOPIC             = "channel_topic"
	CHANNEL_UNARCHIVE         = "channel_unarchive"
	FILE_COMMENT              = "file_comment"
	FILE_MENTION              = "file_mention"
	FILE_SHARE                = "file_share"
	GROUP_ARCHIVE             = "group_archive"
	GROUP_JOIN                = "group_join"
	GROUP_LEAVE               = "group_leave"
	GROUP_NAME                = "group_name"
	GROUP_PURPOSE             = "group_purpose"
	GROUP_TOPIC               = "group_topic"
	GROUP_UNARCHIVE           = "group_unarchive"
	ME_MESSAGE                = "me_message"
	MESSAGE_CHANGED           = "message_changed"
	MESSAGE_DELETED           = "message_deleted"
	PINNED_ITEM               = "pinned_item"
	UNPINNED_ITEM             = "unpinned_item"
)

var (
	possibleSubTypes = [...]SubType{
		BOT_MESSAGE, CHANNEL_ARCHIVE, CHANNEL_JOIN, CHANNEL_LEAVE, CHANNEL_NAME, CHANNEL_PURPOSE, CHANNEL_TOPIC,
		CHANNEL_UNARCHIVE, FILE_COMMENT, FILE_MENTION, FILE_SHARE, GROUP_ARCHIVE, GROUP_JOIN, GROUP_LEAVE,
		GROUP_NAME, GROUP_PURPOSE, GROUP_TOPIC, GROUP_UNARCHIVE, ME_MESSAGE, MESSAGE_CHANGED, MESSAGE_DELETED,
		PINNED_ITEM, UNPINNED_ITEM,
	}
)

/*
UnmarshalText parses a given subtype value to SubType
This method is mainly used by encode/json.
*/
func (subType *SubType) UnmarshalText(b []byte) error {
	str := string(b)
	for _, val := range possibleSubTypes {
		if str == val.String() {
			*subType = val
			return nil
		}
	}
	*subType = EMPTY
	return nil
}

/*
String returns the stringified subtype name, which corresponds to the one sent from/to slack RTM endpoint.
*/
func (subType SubType) String() string {
	return string(subType)
}

/*
MarshalText returns the stringified value of slack subtype.
This method is mainly used by encode/json.
*/
func (subType *SubType) MarshalText() ([]byte, error) {
	if str := subType.String(); strings.Compare(str, "") == 0 {
		return []byte(""), nil // EMPTY
	} else {
		return []byte(str), nil
	}
}

/*
https://api.slack.com/events/message#message_subtypes
*/
type CommonMessage struct {
	Type EventType `json:"type,omitempty"`

	// Regular user message and some miscellaneous message share the common type of "message."
	// So take a look at subtype to distinguish. Regular user message has empty subtype.
	SubType SubType `json:"subtype,omitempty"`
}
