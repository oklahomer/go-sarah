package sarah

import "time"

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
