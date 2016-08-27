package sarah

// CommandResponse is returned by Command execution when response is available.
type Message struct {
	RoomID  string
	Content interface{}
}

func (message *Message) GetRoomID() string {
	return message.RoomID
}
