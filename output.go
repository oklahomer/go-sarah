package sarah

// Output defines interface that each outgoing message must satisfy.
type Output interface {
	Destination() OutputDestination
	Content() interface{}
}

type OutputMessage struct {
	destination OutputDestination
	content     interface{}
}

func NewOutputMessage(destination OutputDestination, content interface{}) Output {
	return &OutputMessage{
		destination: destination,
		content:     content,
	}
}

func (output *OutputMessage) Destination() OutputDestination {
	return output.destination
}

func (output *OutputMessage) Content() interface{} {
	return output.content
}
