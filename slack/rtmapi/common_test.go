package rtmapi

import (
	"bytes"
	"strconv"
	"testing"
	"time"
)

func TestUnmarshalTimeStampText(t *testing.T) {
	timeStamp := &TimeStamp{}
	if err := timeStamp.UnmarshalText([]byte("1355517536.000001")); err != nil {
		t.Errorf("error on unmarshal slack timestamp. %s.", err.Error())
	}

	expectedTime := time.Unix(1355517536, 0)

	if !timeStamp.Time.Equal(expectedTime) {
		t.Errorf("unmarshaled time is wrong %s. expected %s.", timeStamp.Time.String(), expectedTime.String())
	}
}

func TestUnmarshalInvalidTimeStampText(t *testing.T) {
	invalidInput := "FooBar"
	timeStamp := &TimeStamp{}
	if err := timeStamp.UnmarshalText([]byte(invalidInput)); err == nil {
		t.Errorf("error should be returned for input %s.", invalidInput)
	}
}

func TestMarshalTimeStampText(t *testing.T) {
	now := time.Now()
	slackTimeStamp := strconv.Itoa(now.Second()) + ".123"
	timeStamp := &TimeStamp{Time: now, OriginalValue: slackTimeStamp}
	if b, e := timeStamp.MarshalText(); e == nil {
		if !bytes.Equal(b, []byte(slackTimeStamp)) {
			t.Errorf("marshaled value is wrong %s. expected %s.", string(b), slackTimeStamp)
		}
	} else {
		t.Errorf("error on marshal slack timestamp. %s.", e.Error())
	}
}
