package rtmapi

import (
	"strings"
)

// EventType represents the type of event sent from slack.
// Event is passed to client in a form of JSON string, which has a field named "type."
type EventType string

// List of available EventTypes
const (
	UnsupportedEvent          EventType = "unsupported"
	HelloEvent                          = "hello"
	MessageEvent                        = "message"
	TeamMigrationStartedEvent           = "team_migration_started"
	PingEvent                           = "ping"
	PongEvent                           = "pong"
)

var (
	possibleEvents = [...]EventType{HelloEvent, MessageEvent, TeamMigrationStartedEvent, PingEvent, PongEvent}
)

// UnmarshalText parses a given event value to EventType.
// This method is mainly used by encode/json.
func (eventType *EventType) UnmarshalText(b []byte) error {
	str := string(b)
	for _, val := range possibleEvents {
		if str == val.String() {
			*eventType = val
			return nil
		}
	}
	*eventType = UnsupportedEvent
	return nil
}

// String returns the stringified event name, which corresponds to the one sent from/to slack RTM endpoint.
func (eventType EventType) String() string {
	return string(eventType)
}

// MarshalText returns the stringified value of slack event.
// This method is mainly used by encode/json.
func (eventType *EventType) MarshalText() ([]byte, error) {
	str := eventType.String()

	if strings.Compare(str, "") == 0 {
		return []byte(UnsupportedEvent), nil
	}

	return []byte(str), nil
}

// CommonEvent takes care of some common fields all incoming/outgoing event MUST have.
// https://api.slack.com/rtm#events
type CommonEvent struct {
	Type EventType `json:"type,omitempty"`
}

// SubType may given as a part of message payload to describe detailed content.
type SubType string

const (
	// List of available SubTypes
	Empty            SubType = "" // can be absent
	BotMessage               = "bot_message"
	ChannelArchive           = "channel_archive"
	ChannelJoin              = "channel_join"
	ChannelLeave             = "channel_leave"
	ChannelName              = "channel_name"
	ChannelPurpose           = "channel_purpose"
	ChannelTopic             = "channel_topic"
	ChannelUnarchive         = "channel_unarchive"
	FileComment              = "file_comment"
	FileMention              = "file_mention"
	FileShare                = "file_share"
	GroupArchive             = "group_archive"
	GroupJoin                = "group_join"
	GroupLeave               = "group_leave"
	GroupName                = "group_name"
	GroupPurpose             = "group_purpose"
	GroupTopic               = "group_topic"
	GroupUnarchive           = "group_unarchive"
	MeMessage                = "me_message"
	MessageChanged           = "message_changed"
	MessageDeleted           = "message_deleted"
	PinnedItem               = "pinned_item"
	UnpinnedItem             = "unpinned_item"
)

var (
	possibleSubTypes = [...]SubType{
		BotMessage, ChannelArchive, ChannelJoin, ChannelLeave, ChannelName, ChannelPurpose, ChannelTopic,
		ChannelUnarchive, FileComment, FileMention, FileShare, GroupArchive, GroupJoin, GroupLeave,
		GroupName, GroupPurpose, GroupTopic, GroupUnarchive, MeMessage, MessageChanged, MessageDeleted,
		PinnedItem, UnpinnedItem,
	}
)

// UnmarshalText parses a given subtype value to SubType
// This method is mainly used by encode/json.
func (subType *SubType) UnmarshalText(b []byte) error {
	str := string(b)
	for _, val := range possibleSubTypes {
		if str == val.String() {
			*subType = val
			return nil
		}
	}
	*subType = Empty
	return nil
}

// String returns the stringified subtype name, which corresponds to the one sent from/to slack RTM endpoint.
func (subType SubType) String() string {
	return string(subType)
}

// MarshalText returns the stringified value of slack subtype.
// This method is mainly used by encode/json.
func (subType *SubType) MarshalText() ([]byte, error) {
	str := subType.String()

	if strings.Compare(str, "") == 0 {
		return []byte(""), nil // EMPTY
	}

	return []byte(str), nil
}

// CommonMessage contains some common fields of message event.
// See SubType field to distinguish corresponding event struct.
// https://api.slack.com/events/message#message_subtypes
type CommonMessage struct {
	Type EventType `json:"type"`

	// Regular user message and some miscellaneous message share the common type of "message."
	// So take a look at subtype to distinguish. Regular user message has empty subtype.
	SubType SubType `json:"subtype"`
}
