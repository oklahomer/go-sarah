package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"sync"
	"testing"
	"time"
)

type DummyReporter struct {
	ReportQueueSizeFunc func(context.Context, int)
}

func (r *DummyReporter) ReportQueueSize(ctx context.Context, i int) {
	r.ReportQueueSizeFunc(ctx, i)
}

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

func TestNewConfig_UnmarshalJson(t *testing.T) {
	config := NewConfig()
	oldWorkerNum := config.WorkerNum
	oldQueueSize := config.QueueSize
	newQueueSize := oldQueueSize + 10
	jsonBytes := []byte(fmt.Sprintf(`{"queue_size": %d, "supervise_interval": 80}`, newQueueSize))

	if err := json.Unmarshal(jsonBytes, config); err != nil {
		t.Fatalf("Error on parsing given JSON structure: %s. %s.", string(jsonBytes), err.Error())
	}

	if config.QueueSize != newQueueSize {
		t.Errorf("QueueSize is not overridden with JSON value: %d.", config.QueueSize)
	}

	if config.SuperviseInterval != 80*time.Nanosecond {
		t.Errorf("SuperviseInterval is not overridden with JSON value: %d.", config.SuperviseInterval.Nanoseconds())
	}

	if config.WorkerNum != oldWorkerNum {
		t.Errorf("WorkerNum should stay when JSON value is not given: %d.", config.WorkerNum)
	}
}

func TestNewConfig_UnmarshalYaml(t *testing.T) {
	config := NewConfig()
	oldWorkerNum := config.WorkerNum
	oldQueueSize := config.QueueSize
	newQueueSize := oldQueueSize + 10
	newIntervalSec := 100
	yamlBytes := []byte(fmt.Sprintf("queue_size: %d\nsupervise_interval: %ds", newQueueSize, newIntervalSec))

	if err := yaml.Unmarshal(yamlBytes, config); err != nil {
		t.Fatalf("Error on parsing given YAML structure: %s. %s.", string(yamlBytes), err.Error())
	}

	if config.QueueSize != newQueueSize {
		t.Errorf("QueueSize is not overridden with YAML value: %d.", config.QueueSize)
	}

	if config.SuperviseInterval != time.Duration(newIntervalSec)*time.Second {
		t.Errorf("SuperviseInterval is not overridden with YAML value: %d.", config.SuperviseInterval)
	}

	if config.WorkerNum != oldWorkerNum {
		t.Errorf("WorkerNum should stay when YAML value is not given: %d.", config.WorkerNum)
	}
}

func TestWithReporter(t *testing.T) {
	reporter := &DummyReporter{}
	option := WithReporter(reporter)

	worker := &worker{}
	err := option(worker)

	if err != nil {
		t.Fatalf("Unexpected error occurred: %s.", err.Error())
	}

	if worker.reporter == nil {
		t.Error("Given reporter is not set")
	}
}

func TestRun(t *testing.T) {
	mutex := &sync.RWMutex{}

	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)
	defer cancelWorker()
	worker, err := Run(workerCtx, NewConfig())
	if err != nil {
		t.Fatalf("Unexpected error: %s.", err.Error())
	}

	isFinished := false
	worker.Enqueue(func() {
		mutex.Lock()
		defer mutex.Unlock()
		isFinished = true
	})

	time.Sleep(100 * time.Millisecond)
	func() {
		mutex.RLock()
		defer mutex.RUnlock()
		if isFinished == false {
			t.Fatal("Job is not executed.")
		}
	}()

	// panic won't affect main process
	worker.Enqueue(func() {
		panic("Panic! Catch me!!")
	})
	time.Sleep(100 * time.Millisecond)
}

func TestRun_ErrEnqueueAfterShutdown(t *testing.T) {
	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)

	worker, err := Run(workerCtx, NewConfig())
	if err != nil {
		t.Fatalf("Unexpected error: %s.", err.Error())
	}

	cancelWorker()
	time.Sleep(100 * time.Millisecond) // Wait til cancel is propagated to all worker goroutines.

	err = worker.Enqueue(func() {})

	if err != ErrEnqueueAfterWorkerShutdown {
		t.Errorf("Expected error is not returned: %T", err)
	}
}

func TestRun_ErrQueueOverflow(t *testing.T) {
	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)
	defer cancelWorker()

	config := NewConfig()
	config.QueueSize = 0
	config.WorkerNum = 1
	worker, err := Run(workerCtx, config)
	time.Sleep(100 * time.Millisecond) // Wait til worker goroutines are completely activated.

	// This job blocks the only available worker
	err = worker.Enqueue(func() {
		time.Sleep(3 * time.Second)
	})
	if err != nil {
		t.Fatalf("First enqueue should success: %s", err.Error())
	}

	// Next job should be blocked with no buffered channel.
	err = worker.Enqueue(func() {})
	if err != ErrQueueOverflow {
		t.Errorf("Expected error is not returned: %T", err)
	}
}

func TestRun_WorkerOption(t *testing.T) {
	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)
	defer cancelWorker()

	calledCnt := 0
	_, err := Run(workerCtx, NewConfig(), func(_ *worker) error {
		calledCnt++
		return nil
	})

	if err != nil {
		t.Fatalf("First enqueue should success: %s", err.Error())
	}

	if calledCnt != 1 {
		t.Error("Supplied WorkerOption is not called.")
	}
}

func TestRun_WorkerOptionError(t *testing.T) {
	rootCtx := context.Background()
	workerCtx, cancelWorker := context.WithCancel(rootCtx)
	defer cancelWorker()

	calledCnt := 0
	expectedErr := errors.New("expected error")
	_, err := Run(workerCtx, NewConfig(), func(_ *worker) error {
		calledCnt++
		return expectedErr
	})

	if err == nil {
		t.Fatal("Error is not returned.")
	}

	if err != expectedErr {
		t.Fatalf("Expected error is not returned: %s", err.Error())
	}
}

func Test_superviseQueueLength(t *testing.T) {
	job := make(chan func(), 10)
	for i := 0; i < cap(job); i++ {
		job <- func() {}
	}

	reportedSize := make(chan int, 1)
	reporter := &DummyReporter{
		ReportQueueSizeFunc: func(_ context.Context, i int) {
			reportedSize <- i
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	go superviseQueueLength(ctx, reporter, job, 1*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case size := <-reportedSize:
		if size != cap(job) {
			t.Errorf("Expected report size to be %d, but was %d.", cap(job), size)
		}
	case <-time.NewTimer(1 * time.Second).C:
		t.Fatal("Taking too long.")
	}
}
