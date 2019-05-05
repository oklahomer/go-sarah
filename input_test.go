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
	input := &DummyInput{
		SenderKeyValue: senderKey,
		MessageValue:   message,
		SentAtValue:    sentAt,
		ReplyToValue:   dest,
	}
	helpInput := NewHelpInput(input)

	if helpInput.SenderKey() != senderKey {
		t.Errorf("Expected sender key was not returned: %s.", senderKey)
	}
	if helpInput.Message() != message {
		t.Errorf("Expected message was not returned: %s.", message)
	}
	if helpInput.SentAt() != sentAt {
		t.Errorf("Expected time was not returned: %s.", sentAt.String())
	}
	if helpInput.ReplyTo() != dest {
		t.Errorf("Expected reply destination was not returned: %s.", dest)
	}
	if helpInput.OriginalInput != input {
		t.Errorf("Original Input value is not set: %#v", helpInput.OriginalInput)
	}
}

func TestNewAbortInput(t *testing.T) {
	senderKey := "sender"
	message := "Hello, 世界."
	sentAt := time.Now()
	dest := "100 N University Dr Edmond, OK"
	input := &DummyInput{
		SenderKeyValue: senderKey,
		MessageValue:   message,
		SentAtValue:    sentAt,
		ReplyToValue:   dest,
	}
	abortInput := NewAbortInput(input)

	if abortInput.SenderKey() != senderKey {
		t.Errorf("Expected sender key was not returned: %s.", senderKey)
	}
	if abortInput.Message() != message {
		t.Errorf("Expected message was not returned: %s.", message)
	}
	if abortInput.SentAt() != sentAt {
		t.Errorf("Expected time was not returned: %s.", sentAt.String())
	}
	if abortInput.ReplyTo() != dest {
		t.Errorf("Expected reply destination was not returned: %s.", dest)
	}
	if abortInput.OriginalInput != input {
		t.Errorf("Original Input value is not set: %#v", abortInput.OriginalInput)
	}
}
