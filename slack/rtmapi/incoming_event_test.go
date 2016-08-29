package rtmapi

import (
	"encoding/json"
	"testing"
)

func TestDecodeMessage(t *testing.T) {
	event, _ := DecodeEvent(json.RawMessage([]byte("{\"type\": \"message\", \"channel\": \"C2147483705\", \"user\": \"U2147483697\", \"text\": \"Hello, world!\", \"ts\": \"1355517523.000005\", \"edited\": { \"user\": \"U2147483697\", \"ts\": \"1355517536.000001\"}}")))

	message, ok := event.(*Message)
	if message.Type != MESSAGE {
		t.Errorf("unexpected type %s", message.Type)
	}
	if !ok {
		t.Errorf("unexpected event %#v", event)
	}
	if message.TimeStamp.Time.Unix() != 1355517523 {
		t.Errorf("unexpected unix timestamp %d", message.TimeStamp.Time.Unix())
	}
	if message.TimeStamp.OriginalValue != "1355517523.000005" {
		t.Errorf("unexpected unix timestamp %s", message.TimeStamp.OriginalValue)
	}
	if message.Channel.Name != "C2147483705" {
		t.Errorf("unexpected channel value %#v", message.Channel)
	}
	if message.Sender.ID != "U2147483697" {
		t.Errorf("unexpected sender id %#v", message.Sender)
	}
}

func TestDecodeUnknownEvent(t *testing.T) {
	event, err := DecodeEvent(json.RawMessage([]byte("{\"type\": \"foo\", \"channel\": \"C2147483705\"}")))

	if event != nil {
		t.Error("event is returned even though unknown type field is given")
	}

	if _, ok := err.(*UnknownEventTypeError); !ok {
		t.Errorf("returned error is not type of UnknownEventTypeError, but is %#v", err.Error())
	}
}
