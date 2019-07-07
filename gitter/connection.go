package gitter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/oklahomer/go-sarah"
	"golang.org/x/xerrors"
	"io"
	"time"
)

var (
	// ErrEmptyPayload is an error that represents empty payload.
	ErrEmptyPayload = xerrors.New("empty payload was given")
)

// MessageReceiver defines an interface that receives RoomMessage.
type MessageReceiver interface {
	Receive() (*RoomMessage, error)
}

// Connection defines an interface that satisfies both MessageReceiver and io.Closer.
type Connection interface {
	MessageReceiver
	io.Closer
}

// connWrapper stashes connection per Room to utilize HTTP streaming API.
type connWrapper struct {
	Room       *Room
	readCloser io.ReadCloser
}

// NewConnection creates and return new Connection instance
func newConnWrapper(room *Room, readCloser io.ReadCloser) Connection {
	return &connWrapper{
		Room:       room,
		readCloser: readCloser,
	}
}

// ReadLine read single line from its connection.
// This line should contain JSON-formatted payload.
func (conn *connWrapper) Receive() (*RoomMessage, error) {
	// The document reads "The JSON stream returns messages as JSON objects that are delimited by carriage return (\r)"
	// but seems like '\n' is given, instead. Weired.
	reader := bufio.NewReader(conn.readCloser)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	message, err := decodePayload(line)
	if err != nil {
		return nil, err
	}

	return NewRoomMessage(conn.Room, message), nil
}

// Close closes its connection with gitter
func (conn *connWrapper) Close() error {
	return conn.readCloser.Close()
}

func decodePayload(payload []byte) (*Message, error) {
	// https://developer.gitter.im/docs/streaming-api#json-stream-application-json-
	// Parsers must be tolerant of occasional extra newline characters placed between messages.
	// These characters are sent as periodic "keep-alive" messages to tell clients and NAT firewalls
	// that the connection is still alive during low message volume periods.
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}

	message := &Message{}
	if err := json.Unmarshal(payload, message); err != nil {
		return nil, NewMalformedPayloadError(err.Error())
	}

	return message, nil
}

// RoomMessage stashes received Message and additional Room information.
type RoomMessage struct {
	Room            *Room
	ReceivedMessage *Message
}

// NewRoomMessage creates and returns new RoomMessage instance.
func NewRoomMessage(room *Room, message *Message) *RoomMessage {
	return &RoomMessage{
		Room:            room,
		ReceivedMessage: message,
	}
}

// SenderKey returns message sending user's ID.
func (message *RoomMessage) SenderKey() string {
	return fmt.Sprintf("%s|%s", message.Room.ID, message.ReceivedMessage.FromUser.ID)
}

// Message returns received text.
func (message *RoomMessage) Message() string {
	return message.ReceivedMessage.Text
}

// SentAt returns when the message is sent.
func (message *RoomMessage) SentAt() time.Time {
	return message.ReceivedMessage.SendTimeStamp.Time
}

// ReplyTo returns Room that message was being delivered.
func (message *RoomMessage) ReplyTo() sarah.OutputDestination {
	return message.Room
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
