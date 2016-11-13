package rtmapi

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestUnmarshalEventType(t *testing.T) {
	var eventType EventType
	if err := eventType.UnmarshalText([]byte(HelloEvent)); err != nil {
		t.Errorf("error on valid event type unmarshal. %s.", err.Error())
		return
	}

	if strings.Compare(string(eventType), string(HelloEvent)) != 0 {
		t.Errorf("event type, %s, is wrong. expecting %s.", eventType, HelloEvent)
	}
}

func TestUnmarshalInvalidEventType(t *testing.T) {
	var eventType EventType
	if err := eventType.UnmarshalText([]byte("INVALID")); err != nil {
		t.Errorf("error on event type unmarshal, where it should be UNKNOWN. %s.", err.Error())
		return
	}

	if strings.Compare(string(eventType), string(UnsupportedEvent)) != 0 {
		t.Errorf("event type, %s, is wrong. expecting %s.", eventType, UnsupportedEvent)
	}
}

func TestMarshalEventType(t *testing.T) {
	eventType := EventType(HelloEvent)
	if b, e := eventType.MarshalText(); e == nil {
		if !bytes.Equal(b, []byte(HelloEvent)) {
			t.Errorf("marshaled value is wrong %s. expected %s.", string(b), string(HelloEvent))
		}
	} else {
		t.Errorf("error on marshal slack event type. %s.", e.Error())
	}
}

func TestMarshalZeroValuedEventType(t *testing.T) {
	var eventType EventType
	if b, e := eventType.MarshalText(); e == nil {
		if !bytes.Equal(b, []byte(UnsupportedEvent)) {
			t.Errorf("marshaled value is wrong %s. expected %s.", string(b), string(UnsupportedEvent))
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

	if parsedEvent.Type != MessageEvent {
		t.Errorf("type field is not properly parsed. %s", parsedEvent.Type)
	}

}

func TestMarshalCommonEvent(t *testing.T) {
	event := CommonEvent{Type: MessageEvent}
	if b, err := json.Marshal(event); err == nil {
		if !strings.Contains(string(b), string(MessageEvent)) {
			t.Errorf(`returned text doesn't contain "message". %s.`, string(b))
		}
	} else {
		t.Errorf("error on json.Marshal. %s.", err.Error())
	}

}
