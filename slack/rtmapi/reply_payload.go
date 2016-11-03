package rtmapi

import (
	"encoding/json"
)

// WebSocketReply is passed from slack as a reply to client message, and indicates its status.
// https://api.slack.com/rtm#ping_and_pong#handling_responses
type WebSocketReply struct {
	// https://willnorris.com/2014/05/go-rest-apis-and-pointers
	OK *bool `json:"ok"`

	ReplyTo   uint       `json:"reply_to"`
	TimeStamp *TimeStamp `json:"ts"`
	Text      string     `json:"text"`
}

// DecodeReply parses given reply payload from slack.
func DecodeReply(input json.RawMessage) (*WebSocketReply, error) {
	reply := &WebSocketReply{}
	if err := json.Unmarshal(input, reply); err != nil {
		return nil, NewMalformedPayloadError(err.Error())
	}

	// Check required fields.
	if reply.OK == nil {
		return nil, NewMalformedPayloadError("ok field is not given. " + string(input))
	}

	return reply, nil
}
