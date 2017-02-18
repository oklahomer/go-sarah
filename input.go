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

// SenderKey returns string representing message sender.
func (hi *HelpInput) SenderKey() string {
	return hi.senderKey
}

// Message returns sent message.
func (hi *HelpInput) Message() string {
	return hi.message
}

// SentAt returns message event's timestamp.
func (hi *HelpInput) SentAt() time.Time {
	return hi.sentAt
}

// ReplyTo returns slack channel to send reply to.
func (hi *HelpInput) ReplyTo() OutputDestination {
	return hi.replyTo
}

// AbortInput is a common Input implementation that represents user's request for context cancellation.
// When this type is given, each Bot/Adapter implementation should cancel and remove corresponding user's conversational context.
type AbortInput struct {
	senderKey string
	message   string
	sentAt    time.Time
	replyTo   OutputDestination
}

// SenderKey returns string representing message sender.
func (ai *AbortInput) SenderKey() string {
	return ai.senderKey
}

// Message returns sent message.
func (ai *AbortInput) Message() string {
	return ai.message
}

// SentAt returns message event's timestamp.
func (ai *AbortInput) SentAt() time.Time {
	return ai.sentAt
}

// ReplyTo returns slack channel to send reply to.
func (ai *AbortInput) ReplyTo() OutputDestination {
	return ai.replyTo
}

// NewAbortInput creates a new AbortInput instance with given arguments and returns it.
func NewAbortInput(senderKey, message string, sentAt time.Time, replyTo OutputDestination) *AbortInput {
	return &AbortInput{
		senderKey: senderKey,
		message:   message,
		sentAt:    sentAt,
		replyTo:   replyTo,
	}
}
