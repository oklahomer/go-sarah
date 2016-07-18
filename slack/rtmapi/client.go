package rtmapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/websocket"
)

type Client struct {
	PayloadDecoder func(json.RawMessage) (DecodedEvent, error)
}

func NewClient() *Client {
	return &Client{PayloadDecoder: DefaultPayloadDecoder}
}

func (client *Client) Connect(url string) (*websocket.Conn, error) {
	return websocket.Dial(url, "", "http://localhost")
}

func (client *Client) DecodePayload(payload json.RawMessage) (DecodedEvent, error) {
	return client.PayloadDecoder(payload)
}

/*
DecodePayload decodes given payloads, which include various kinds of events and reply from slack.
When given payload is an event, it returns decoded event or error; while it returns nil or error when given payload is a reply.
Beware that it does nothing and returns nil when WebSocketReply is given from slack and it doesn't indicate error on previous post.
What we always want is just events, but reply is given and it has different format so there...
*/
func DefaultPayloadDecoder(payload json.RawMessage) (DecodedEvent, error) {
	decodedEvent, eventDecodeError := DecodeEvent(payload)

	if _, ok := eventDecodeError.(*MalformedEventTypeError); ok {
		// When "type" field is not present or is unknown, this MIGHT be a reply payload that indicate current posting status.
		// Check the reply status and leave a log if this indicates previous malformed message.
		if reply, err := DecodeReply(payload); err != nil {
			// This payload can't be treated as any of known event, status reply, or anything.
			return nil, NewMalformedPayloadError(err.Error())
		} else if !*reply.OK {
			return nil, NewReplyStatusError(reply)
		}

		// No problem with the previous message posting.
		return nil, nil
	} else if eventDecodeError != nil {
		return nil, NewMalformedPayloadError(eventDecodeError.Error())
	}

	return decodedEvent, nil
}

func ReceivePayload(conn *websocket.Conn) (json.RawMessage, error) {
	payload := json.RawMessage{}

	// TODO err := websocket.Message.Receive(conn, &payload)
	err := websocket.JSON.Receive(conn, &payload)
	if err != nil {
		return nil, err
	} else if len(payload) == 0 {
		// TODO
		return nil, errors.New("emptry payload is given")
	}

	return payload, nil
}

/*
ReplyStatusError is returned when given WebSocketReply payload is indicating a status error.
*/
type ReplyStatusError struct {
	Reply *WebSocketReply
}

/*
Error returns its error string.
*/
func (e *ReplyStatusError) Error() string {
	return fmt.Sprintf("error on previous message posting. %#v", e.Reply)
}

/*
NewReplyStatusError creates new ReplyStatusError instance with given arguments.
*/
func NewReplyStatusError(reply *WebSocketReply) *ReplyStatusError {
	return &ReplyStatusError{Reply: reply}
}

type TextMessage struct {
	channel string
	text    string
}

func NewTextMessage(channel string, text string) *TextMessage {
	return &TextMessage{channel: channel, text: text}
}
