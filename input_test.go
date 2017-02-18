package sarah

import (
	"testing"
	"time"
)

type DummyInput struct {
	SenderKeyValue string
	MessageValue   string
	SentAtValue    time.Time
	ReplyToValue   OutputDestination
}

func (i *DummyInput) SenderKey() string {
	return i.SenderKeyValue
}

func (i *DummyInput) Message() string {
	return i.MessageValue
}

func (i *DummyInput) SentAt() time.Time {
	return i.SentAtValue
}

func (i *DummyInput) ReplyTo() OutputDestination {
	return i.ReplyToValue
}

func TestNewHelpInput(t *testing.T) {
	senderKey := "sender"
	message := "Hello, 世界."
	sentAt := time.Now()
	dest := "100 N University Dr Edmond, OK"
	input := NewHelpInput(senderKey, message, sentAt, dest)

	if input.SenderKey() != senderKey {
		t.Errorf("Expected sender key was not returned: %s.", senderKey)
	}
	if input.Message() != message {
		t.Errorf("Expected message was not returned: %s.", message)
	}
	if input.SentAt() != sentAt {
		t.Errorf("Expected time was not returned: %s.", sentAt.String())
	}
	if input.ReplyTo() != dest {
		t.Errorf("Expected reply destination was not returned: %s.", dest)
	}
}
