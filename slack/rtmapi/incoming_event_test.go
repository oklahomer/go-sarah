package rtmapi

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestDecodeEvent(t *testing.T) {
	type output struct {
		payload reflect.Type
		err     error
	}
	var decodeTests = []struct {
		input  string
		output output
	}{
		{
			`{"type": "message", "channel": "C2147483705", "user": "U2147483697", "text": "Hello, world!", "ts": "1355517523.000005", "edited": { "user": "U2147483697", "ts": "1355517536.000001"}}`,
			output{
				reflect.TypeOf(&Message{}),
				nil,
			},
		},
		{
			`{"type": "message", "subtype": "channel_join", "text": "<@UXXXXX|bobby> has joined the channel", "ts": "1403051575.000407", "user": "U023BECGF"}`,
			output{
				reflect.TypeOf(&MiscMessage{}),
				nil,
			},
		},
		{
			`{"type": "", "channel": "C2147483705"}`,
			output{
				nil,
				UnsupportedEventTypeError,
			},
		},
		{
			`{"type": "foo", "channel": "C2147483705"}`,
			output{
				nil,
				UnsupportedEventTypeError,
			},
		},
		{
			`{"channel": "C2147483705"}`,
			output{
				nil,
				EventTypeNotGivenError,
			},
		},
	}

	for i, testSet := range decodeTests {
		testCnt := i + 1
		inputByte := []byte(testSet.input)
		event, err := DecodeEvent(inputByte)

		if testSet.output.payload != reflect.TypeOf(event) {
			t.Errorf("Test No. %d. expected return type of %s, but was %#v", testCnt, testSet.output.payload.Name(), err)
		}
		if testSet.output.err != err {
			t.Errorf("Test No. %d. Expected return error of %#v, but was %#v", testCnt, testSet.output.err, err)
		}
	}
}

func TestMessage_Interface(t *testing.T) {
	input := []byte(`{"type": "message", "channel": "C2147483705", "user": "U2147483697", "text": "Hello, world!", "ts": "1355517523.000005", "edited": { "user": "U2147483697", "ts": "1355517536.000001"}}`)
	message := &Message{}
	err := json.Unmarshal(input, message)
	if err != nil {
		t.Fatalf("unexpected error on unmarshaling payload. %s", err.Error())
	}

	if message.SenderKey() != "C2147483705|U2147483697" {
		t.Errorf("unexpected sender key %s", message.SenderKey())
	}
	if !message.SentAt().Equal(time.Unix(1355517523, 0)) {
		t.Errorf("unexpected unix timestamp %d", message.SentAt().Second())
	}
	if message.Message() != "Hello, world!" {
		t.Errorf("unexpected Message %s", message.Message())
	}

	replyTo, ok := message.ReplyTo().(*Channel)
	if !ok {
		t.Fatalf("unexpected ReplyTo struct. %#v", message.ReplyTo())
	}
	if replyTo.Name != "C2147483705" {
		t.Errorf("unexpected ReplyTo %s", message.ReplyTo())
	}
}
