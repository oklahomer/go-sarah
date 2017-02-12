package worker

import (
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"runtime"
	"time"
)

// Config contains some configuration variables.
// Use NewConfig to construct Config instance with default value and feed the instance to json.Unmarshal or yaml.Unmarshal to override.
type Config struct {
	WorkerNum         uint          `json:"worker_num" yaml:"worker_num"`
	QueueSize         uint          `json:"queue_size" yaml:"queue_size"`
	SuperviseInterval time.Duration `json:"supervise_interval" yaml:"supervise_interval"`
	Reporter          Reporter
}

// NewConfig returns Config instance with default configuration values.
// To override with desired value, pass the returned instance to json.Unmarshal or yaml.Unmarshal.
func NewConfig() *Config {
	return &Config{
		WorkerNum:         100,
		QueueSize:         10,
		SuperviseInterval: 60 * time.Second,

		// Reporter, by default, reports current statistics via logger with INFO level.
		// To modify this behavior implement Reporter interface and assign this manually.
		Reporter: &defaultReporter{
			reportQueueSizeFunc: func(_ context.Context, size int) {
				log.Infof("worker queue length: %d", size)
			},
		},
	}
}

// Reporter is an interface to report statistics such as queue length to outer service.
// Implement this to pass statistics variable to desired service.
// e.g. Report stats to prometheus via exporter
type Reporter interface {
	ReportQueueSize(context.Context, int)
}

type defaultReporter struct {
	reportQueueSizeFunc func(context.Context, int)
}

func (r defaultReporter) ReportQueueSize(ctx context.Context, size int) {
	r.reportQueueSizeFunc(ctx, size)
}

// Run creates as many child workers as specified and start those child workers.
func Run(ctx context.Context, config *Config) chan<- func() {
	log.Infof("start workers")

	job := make(chan func(), config.QueueSize)

	var i uint
	for i = 1; i <= config.WorkerNum; i++ {
		go runChild(ctx, job, i)
	}

	if config.SuperviseInterval > 0 {
		go superviseQueueLength(ctx, config.Reporter, job, config.SuperviseInterval)
	}

	return job
}

func runChild(ctx context.Context, job <-chan func(), workerID uint) {
	log.Infof("start worker id: %d.", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Infof("stopping worker id: %d", workerID)
			return
		case job := <-job:
			log.Debugf("receiving job on worker: %d", workerID)
			// To avoid given job's panic affect later jobs, wrap them with recover.
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Warnf("panic in given job. recovered: %#v", r)

						// Display stack trace
						for depth := 0; ; depth++ {
							_, src, line, ok := runtime.Caller(depth)
							if !ok {
								break
							}
							log.Warnf(" -> depth:%d. file:%s. line:%d.", depth, src, line)
						}
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
