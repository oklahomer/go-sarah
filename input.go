package sarah

import "time"

// Input defines an interface that each incoming message must satisfy.
// Every Bot/Adapter implementation must define one or customized Input implementations for the corresponding incoming messages.
//
// It is Bot/Adapter's responsibility to receive input from the messaging user, convert it to Input, and see if the input represents a specific request: help and context cancellation.
// When the Input represents a request for help, then pass the Input to NewHelpInput to wrap it with a HelpInput;
// when the Input represents a request for a context cancellation, pass the Input to AbortInput to wrap it with AbortInput.
//
// HelpInput and AbortInput can be passed to Sarah through the function given to Bot.Run -- func(Input) error -- just like any other Input.
// Sarah then passes a job to the worker.Worker implementation to execute Bot.Respond, in a panic-proof concurrent manner, with the given Input.
type Input interface {
	// SenderKey returns the stringified representation of the sender identifier.
	// This value can be used as a unique key to store the sender's conversational context in UserContextStorage.
	// Generally, when the connecting chat service has the concept of group or chat room,
	// this sender key should contain the group/room identifier along with the user identifier
	// so the user's conversational context is only applicable in the exact same group/room.
	//
	// e.g. senderKey := fmt.Sprintf("%d_%d", roomID, userID)
	SenderKey() string

	// Message returns the stringified representation of the user input.
	// This may return an empty string when this Input implementation represents a non-text payload such as a photo,
	// video clip, or file.
	Message() string

	// SentAt returns the timestamp when the message is sent.
	// This may return a message reception time if the connecting chat service does not provide one.
	// e.g. XMPP server provides a timestamp only when a delayed message is delivered with the XEP-0203 protocol.
	SentAt() time.Time

	// ReplyTo returns the sender's address or location to be used to reply a message.
	// This can be passed to Bot.SendMessage() as part of Output value to specify the sending destination.
	// This typically contains a chat room, member id, or e-mail address.
	// e.g. JID of XMPP server/client.
	ReplyTo() OutputDestination
}

// NewHelpInput creates a new instance of an Input implementation -- HelpInput -- with the given Input.
func NewHelpInput(input Input) *HelpInput {
	return &HelpInput{
		OriginalInput: input,
		senderKey:     input.SenderKey(),
		message:       input.Message(),
		sentAt:        input.SentAt(),
		replyTo:       input.ReplyTo(),
	}
}

// HelpInput is a common Input implementation that represents a user's request for a help.
// When this type is given to Bot.Respond, a Bot implementation should list up registered Commands' instructions and send them back to the user.
type HelpInput struct {
	OriginalInput Input
	senderKey     string
	message       string
	sentAt        time.Time
	replyTo       OutputDestination
}

var _ Input = (*HelpInput)(nil)

// SenderKey returns a stringified representation of the message sender.
func (hi *HelpInput) SenderKey() string {
	return hi.senderKey
}

// Message returns the stringified representation of the message.
func (hi *HelpInput) Message() string {
	return hi.message
}

// SentAt returns the timestamp when the message is sent.
func (hi *HelpInput) SentAt() time.Time {
	return hi.sentAt
}

// ReplyTo returns the sender's address or location to be used to reply a message.
func (hi *HelpInput) ReplyTo() OutputDestination {
	return hi.replyTo
}

// NewAbortInput creates a new instance of an Input implementation -- AbortInput -- with the given input.
func NewAbortInput(input Input) *AbortInput {
	return &AbortInput{
		OriginalInput: input,
		senderKey:     input.SenderKey(),
		message:       input.Message(),
		sentAt:        input.SentAt(),
		replyTo:       input.ReplyTo(),
	}
}

// AbortInput is a common Input implementation that represents the user's request for a context cancellation.
// When this type is given to Bot.Respond, the Bot implementation should cancel the user's current conversational context.
type AbortInput struct {
	OriginalInput Input
	senderKey     string
	message       string
	sentAt        time.Time
	replyTo       OutputDestination
}

var _ Input = (*AbortInput)(nil)

// SenderKey returns a stringified representation of the message sender.
func (ai *AbortInput) SenderKey() string {
	return ai.senderKey
}

// Message returns the stringified representation of the message.
func (ai *AbortInput) Message() string {
	return ai.message
}

// SentAt returns the timestamp when the message is sent.
func (ai *AbortInput) SentAt() time.Time {
	return ai.sentAt
}

// ReplyTo returns the sender's address or location to be used to reply a message.
func (ai *AbortInput) ReplyTo() OutputDestination {
	return ai.replyTo
}
