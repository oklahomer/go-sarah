/*
Package workers provides general purpose worker mechanism that outputs stacktrace when given job panics.
*/
package workers

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/v3/log"
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

// Stats represents a group of statistical data.
// This can be passed to Reporter.Report() to report current state.
type Stats struct {
	// QueueSize is the size of queued task to work.
	// Use this value to adjust Config.QueueSize.
	QueueSize int
}

// Reporter is an interface to report statistics such as queue length to outer service.
// Implement this to pass statistical variables in desired way.
// e.g. Report stats to prometheus via exporter
type Reporter interface {
	Report(context.Context, *Stats)
}

type reporter struct {
}

func (*reporter) Report(_ context.Context, stats *Stats) {
	log.Infof("Worker queue length: %d", stats.QueueSize)
}

// WorkerOption defines function that worker's functional option must satisfy.
type WorkerOption func(*worker)

// WithReporter creates and returns WorkerOption to set preferred Reporter implementation.
func WithReporter(reporter Reporter) WorkerOption {
	return func(w *worker) {
		w.reporter = reporter
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
// Worker implementation can be fed to sarah.RegisterWorker() to replace default implementation as below.
// Given worker is used on sarah.Run() call.
//
//   myWorker := NewMyWorkerImpl()
//   sarah.RegisterWorker(myWorker)
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
		opt(w)
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
		go supervise(ctx, w.reporter, incoming, config.SuperviseInterval)
	}

	return w, nil
}

func runChild(ctx context.Context, job <-chan func(), workerID uint) {
	log.Debugf("Start worker id: %d.", workerID)

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

func supervise(ctx context.Context, reporter Reporter, job chan<- func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			stats := &Stats{
				QueueSize: len(job),
			}
			reporter.Report(ctx, stats)

		}
	}
}
