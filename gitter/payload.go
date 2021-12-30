package gitter

import (
	"time"
)

const (
	// TimeFormat defines the Gitter-styled timestamp format.
	// https://golang.org/pkg/time/#Time.Format
	TimeFormat = "2006-01-02T15:04:05.999Z"
)

// TimeStamp represents the timestamp when its belonging event occurred.
type TimeStamp struct {
	// Time is the time.Time representation of the timestamp.
	Time time.Time

	// OriginalValue is the original timestamp value given by Gitter.
	OriginalValue string // e.g. "2014-03-24T15:41:18.991Z"
}

// UnmarshalText unmarshals the given Gitter-styled timestamp to TimeStamp.
func (timeStamp *TimeStamp) UnmarshalText(b []byte) error {
	str := string(b)
	timeStamp.OriginalValue = str

	t, err := time.Parse(TimeFormat, str)
	if err != nil {
		return err
	}
	timeStamp.Time = t

	return nil
}

// String returns the original Gitter-styled timestamp value.
func (timeStamp *TimeStamp) String() string {
	return timeStamp.OriginalValue
}

// MarshalText marshals TimeStamp to a Gitter-styled one.
func (timeStamp *TimeStamp) MarshalText() ([]byte, error) {
	return []byte(timeStamp.String()), nil
}

// Room represents Gitter's room resource.
// https://developer.gitter.im/docs/rooms-resource
type Room struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Topic          string    `json:"topic"`
	URI            string    `json:"uri"`
	OneToOne       bool      `json:"oneToOne"`
	Users          []*User   `json:"users"`
	UnreadItems    uint      `json:"unreadItems"`
	Mentions       uint      `json:"mentions"`
	LastAccessTime TimeStamp `json:"lastAccessTime"`
	Favourite      uint      `json:"favourite"`
	Lurk           bool      `json:"lurk"`
	URL            string    `json:"url"`        // path
	GitHubType     string    `json:"githubType"` // TODO type
	Tags           []string  `json:"tags"`
	Version        uint      `json:"v"`
}

// Rooms represents a group of Room instances.
type Rooms []*Room

// User represents Gitter's user resource.
// https://developer.gitter.im/docs/user-resource
type User struct {
	ID              string `json:"id"`
	UserName        string `json:"username"`
	DisplayName     string `json:"displayName"`
	URL             string `json:"url"` // path
	AvatarURLSmall  string `json:"avatarUrlSmall"`
	AvatarURLMedium string `json:"avatarUrlMedium"`
}

// Message represents Gitter's message resource.
// https://developer.gitter.im/docs/messages-resource
type Message struct {
	ID            string    `json:"id"`
	Text          string    `json:"text"`
	HTML          string    `json:"html"`
	SendTimeStamp TimeStamp `json:"sent"`
	EditTimeStamp TimeStamp `json:"editedAt"`
	FromUser      User      `json:"fromUser"`
	Unread        bool      `json:"unread"`
	ReadBy        uint      `json:"readBy"`
	URLs          []string  `json:"urls"`
	Mentions      []Mention `json:"mentions"`
	Issues        []Issue   `json:"issues"`
	Meta          []Meta    `json:"meta"` // Reserved, but not in use
	Version       uint      `json:"v"`
}

// Mention represents a mention in the message.
type Mention struct {
	ScreenName string `json:"screenName"`
	UserID     string `json:"userId"`
}

// Issue represents issue number mentioned in a message.
type Issue struct {
	Number uint `json:"number"`
}

// Meta is reserved, but is not used so far.
// https://developer.gitter.im/docs/messages-resource
type Meta struct {
}
