package rtmapi

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestUnmarshalEventType(t *testing.T) {
	var eventType EventType
	if err := eventType.UnmarshalText([]byte(HELLO)); err != nil {
		t.Errorf("error on valid event type unmarshal. %s.", err.Error())
		return
	}

	if strings.Compare(string(eventType), string(HELLO)) != 0 {
		t.Errorf("event type, %s, is wrong. expecting %s.", eventType, HELLO)
	}
}

func TestUnmarshalInvalidEventType(t *testing.T) {
	var eventType EventType
	if err := eventType.UnmarshalText([]byte("INVALID")); err != nil {
		t.Errorf("error on event type unmarshal, where it should be UNKNOWN. %s.", err.Error())
		return
	}

	if strings.Compare(string(eventType), string(UNSUPPORTED)) != 0 {
		t.Errorf("event type, %s, is wrong. expecting %s.", eventType, UNSUPPORTED)
	}
}

func TestMarshalEventType(t *testing.T) {
	eventType := EventType(HELLO)
	if b, e := eventType.MarshalText(); e == nil {
		if !bytes.Equal(b, []byte(HELLO)) {
			t.Errorf("marshaled value is wrong %s. expected %s.", string(b), string(HELLO))
		}
	} else {
		t.Errorf("error on marshal slack event type. %s.", e.Error())
	}
}

func TestMarshalZeroValuedEventType(t *testing.T) {
	var eventType EventType
	if b, e := eventType.MarshalText(); e == nil {
		if !bytes.Equal(b, []byte(UNSUPPORTED)) {
			t.Errorf("marshaled value is wrong %s. expected %s.", string(b), string(UNSUPPORTED))
		}
	} else {
		t.Errorf("error on marshal slack event type. %s.", e.Error())
	}
}

func TestUnmarshalCommonEvent(t *testing.T) {
	messageEvent := []byte(`{"type": "message"}`)
	parsedEvent := CommonEvent{}
	if err := json.Unmarshal(messageEvent, &parsedEvent); err != nil {
		t.Errorf("error on parsing given JSON structure. %s. %s.", string(messageEvent), err.Error())
		return
	}

	if parsedEvent.Type != MESSAGE {
		t.Errorf("type field is not properly parsed. %s", parsedEvent.Type)
	}

}

func TestMarshalCommonEvent(t *testing.T) {
	event := CommonEvent{Type: MESSAGE}
	if b, err := json.Marshal(event); err == nil {
		if !strings.Contains(string(b), string(MESSAGE)) {
			t.Errorf(`returned text doesn't contain "message". %s.`, string(b))
		}
	} else {
		t.Errorf("error on json.Marshal. %s.", err.Error())
	}

}
