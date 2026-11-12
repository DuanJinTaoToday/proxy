package server

import (
	"net/http"
	"time"
)

func NewHTTP(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: time.Second,
		ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: 30 * time.Second,
	}
}
