package main

import (
	"context"
	"errors"
	"github.com/oklahomer/go-kasumi/logger"
	"net/http"
	"time"
)

// shutdownTimeout bounds how long Run waits for active connections to finish once ctx is canceled.
const shutdownTimeout = 5 * time.Second

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

// Run serves HTTP requests until ctx is canceled and then shuts the server down gracefully.
func (s *server) Run(ctx context.Context) {
	go func() {
		err := s.sv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("Failed to run HTTP server: %+v", err)
		}
	}()

	<-ctx.Done()

	// ctx is already canceled here, so a shutdown bound to it would return at once without draining active connections.
	// Derive a fresh deadline that ignores the cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	err := s.sv.Shutdown(shutdownCtx)
	if err != nil {
		logger.Errorf("Failed to stop HTTP server: %+v", err)
	}
}
