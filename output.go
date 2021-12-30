package sarah

// Output defines an interface that each outgoing message must satisfy.
type Output interface {
	// Destination returns the destination the output is to be sent.
	Destination() OutputDestination

	// Content returns the sending payload.
	Content() interface{}
}

// OutputMessage represents an outgoing message.
type OutputMessage struct {
	destination OutputDestination
	content     interface{}
}

var _ Output = (*OutputMessage)(nil)

// NewOutputMessage creates a new instance of an Output implementation -- OutputMessage -- with the given OutputDestination and the payload.
func NewOutputMessage(destination OutputDestination, content interface{}) Output {
	return &OutputMessage{
		destination: destination,
		content:     content,
	}
}

// Destination returns its destination in a form of OutputDestination.
// Each Bot/Adapter implementation must explicitly define an OutputDestination implementation
// so that Bot.SendMessage and Adapter.SendMessage can specify where the message should be directed to.
func (output *OutputMessage) Destination() OutputDestination {
	return output.destination
}

// Content returns a sending payload.
// Each Bot/Adapter must be capable of properly handling the payload and sending the message to the given destination.
func (output *OutputMessage) Content() interface{} {
	return output.content
}
