package sarah

// OutputDestination defines an interface that represents a destination where the outgoing message is heading to, which actually is empty.
// Think of this as a kind of marker interface with a more meaningful name.
// Every Bot and Adapter implementation MUST define a struct to express the destination for the connecting chat service.
type OutputDestination interface{}
