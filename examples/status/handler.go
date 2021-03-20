package main

import (
	"encoding/json"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
	"net/http"
	"runtime"
)

// setStatusHandler sets an endpoint that returns current status of go-sarah, its belonging sarah.Bot implementations and sarah.Worker.
//
//	curl -s -XGET   "http://localhost:8080/status" | jq .
//	{
//    "worker": [
//      {
//        "report_time": "2018-06-23T15:22:37.274064679+09:00",
//        "queue_size": 0
//      },
//      {
//        "report_time": "2018-06-23T15:22:47.275251621+09:00",
//        "queue_size": 0
//      },
//      {
//        "report_time": "2018-06-23T15:22:57.272596709+09:00",
//        "queue_size": 0
//      },
//      {
//        "report_time": "2018-06-23T15:23:07.275004281+09:00",
//        "queue_size": 0
//      },
//      {
//        "report_time": "2018-06-23T15:23:17.276197523+09:00",
//        "queue_size": 0
//      }
//    ],
//	  "runtime": {
//	    "goroutine_count": 115,
//	    "cpu_count": 4,
//	    "gc_count": 1
//	  },
//	  "bot_system": {
//	    "running": true,
//	    "bots": [
//	      {
//	        "type": "nullBot",
//	        "running": true
//	      },
//	      {
//	        "type": "slack",
//	        "running": true
//	      }
//	    ]
//	  }
//	}
func setStatusHandler(mux *http.ServeMux, ws *workerStats) {
	mux.HandleFunc("/status", func(writer http.ResponseWriter, request *http.Request) {
		runnerStatus := sarah.CurrentStatus()
		systemStatus := &botSystemStatus{}
		systemStatus.Running = runnerStatus.Running
		for _, b := range runnerStatus.Bots {
			bs := &botStatus{
				BotType: b.Type,
				Running: b.Running,
			}
			systemStatus.Bots = append(systemStatus.Bots, bs)
		}

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		status := &status{
			Worker: ws.history(),
			Runtime: &runtimeStatus{
				NumGoroutine: runtime.NumGoroutine(),
				NumCPU:       runtime.NumCPU(),
				NumGC:        memStats.NumGC,
			},
			BotRunner: systemStatus,
		}
		bytes, err := json.Marshal(status)
		if err == nil {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write(bytes)
		} else {
			logger.Errorf("Failed to parse json: %+v", err)
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}

type status struct {
	Worker    []workerStatsElem `json:"worker"`
	Runtime   *runtimeStatus    `json:"runtime"`
	BotRunner *botSystemStatus  `json:"bot_system"`
}

type runtimeStatus struct {
	NumGoroutine int    `json:"goroutine_count"`
	NumCPU       int    `json:"cpu_count"`
	NumGC        uint32 `json:"gc_count"`
}

type botStatus struct {
	BotType sarah.BotType `json:"type"`
	Running bool          `json:"running"`
}

type botSystemStatus struct {
	Running bool         `json:"running"`
	Bots    []*botStatus `json:"bots"`
}
