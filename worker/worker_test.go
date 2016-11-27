package worker

import (
	"golang.org/x/net/context"
	"sync"
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config.QueueSize == 0 {
		t.Error("Default value is not set for QueueSize.")
	}

	if config.SuperviseInterval == 0 {
		t.Error("Default value is not set for SuperviseInterval.")
	}

	if config.WorkerNum == 0 {
		t.Error("Default value is not set for WorkerNum.")
	}
}

func TestRun(t *testing.T) {
	mutex := &sync.RWMutex{}

	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)
	defer cancelWorker()
	job := Run(workerCtx, NewConfig())

	isFinished := false
	job <- func() {
		mutex.Lock()
		defer mutex.Unlock()
		isFinished = true
	}

	time.Sleep(100 * time.Millisecond)
	func() {
		mutex.RLock()
		defer mutex.RUnlock()
		if isFinished == false {
			t.Fatal("Job is not executed.")
		}
	}()

	// panic won't affect main process
	job <- func() {
		panic("Panic! Catch me!!")
	}
	time.Sleep(100 * time.Millisecond)
}
