package rtmapi

import "testing"

func TestIncrementOutgoingEventID(t *testing.T) {
	idDispenser := NewOutgoingEventID()

	if idDispenser.id != 0 {
		t.Errorf("id value is not starting from 0. id was %d", idDispenser.id)
		return
	}

	nextID := idDispenser.Next()
	if nextID != 1 {
		t.Errorf("id 1 must be given on first Next() call. id was %d.", nextID)
		return
	}
}
