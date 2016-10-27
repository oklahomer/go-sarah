package sarah

import "time"

type testInput struct {
	senderKey string
	message   string
	sentAt    time.Time
	replyTo   OutputDestination
}

func (i *testInput) SenderKey() string {
	return i.senderKey
}

func (i *testInput) Message() string {
	return i.message
}

func (i *testInput) SentAt() time.Time {
	return i.sentAt
}

func (i *testInput) ReplyTo() OutputDestination {
	return i.replyTo
}
