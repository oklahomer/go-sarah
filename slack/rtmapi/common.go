package rtmapi

import (
	"strconv"
	"strings"
	"time"
)

// TimeStamp represents slack flavored time representation.
// Slack may pass timestamp in a form of "1355517536.000001," where first preceding integers before dot represents the UNIX timestamp.
// Following integers is used to uniquify the timestamp within a given channel.
// https://api.slack.com/events/message
type TimeStamp struct {
	Time time.Time

	// OriginalValue is exactly the same value slack passes. e.g. "1355517536.000001"
	OriginalValue string
}

// UnmarshalText parses a given slack timestamp to time.Time.
// This method is mainly used by encode/json.
func (timeStamp *TimeStamp) UnmarshalText(b []byte) error {
	str := string(b)
	timeStamp.OriginalValue = str

	i, err := strconv.ParseInt(strings.Split(str, ".")[0], 10, 64)
	if err != nil {
		return err
	}
	timeStamp.Time = time.Unix(i, 0)

	return nil
}

// String returns the original timestamp value given by slack.
func (timeStamp *TimeStamp) String() string {
	return timeStamp.OriginalValue
}

// MarshalText returns the stringified value of slack flavored timestamp.
// To ensure idempotence of marshal/unmarshal, this returns the original value given by slack.
func (timeStamp *TimeStamp) MarshalText() ([]byte, error) {
	return []byte(timeStamp.String()), nil
}

// Channel represents Slack channel.
type Channel struct {
	Name string
}

// UnmarshalText parses a given slack chanel information to Channel
// This method is mainly used by encode/json.
func (channel *Channel) UnmarshalText(b []byte) error {
	str := string(b)

	channel.Name = str

	return nil
}

// MarshalText returns the stringified value of Channel
func (channel *Channel) MarshalText() ([]byte, error) {
	return []byte(channel.Name), nil
}

// MalformedPayloadError represents an error that given JSON payload is not properly formatted.
// e.g. required fields are not given, or payload is not a valid JSON string.
type MalformedPayloadError struct {
	Err string
}

// Error returns its error string.
func (e *MalformedPayloadError) Error() string {
	return e.Err
}

// NewMalformedPayloadError creates new MalformedPayloadError instance with given arguments.
func NewMalformedPayloadError(str string) *MalformedPayloadError {
	return &MalformedPayloadError{Err: str}
}
