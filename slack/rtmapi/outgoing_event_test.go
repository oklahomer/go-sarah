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
		t.Error("error occured while encoding. %s.", err.Error())
		return
	}

	if strings.Contains(string(val), "ping") != true {
		t.Error("returned string doesn't contain \"ping\"", string(val))
		return
	}
}

func TestUnmarshalPingEvent(t *testing.T) {
	str := "{\"type\": \"ping\", \"id\": 123}"
	ping := &Ping{}
	if err := json.Unmarshal([]byte(str), ping); err != nil {
		t.Errorf("error on Unmarshal. %s.", err.Error())
		return
	}

	if ping.Type != PING {
		t.Errorf("something is wrong with unmarshaled result. %s.", ping)
	}

	if ping.ID != 123 {
		t.Errorf("unmarshaled id is wrong %d. expecting %d.", ping.ID, 123)
	}
}
