// +build !go1.8

package main

import (
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"net/http"
)

type server struct {
	sv *http.Server
}

func newServer(runner sarah.Runner, wsr *workerStats) *server {
	mux := http.NewServeMux()
	setStatusHandler(mux, runner, wsr)
	return &server{
		sv: &http.Server{Addr: ":8080", Handler: mux},
	}
}

func (s *server) Run(ctx context.Context) {
	go s.sv.ListenAndServe()

	<-ctx.Done()
	//s.sv.Shutdown(ctx)
}
