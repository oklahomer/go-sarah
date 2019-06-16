package gitter

import (
	"golang.org/x/xerrors"
	"time"
)

const (
	// TimeFormat defines gitter-styled timestamp format.
	// https://golang.org/pkg/time/#Time.Format
	TimeFormat = "2006-01-02T15:04:05.999Z"
)

// TimeStamp represents gitter timestamp.
type TimeStamp struct {
	Time          time.Time
	OriginalValue string // e.g. "2014-03-24T15:41:18.991Z"
}

// UnmarshalText unmarshals given gitter-styled timestamp to TimeStamp
func (timeStamp *TimeStamp) UnmarshalText(b []byte) error {
	str := string(b)
	timeStamp.OriginalValue = str

	t, err := time.Parse(TimeFormat, str)
	if err != nil {
		return xerrors.Errorf("failed to parse timestamp %s: %w", str, err)
	}
	timeStamp.Time = t

	return nil
}

// String returns original gitter-styled timestamp value.
func (timeStamp *TimeStamp) String() string {
	return timeStamp.OriginalValue
}

// MarshalText marshals TimeStamp to gitter-styled one.
func (timeStamp *TimeStamp) MarshalText() ([]byte, error) {
	return []byte(timeStamp.String()), nil
}

// Room represents gitter room resource.
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

// Rooms is a group of Room
type Rooms []*Room

// User represents gitter user resource.
// https://developer.gitter.im/docs/user-resource
type User struct {
	ID              string `json:"id"`
	UserName        string `json:"username"`
	DisplayName     string `json:"displayName"`
	URL             string `json:"url"` // path
	AvatarURLSmall  string `json:"avatarUrlSmall"`
	AvatarURLMedium string `json:"avatarUrlMedium"`
}

// Message represents gitter message resource.
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

// Mention represents mention in the message.
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
