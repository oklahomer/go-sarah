package gitter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/v4"
	"io"
	"time"
)

var (
	// ErrEmptyPayload is an error that represents an empty payload.
	ErrEmptyPayload = errors.New("empty payload was given")
)

// MessageReceiver defines an interface that receives RoomMessage over Streaming API.
type MessageReceiver interface {
	// Receive reads a new incoming message and return this as RoomMessage.
	// This blocks till a new message comes.
	Receive() (*RoomMessage, error)
}

// Connection defines an interface that satisfies both MessageReceiver and io.Closer.
type Connection interface {
	MessageReceiver
	io.Closer
}

// connWrapper stashes a connection for a designated Room to utilize HTTP streaming API.
type connWrapper struct {
	Room       *Room
	readCloser io.ReadCloser
}

var _ Connection = (*connWrapper)(nil)

// newConnWrapper creates and returns a new connection wrapper for a given Room in a form of Connection.
func newConnWrapper(room *Room, readCloser io.ReadCloser) Connection {
	return &connWrapper{
		Room:       room,
		readCloser: readCloser,
	}
}

// Receive reads a new incoming message and return this as RoomMessage.
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

// Close closes its connection to Gitter's Streaming API.
func (conn *connWrapper) Close() error {
	return conn.readCloser.Close()
}

func decodePayload(payload []byte) (*Message, error) {
	// https://developer.gitter.im/docs/streaming-api#json-stream-application-json-
	// Parsers must be tolerant of extra newline characters occasionally placed in between messages.
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

// RoomMessage is a sarah.Input implementation that represents a received message.
type RoomMessage struct {
	// Room represents where the message was sent.
	Room *Room

	// ReceivedMessage represents the received message.
	ReceivedMessage *Message
}

var _ sarah.Input = (*RoomMessage)(nil)

// NewRoomMessage creates and returns a new RoomMessage instance.
func NewRoomMessage(room *Room, message *Message) *RoomMessage {
	return &RoomMessage{
		Room:            room,
		ReceivedMessage: message,
	}
}

// SenderKey returns the message sender's id.
func (message *RoomMessage) SenderKey() string {
	return fmt.Sprintf("%s|%s", message.Room.ID, message.ReceivedMessage.FromUser.ID)
}

// Message returns the received text.
func (message *RoomMessage) Message() string {
	return message.ReceivedMessage.Text
}

// SentAt returns when the message is sent.
func (message *RoomMessage) SentAt() time.Time {
	return message.ReceivedMessage.SendTimeStamp.Time
}

// ReplyTo returns the Room the message was sent.
func (message *RoomMessage) ReplyTo() sarah.OutputDestination {
	return message.Room
}

// MalformedPayloadError represents an error that a given JSON payload is not properly formatted.
// e.g. required fields are not given, or payload is not a valid JSON string.
type MalformedPayloadError struct {
	// Err tells the error reason.
	Err string
}

// Error returns its error message.
func (e *MalformedPayloadError) Error() string {
	return e.Err
}

// NewMalformedPayloadError creates a new MalformedPayloadError instance with the given error message.
func NewMalformedPayloadError(str string) *MalformedPayloadError {
	return &MalformedPayloadError{Err: str}
}
