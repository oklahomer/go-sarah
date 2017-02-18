package sarah

import "time"

// Input defines interface that each incoming message must satisfy.
// Each Bot/Adapter implementation may define customized Input implementation for each messaging content.
//
// See slack.MessageInput.
type Input interface {
	SenderKey() string
	Message() string
	SentAt() time.Time
	ReplyTo() OutputDestination
}

// HelpInput is a common Input implementation that represents user's request for help.
// When this type is given, each Bot/Adapter implementation should list up registered Commands' input examples,
// and send them back to user.
type HelpInput struct {
	senderKey string
	message   string
	sentAt    time.Time
	replyTo   OutputDestination
}

// NewHelpInput creates a new HelpInput instance with given arguments and returns it.
func NewHelpInput(senderKey, message string, sentAt time.Time, replyTo OutputDestination) *HelpInput {
	return &HelpInput{
		senderKey: senderKey,
		message:   message,
		sentAt:    sentAt,
		replyTo:   replyTo,
	}
}

func (hi *HelpInput) SenderKey() string {
	return hi.senderKey
}

func (hi *HelpInput) Message() string {
	return hi.message
}

func (hi *HelpInput) SentAt() time.Time {
	return hi.sentAt
}

func (hi *HelpInput) ReplyTo() OutputDestination {
	return hi.replyTo
}
