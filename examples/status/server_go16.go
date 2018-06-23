// +build !go1.8

package main

import (
	"golang.org/x/net/context"
	"net/http"
)

type server struct {
	sv *http.Server
}

func newServer(mux *http.ServeMux) *server {
	return &server{
		sv: &http.Server{Addr: ":8080", Handler: mux},
	}
}

func (s *server) Run(ctx context.Context) {
	go s.sv.ListenAndServe()

	<-ctx.Done()
	//s.sv.Shutdown(ctx)
}
