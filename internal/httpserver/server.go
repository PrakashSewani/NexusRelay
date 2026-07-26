package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Server struct {
	server        *http.Server
	shutdownGrace time.Duration
}

func New(address string, handler http.Handler, shutdownGrace time.Duration) *Server {
	return &Server{
		server: &http.Server{
			Addr:              address,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
		shutdownGrace: shutdownGrace,
	}
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on operational HTTP address: %w", err)
	}
	return s.Serve(ctx, listener)
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- s.server.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve operational HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), s.shutdownGrace)
	defer cancel()
	if err := s.server.Shutdown(shutdownContext); err != nil {
		_ = s.server.Close()
		<-serveResult
		return fmt.Errorf("shut down operational HTTP: %w", err)
	}
	if err := <-serveResult; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve operational HTTP: %w", err)
	}
	return nil
}
