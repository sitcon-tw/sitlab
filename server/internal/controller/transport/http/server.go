package httpserver

import (
	"net/http"
	"time"
)

type ServerConfig struct {
	Addr string
}

func NewServer(cfg ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}
}
