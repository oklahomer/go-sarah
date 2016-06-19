package worker

import (
	"testing"
	"time"
)

func TestConstruct(t *testing.T) {
	workerNum := 5
	pool := NewPool(workerNum)

	// Worker size
	if actualSize := len(pool.workers); actualSize != workerNum {
		t.Errorf("unexpected worker size. expected: %d. actual: %d.", workerNum, actualSize)
	}

	// Check id designation and its duplication
	workerIDs := []int{}
	for _, worker := range pool.workers {
		workerIDs = append(workerIDs, worker.ID)
	}
	uniqueWorkerIDs := make(map[int]bool)
	for id := range workerIDs {
		uniqueWorkerIDs[id] = true
	}
	if len(uniqueWorkerIDs) != workerNum {
		t.Errorf("worker.ID duplicates, %v.", workerIDs)
	}
}

func TestStatus(t *testing.T) {
	workerNum := 5
	pool := NewPool(workerNum)

	if pool.isRunning != false {
		t.Errorf("worker pool status insists its running.")
	}

	err := pool.Run()
	if err != nil {
		t.Errorf("error on worker pool start: %s.", err.Error())
	}
	if pool.isRunning == false {
		t.Error("status not updated.")
	}

	err = pool.Run()
	if err == nil {
		t.Errorf("error should be given on multiple Run.")
	}

	isFinished := false
	pool.EnqueueJob(func() {
		isFinished = true
	})
	time.Sleep(500 * time.Millisecond)
	if isFinished == false {
		t.Error("job is not executed")
	}

	err = pool.Stop()
	if err != nil {
		t.Errorf("error on worker pool stop: %s.", err.Error())
	}
	if pool.isRunning == true {
		t.Error("status not updated.")
	}

	err = pool.Stop()
	if err == nil {
		t.Errorf("error should be given on multiple Stop.")
	}
}
