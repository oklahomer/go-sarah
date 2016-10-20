package rtmapi

import "sync"

// OutgoingEventID manages posting payloads' unique IDs.
// https://api.slack.com/rtm#sending_messages
type OutgoingEventID struct {
	id    uint
	mutex *sync.Mutex
}

// NewOutgoingEventID creates and returns new OutgoingEventID instance.
func NewOutgoingEventID() *OutgoingEventID {
	return &OutgoingEventID{
		id:    0,
		mutex: &sync.Mutex{},
	}
}

// Next increments the internally stored ID value and return this to caller.
func (m *OutgoingEventID) Next() uint {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.id++
	return m.id
}
