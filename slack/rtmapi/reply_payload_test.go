package rtmapi

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeReply(t *testing.T) {
	str := "{\"ok\": true, \"reply_to\": 1, \"ts\": \"1355517523.000005\", \"text\": \"Hello world\"}"
	if reply, err := DecodeReply([]byte(str)); err == nil {
		if reply == nil {
			t.Error("reply instance is not returned.")
			return
		}

		if reply.OK == nil || !*reply.OK {
			t.Errorf("expecting ok status of true, but wasn't. %#v", reply)
		}

		if reply.ReplyTo != 1 {
			t.Errorf("expecting ReplyTo to be 1, but was %d", reply.ReplyTo)
		}

		if strings.Compare(reply.Text, "Hello world") != 0 {
			t.Errorf("expecting Text to be 'Hello world,' but was %s", reply.Text)
		}

		if !reply.TimeStamp.Time.Equal(time.Unix(1355517523, 0)) {
			t.Errorf("expected timestamp is not returned. %s", reply.TimeStamp.Time.String())
		}
	} else {
		t.Errorf("error on DecodeReply. %s", err.Error())
	}
}
