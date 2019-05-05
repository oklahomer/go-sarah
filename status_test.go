package sarah

import (
	"testing"
	"time"
)

func TestCurrentStatus(t *testing.T) {
	// Override the package scoped variable that holds *status instance.
	// Copy of this status should be returned on CurrentStatus().
	botType := BotType("dummy")
	runnerStatus = &status{
		bots: []*botStatus{
			{
				botType:  botType,
				finished: make(chan struct{}),
			},
		},
	}

	// Check initial state
	currentStatus := CurrentStatus()

	if currentStatus.Running {
		t.Error("Status should not be Running at this point.")
	}

	if len(currentStatus.Bots) != 1 {
		t.Fatalf("Unexpected number of BotStatus is returned: %d.", len(currentStatus.Bots))
	}

	if currentStatus.Bots[0].Type != botType {
		t.Errorf("Expected BotType is not set. %#v", currentStatus.Bots[0])
	}
}

func Test_status_start(t *testing.T) {
	s := &status{}

	// Initial call
	err := s.start()
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if s.finished == nil {
		t.Error("A channel to judge running status must be set.")
	}

	// Successive call should return an error
	err = s.start()
	if err == nil {
		t.Fatalf("Expected error is not returned.")
	}
	if err != ErrRunnerAlreadyRunning {
		t.Errorf("Returned error is not the expected one: %s", err.Error())
	}
}

func Test_status_running(t *testing.T) {
	s := &status{}

	if s.running() {
		t.Error("Status should not be Running at this point.")
	}

	s.finished = make(chan struct{})
	if !s.running() {
		t.Error("Status should be Running at this point.")
	}

	close(s.finished)
	if s.running() {
		t.Error("Status should not be Running at this point.")
	}
}

func Test_status_stop(t *testing.T) {
	s := &status{
		finished: make(chan struct{}),
	}

	s.stop()

	select {
	case <-s.finished:
		// O.K. Channel is closed.

	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Error("A channel is not closed on status.stop.")

	}

	s.stop() // Multiple call to this method should not panic.
}

func Test_status_addBot(t *testing.T) {
	botType := BotType("dummy")
	bot := &DummyBot{BotTypeValue: botType}
	s := &status{}
	s.addBot(bot)

	botStatuses := s.bots
	if len(botStatuses) != 1 {
		t.Fatal("Status for one and only one Bot should be set.")
	}

	bs := botStatuses[0]

	if bs.botType != botType {
		t.Errorf("Expected BotType is not set: %s.", bs.botType)
	}

	if !bs.running() {
		t.Error("Bot status must be running at this point.")
	}
}

func Test_status_stopBot(t *testing.T) {
	botType := BotType("dummy")
	bs := &botStatus{
		botType:  botType,
		finished: make(chan struct{}),
	}
	s := &status{
		bots: []*botStatus{bs},
	}

	bot := &DummyBot{BotTypeValue: botType}
	s.stopBot(bot)

	botStatuses := s.bots
	if len(botStatuses) != 1 {
		t.Fatal("Status for one and only one Bot should be set.")
	}

	stored := botStatuses[0]

	if stored.botType != botType {
		t.Errorf("Expected BotType is not set: %s.", bs.botType)
	}

	if stored.running() {
		t.Error("Bot status must not be running at this point.")
	}
}

func Test_status_snapshot(t *testing.T) {
	botType := BotType("dummy")
	bs := &botStatus{
		botType:  botType,
		finished: make(chan struct{}),
	}
	s := &status{
		bots:     []*botStatus{bs},
		finished: make(chan struct{}),
	}

	snapshot := s.snapshot()
	if !snapshot.Running {
		t.Error("Status.Running should be true at this point.")
	}

	if len(snapshot.Bots) != 1 {
		t.Errorf("The number of registered Bot should be one, but was %d.", len(snapshot.Bots))
	}

	if !snapshot.Bots[0].Running {
		t.Error("BotStatus.Running should be true at this point.")
	}

	close(bs.finished)
	close(s.finished)

	snapshot = s.snapshot()

	if snapshot.Running {
		t.Error("Status.Running should be false at this point.")
	}

	if snapshot.Bots[0].Running {
		t.Error("BotStatus.Running should be false at this point.")
	}
}

func Test_botStatus_running(t *testing.T) {
	bs := &botStatus{
		botType:  "dummy",
		finished: make(chan struct{}),
	}

	if !bs.running() {
		t.Error("botStatus.running() should be true at this point.")
	}

	close(bs.finished)

	if bs.running() {
		t.Error("botStatus.running() should be false at this point.")
	}
}

func Test_botStatus_stop(t *testing.T) {
	bs := &botStatus{
		finished: make(chan struct{}),
	}

	bs.stop()

	select {
	case <-bs.finished:
		// O.K. Channel is closed.

	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Error("A channel is not closed on botStatus.stop.")

	}

	bs.stop() // Multiple call to this method should not panic.
}
