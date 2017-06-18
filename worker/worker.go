package worker

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"runtime"
	"strings"
	"time"
)

var (
	// ErrEnqueueAfterWorkerShutdown is returned when job is given after worker context cancellation.
	ErrEnqueueAfterWorkerShutdown = errors.New("job can not be enqueued after worker shutdown")

	// ErrQueueOverflow is returned when job is given, but all workers are busy and queue is full.
	ErrQueueOverflow = errors.New("queue is full")
)

// Config contains some configuration variables.
// Use NewConfig to construct Config instance with default value and feed the instance to json.Unmarshal or yaml.Unmarshal to override.
type Config struct {
	WorkerNum         uint          `json:"worker_num" yaml:"worker_num"`
	QueueSize         uint          `json:"queue_size" yaml:"queue_size"`
	SuperviseInterval time.Duration `json:"supervise_interval" yaml:"supervise_interval"`
}

// NewConfig returns Config instance with default configuration values.
// To override with desired value, pass the returned instance to json.Unmarshal or yaml.Unmarshal.
func NewConfig() *Config {
	return &Config{
		WorkerNum:         100,
		QueueSize:         10,
		SuperviseInterval: 60 * time.Second,
	}
}

// Reporter is an interface to report statistics such as queue length to outer service.
// Implement this to pass statistics variable to desired service.
// e.g. Report stats to prometheus via exporter
type Reporter interface {
	ReportQueueSize(context.Context, int)
}

type reporter struct {
}

// ReportQueueSize report current queue size.
func (r reporter) ReportQueueSize(ctx context.Context, size int) {
	log.Infof("worker queue length: %d", size)
}

// WorkerOption defines function that worker's functional option must satisfy.
type WorkerOption func(*worker) error

// WithReporter creates and returns WorkerOption to set preferred Reporter implementation.
func WithReporter(reporter Reporter) WorkerOption {
	return func(w *worker) error {
		w.reporter = reporter
		return nil
	}
}

type worker struct {
	reporter   Reporter
	enqueueFnc func(func()) error
}

func (w *worker) Enqueue(fnc func()) error {
	return w.enqueueFnc(fnc)
}

// Worker is an interface that all Worker implementation must satisfy.
// Worker implementation can be fed to sarah.Runner via sarah.RunnerOption as below.
//
//   myWorker := NewMyWorkerImpl()
//   option := sarah.WithWorker(myWorker)
//
//   runner, _ := sarah.NewRunner(sarah.NewConfig(), option)
type Worker interface {
	Enqueue(func()) error
}

// Run creates as many child workers as specified by *Config and start them.
// When Run completes, Worker is returned so jobs can be enqueued.
// Multiple calls to Run() creates multiple Worker with separate context, queue and child workers.
func Run(ctx context.Context, config *Config, options ...WorkerOption) (Worker, error) {
	incoming := make(chan func(), config.QueueSize)

	w := &worker{
		enqueueFnc: func(job func()) error {
			if err := ctx.Err(); err != nil {
				// Context is canceled.
				return ErrEnqueueAfterWorkerShutdown
			}

			// There is a chance that context is cancelled right after above ctx.Err() check.
			// That however should not be a major problem.
			select {
			case incoming <- job:
				return nil

			default:
				return ErrQueueOverflow

			}
		},
	}

	for _, opt := range options {
		err := opt(w)
		if err != nil {
			return nil, err
		}
	}

	log.Infof("Start spawning %d workers.", config.WorkerNum)
	var i uint
	for i = 1; i <= config.WorkerNum; i++ {
		go runChild(ctx, incoming, i)
	}
	log.Infof("End spawning %d workers.", config.WorkerNum)

	if config.SuperviseInterval > 0 {
		if w.reporter == nil {
			w.reporter = &reporter{}
		}
		go superviseQueueLength(ctx, w.reporter, incoming, config.SuperviseInterval)
	}

	return w, nil
}

func runChild(ctx context.Context, job <-chan func(), workerID uint) {
	log.Debug("Start worker id: %d.", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Stopping worker id: %d", workerID)
			return

		case job := <-job:
			log.Debugf("Receiving job on worker: %d", workerID)
			// To avoid given job's panic affect later jobs, wrap them with recover.
			func() {
				defer func() {
					if r := recover(); r != nil {
						stack := []string{fmt.Sprintf("panic in given job. recovered: %#v", r)}

						// Display stack trace
						for depth := 0; ; depth++ {
							_, src, line, ok := runtime.Caller(depth)
							if !ok {
								break
							}
							stack = append(stack, fmt.Sprintf(" -> depth:%d. file:%s. line:%d.", depth, src, line))
						}

						log.Warn(strings.Join(stack, "\n"))
					}
				}()

				job()
			}()
		}
	}
}

func superviseQueueLength(ctx context.Context, reporter Reporter, job chan<- func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			reporter.ReportQueueSize(ctx, len(job))
		}
	}
}
