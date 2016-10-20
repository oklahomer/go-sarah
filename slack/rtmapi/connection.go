package rtmapi

import (
	"encoding/json"
	"errors"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/websocket"
	"io"
)

var (
	EmptyPayloadError = errors.New("empty payload was given")
)

type DecodedPayload interface{}

type PayloadReceiver interface {
	Receive() (DecodedPayload, error)
}

type PayloadSender interface {
	Send(*Channel, string) error
	Ping() error
}

type Connection interface {
	PayloadReceiver
	PayloadSender
	io.Closer
}

// connWrapper is a thin wrapper that wraps WebSocket connection and its methods.
// This instance is created per-connection.
type connWrapper struct {
	conn *websocket.Conn

	// https://api.slack.com/rtm#sending_messages
	// Every event should have a unique (for that connection) positive integer ID.
	outgoingEventID *OutgoingEventID
}

func newConnectionWrapper(conn *websocket.Conn) Connection {
	return &connWrapper{
		conn:            conn,
		outgoingEventID: NewOutgoingEventID(),
	}

}

func (wrapper *connWrapper) Receive() (DecodedPayload, error) {
	// Blocking method to receive payload from WebSocket connection.
	// When connection is closed in the middle of this method call, this immediately returns error.
	payload := json.RawMessage{}
	err := websocket.JSON.Receive(wrapper.conn, &payload)
	if err != nil {
		return nil, err
	}

	return decodePayload(payload)
}

func (wrapper *connWrapper) Send(channel *Channel, content string) error {
	event := NewOutgoingMessage(wrapper.outgoingEventID, channel, content)
	return websocket.JSON.Send(wrapper.conn, event)
}

func (wrapper *connWrapper) Ping() error {
	ping := NewPing(wrapper.outgoingEventID)
	return websocket.JSON.Send(wrapper.conn, ping)
}

func (wrapper *connWrapper) Close() error {
	return wrapper.conn.Close()
}

func decodePayload(incoming json.RawMessage) (DecodedPayload, error) {
	// First, try decode incoming object as Event.
	decodedEvent, eventDecodeErr := DecodeEvent(incoming)
	if eventDecodeErr == nil {
		return decodedEvent, nil
	}

	if eventDecodeErr == UnsupportedEventTypeError {
		log.Infof("unsupported event type is fed. %s.", string(incoming))
		return nil, eventDecodeErr
	}

	if eventDecodeErr == EventTypeNotGivenError {
		// When incoming object can't be treated as Event, try treat this as WebSocketReply.
		reply, replyDecodeErr := DecodeReply(incoming)
		if replyDecodeErr != nil {
			// Payload is not event or reply.
			return nil, NewMalformedPayloadError(replyDecodeErr.Error())
		}

		return reply, nil
	}

	return nil, NewMalformedPayloadError(eventDecodeErr.Error())
}
