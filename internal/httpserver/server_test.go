package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServerDoesNotMutateReadiness(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	health := &Health{}
	server := New(listener.Addr().String(), health.Handler(), time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx, listener) }()

	assertRemoteStatus(t, listener.Addr().String(), http.StatusServiceUnavailable)
	health.SetReady(true)
	assertRemoteStatus(t, listener.Addr().String(), http.StatusOK)
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !health.Ready() {
		t.Fatal("transport changed process-owned readiness")
	}
}

func TestServerDrainsInFlightHandler(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(listener.Addr().String(), handler, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(ctx, listener) }()
	requestResult := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				requestErr = errors.New("unexpected response status")
			}
		}
		requestResult <- requestErr
	}()
	<-started
	cancel()
	select {
	case err := <-serveResult:
		t.Fatalf("server stopped before handler drained: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-requestResult; err != nil {
		t.Fatalf("in-flight request failed: %v", err)
	}
	if err := <-serveResult; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestServerGraceExpiryForcesHandlerCancellation(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	handler := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
		close(canceled)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(listener.Addr().String(), handler, 25*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(ctx, listener) }()
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			response.Body.Close()
		}
	}()
	<-started
	cancel()
	select {
	case err := <-serveResult:
		if err == nil || !strings.Contains(err.Error(), "shut down operational HTTP") {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not enforce shutdown grace")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("forced connection closure did not cancel handler")
	}
}

func TestServerRunReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := New(listener.Addr().String(), http.NotFoundHandler(), time.Second)
	if err := server.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "listen on operational HTTP address") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestServerServeReportsListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	server := New(listener.Addr().String(), http.NotFoundHandler(), time.Second)
	if err := server.Serve(context.Background(), listener); err == nil || !strings.Contains(err.Error(), "serve operational HTTP") {
		t.Fatalf("Serve() error = %v", err)
	}
}

func assertRemoteStatus(t *testing.T, address string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		response, err := http.Get("http://" + address + "/health/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode != want {
				t.Fatalf("readiness status = %d, want %d", response.StatusCode, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET readiness: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
}
