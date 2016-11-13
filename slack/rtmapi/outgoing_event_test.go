package rtmapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalPingEvent(t *testing.T) {
	ping := &Ping{
		OutgoingCommonEvent: OutgoingCommonEvent{
			ID:          1,
			CommonEvent: CommonEvent{Type: PING},
		},
	}
	val, err := json.Marshal(ping)
	if err != nil {
		t.Fatalf("error occured while encoding. %s.", err.Error())
	}

	if strings.Contains(string(val), "ping") != true {
		t.Fatalf(`returned string doesn't contain "ping". %s.`, string(val))
	}
}

func TestUnmarshalPingEvent(t *testing.T) {
	str := `{"type": "ping", "id": 123}`
	ping := &Ping{}
	if err := json.Unmarshal([]byte(str), ping); err != nil {
		t.Errorf("error on Unmarshal. %s.", err.Error())
		return
	}

	if ping.Type != PING {
		t.Errorf("something is wrong with unmarshaled result. %#v.", ping)
	}

	if ping.ID != 123 {
		t.Errorf("unmarshaled id is wrong %d. expecting %d.", ping.ID, 123)
	}
}
