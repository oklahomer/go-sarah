package sarah

import "time"

// Input defines interface that each incoming message must satisfy.
// Each Bot/Adapter implementation may define customized Input implementation for each messaging content.
//
// See slack.MessageInput.
type Input interface {
	// SenderKey returns the text form of sender identifier.
	// This value can be used internally as a key to store the sender's conversational context in UserContextStorage.
	// Generally, When connecting chat service has the concept of group or chat room,
	// this sender key should contain the group/room identifier along with user identifier
	// so the user's conversational context is only applied in the exact same group/room.
	//
	// e.g. senderKey := fmt.Sprintf("%d_%d", roomID, userID)
	SenderKey() string

	// Message returns the text form of user input.
	// This may return empty string when this Input implementation represents non-text payload such as photo,
	// video clip or file.
	Message() string

	// SentAt returns the timestamp when the message is sent.
	// This may return a message reception time if the connecting chat service does not provide one.
	// e.g. XMPP server only provides timestamp as part of XEP-0203 when delayed message is delivered.
	SentAt() time.Time

	// ReplyTo returns the sender's address or location to be used to reply message.
	// This may be passed to Bot.SendMessage() as part of Output value to specify the sending destination.
	// This typically contains chat room, member id or mail address.
	// e.g. JID of XMPP server/client.
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
