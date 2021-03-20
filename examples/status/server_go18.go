// +build go1.8

package main

import (
	"context"
	"github.com/oklahomer/go-kasumi/logger"
	"net/http"
	"runtime"
)

type server struct {
	sv *http.Server
}

func newServer(wsr *workerStats) *server {
	mux := http.NewServeMux()
	setStatusHandler(mux, wsr)
	return &server{
		sv: &http.Server{Addr: ":8080", Handler: mux},
	}
}

func (s *server) Run(ctx context.Context) {
	runtime.Version()
	go s.sv.ListenAndServe()

	<-ctx.Done()
	err := s.sv.Shutdown(ctx)
	if err != nil {
		logger.Errorf("Failed to stop HTTP server: %+v", err)
	}
}
