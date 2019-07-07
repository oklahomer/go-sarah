package main

import (
	"context"
	"github.com/oklahomer/go-sarah/workers"
	"sync"
	"time"
)

var workerMutex = sync.RWMutex{}

type workerStats []workerStatsElem

func (ws *workerStats) Report(_ context.Context, s *workers.Stats) {
	workerMutex.Lock()
	defer workerMutex.Unlock()

	if len(*ws) >= 50 {
		_, *ws = (*ws)[0], (*ws)[1:]
	}

	val := workerStatsElem{
		ReportTime: time.Now(),
		QueueSize:  s.QueueSize,
	}

	*ws = append(*ws, val)
}

func (ws *workerStats) history() []workerStatsElem {
	stats := make([]workerStatsElem, len(*ws))
	copy(stats, *ws)
	return stats
}

type workerStatsElem struct {
	ReportTime time.Time `json:"report_time"`
	QueueSize  int       `json:"queue_size"`
}
