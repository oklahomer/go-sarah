package sarah

// Output defines interface that each outgoing message must satisfy.
type Output interface {
	Destination() OutputDestination
	Content() interface{}
}

// OutputMessage represents outgoing message.
type OutputMessage struct {
	destination OutputDestination
	content     interface{}
}

// NewOutputMessage creates and returns new OutputMessage instance.
// This satisfies Output interface so can be passed to Bot.SendMessage.
func NewOutputMessage(destination OutputDestination, content interface{}) Output {
	return &OutputMessage{
		destination: destination,
		content:     content,
	}
}

// Destination returns its destination in a form of OutputDestination interface.
// Each Bot/Adapter implementation must explicitly define destination type that satisfies OutputDestination.
func (output *OutputMessage) Destination() OutputDestination {
	return output.destination
}

// Content returns sending content.
// This is just an empty interface, so each Bot/Adapter developer may define depending on whatever the struct should contain.
func (output *OutputMessage) Content() interface{} {
	return output.content
}
