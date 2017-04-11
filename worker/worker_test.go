package worker

import (
	"encoding/json"
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

func TestDefaultReporter_ReportQueueSize(t *testing.T) {
	reportedSize := 0
	reporter := &defaultReporter{
		reportQueueSizeFunc: func(_ context.Context, i int) {
			reportedSize = i
		},
	}

	reportingSize := 100
	reporter.ReportQueueSize(context.TODO(), reportingSize)

	if reportedSize != reportingSize {
		t.Errorf("Expected size %d, but was %d.", reportingSize, reportedSize)
	}
}
