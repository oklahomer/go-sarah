package sarah

import "time"

type WorkerConfig struct {
	workerNum         uint
	queueSize         uint
	superviseInterval time.Duration
}

func NewWorkerConfig(workerNum, queueSize uint, superviseInterval time.Duration) *WorkerConfig {
	return &WorkerConfig{
		workerNum:         workerNum,
		queueSize:         queueSize,
		superviseInterval: superviseInterval,
	}
}

type Config struct {
	worker *WorkerConfig
}

func NewConfig() *Config {
	return &Config{
		worker: NewWorkerConfig(100, 10, 60*time.Second), // default
	}
}

func (config *Config) WorkerConfig(wc *WorkerConfig) {
	config.worker = wc
}
