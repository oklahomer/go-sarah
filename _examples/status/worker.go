package main

import (
	"context"
	"github.com/oklahomer/go-kasumi/worker"
	"slices"
	"sync"
	"time"
)

var workerMutex = sync.RWMutex{}

type workerStats []workerStatsElem

func (ws *workerStats) Report(_ context.Context, s *worker.Stats) {
	workerMutex.Lock()
	defer workerMutex.Unlock()

	if len(*ws) >= 50 {
		*ws = (*ws)[1:]
	}

	val := workerStatsElem{
		ReportTime: time.Now(),
		QueueSize:  s.QueueSize,
	}

	*ws = append(*ws, val)
}

func (ws *workerStats) history() []workerStatsElem {
	workerMutex.RLock()
	defer workerMutex.RUnlock()

	return slices.Clone(*ws)
}

type workerStatsElem struct {
	ReportTime time.Time `json:"report_time"`
	QueueSize  int       `json:"queue_size"`
}
