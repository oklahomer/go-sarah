package worker

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	worker := New()

	if worker.isRunning != false {
		t.Error("unexpected worker state")
	}
}

func TestWorker_Run(t *testing.T) {
	worker := New()
	cancel := make(chan struct{})

	// Start worker
	err := worker.Run(cancel, 5)
	if err != nil {
		t.Fatalf("failed to run. %s", err.Error())
	}

	// panic won't affect
	worker.EnqueueJob(func() {
		panic("My house is on FIRE!!")
	})

	// Check if job runs
	isFinished := false
	worker.EnqueueJob(func() {
		isFinished = true
	})
	time.Sleep(100 * time.Millisecond)
	if isFinished == false {
		t.Error("job is not executed")
	}

	// Error should return on multiple Run call
	err = worker.Run(cancel, 5)
	if err == nil {
		t.Error("worker.Run is called multiple times")
	}

	// Stop worker
	close(cancel)
	time.Sleep(100 * time.Millisecond)
	if worker.IsRunning() != false {
		t.Error("worker.IsRunning still returns true after cancelation")
	}
}
