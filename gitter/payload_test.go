package gitter

import (
	"testing"
	"time"
)

func TestTimeStamp_UnmarshalText(t *testing.T) {
	givenTime := "2015-04-08T07:06:00.000Z"
	expected, _ := time.Parse(TimeFormat, givenTime)
	data := []struct {
		givenValue string
		hasErr     bool
		retVal     time.Time
	}{
		{
			givenValue: "2006-01-02 15:04:05.999Z",
			hasErr:     true,
			retVal:     time.Time{},
		},
		{
			givenValue: givenTime,
			hasErr:     false,
			retVal:     expected,
		},
	}

	for _, datum := range data {
		timestamp := &TimeStamp{}
		err := timestamp.UnmarshalText([]byte(datum.givenValue))

		if datum.hasErr && err == nil {
			t.Errorf("Expected error is not returned.")
			continue
		}

		if !datum.hasErr && !timestamp.Time.Equal(expected) {
			t.Errorf("Unecpected TimeStamp is returned: %s.", timestamp.String())
			continue
		}
	}
}

func TestTimeStamp_String(t *testing.T) {
	timestamp := &TimeStamp{
		OriginalValue: "2006-01-02 15:04:05.999Z",
	}

	if timestamp.String() != timestamp.OriginalValue {
		t.Errorf("Unexpected TimeStamp is retruend: %s.", timestamp.String())
	}
}

func TestTimeStamp_MarshalText(t *testing.T) {
	timestamp := &TimeStamp{
		OriginalValue: "2006-01-02 15:04:05.999Z",
	}

	b, e := timestamp.MarshalText()
	if e != nil {
		t.Fatalf("Unexpected error is returned: %s.", e.Error())
	}

	if string(b) != timestamp.OriginalValue {
		t.Errorf("Unexpected TimeStamp is returned: %s.", string(b))
	}
}
