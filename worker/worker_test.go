package worker

import (
	"golang.org/x/net/context"
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
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	// Start worker
	err := worker.Run(ctx, 5)
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
	err = worker.Run(ctx, 5)
	if err == nil {
		t.Error("worker.Run is called multiple times")
	}

	// Stop worker
	cancel()
	time.Sleep(100 * time.Millisecond)
	if worker.IsRunning() != false {
		t.Error("worker.IsRunning still returns true after cancelation")
	}
}
