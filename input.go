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

// NewHelpInput creates a new HelpInput instance with given user input and returns it.
// This is Bot/Adapter's responsibility to receive an input from user, convert it to sarah.Input and see if the input requests for "help."
// For example, a slack adapter may check if the given message is equal to :help: emoji.
// If true, create a HelpInput instant with NewHelpInput and pass it to go-sarah's core.
func NewHelpInput(input Input) *HelpInput {
	return &HelpInput{
		OriginalInput: input,
		senderKey:     input.SenderKey(),
		message:       input.Message(),
		sentAt:        input.SentAt(),
		replyTo:       input.ReplyTo(),
	}
}

// HelpInput is a common Input implementation that represents user's request for help.
// When this type is given, each Bot/Adapter implementation should list up registered Commands' instructions
// and send them back to user.
type HelpInput struct {
	OriginalInput Input
	senderKey     string
	message       string
	sentAt        time.Time
	replyTo       OutputDestination
}

var _ Input = (*HelpInput)(nil)

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

// NewAbortInput creates a new AbortInput instance with given input.
// When this type is given, each Bot/Adapter implementation should cancel the user's conversational context.
func NewAbortInput(input Input) *AbortInput {
	return &AbortInput{
		OriginalInput: input,
		senderKey:     input.SenderKey(),
		message:       input.Message(),
		sentAt:        input.SentAt(),
		replyTo:       input.ReplyTo(),
	}
}

// AbortInput is a common Input implementation that represents user's request for context cancellation.
// When this type is given, each Bot/Adapter implementation should cancel and remove corresponding user's conversational context.
type AbortInput struct {
	OriginalInput Input
	senderKey     string
	message       string
	sentAt        time.Time
	replyTo       OutputDestination
}

var _ Input = (*AbortInput)(nil)

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
