package rtmapi

import "sync"

func NewOutgoingEventID() *OutgoingEventID {
	return &OutgoingEventID{
		id:    0,
		mutex: &sync.Mutex{},
	}
}

type OutgoingEventID struct {
	id    uint
	mutex *sync.Mutex
}

func (m *OutgoingEventID) Next() uint {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.id++
	return m.id
}
