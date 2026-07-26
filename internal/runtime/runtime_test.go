package runtime

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/httpserver"
)

func TestRunWithoutReadinessServesLiveWhileRemainingNotReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	health := &httpserver.Health{}
	server := httpserver.New(listener.Addr().String(), health.Handler(), time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- RunWithoutReadiness(ctx, health.SetReady, func(serveContext context.Context) error {
			return server.Serve(serveContext, listener)
		})
	}()

	assertStatus(t, listener.Addr().String(), "/health/live", http.StatusOK)
	assertStatus(t, listener.Addr().String(), "/health/ready", http.StatusServiceUnavailable)
	if health.Ready() {
		t.Fatal("scaffold became ready without dependency initialization")
	}

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("RunWithoutReadiness() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scaffold did not shut down gracefully")
	}
}

func TestRunInitializedControlsReadinessAroundCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var mutex sync.Mutex
	var transitions []bool
	setReady := func(ready bool) {
		mutex.Lock()
		transitions = append(transitions, ready)
		mutex.Unlock()
	}
	runStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- RunInitialized(ctx, setReady, func(serveContext context.Context) error {
			close(runStarted)
			<-serveContext.Done()
			mutex.Lock()
			last := transitions[len(transitions)-1]
			mutex.Unlock()
			if last {
				return errors.New("cancellation arrived before readiness was cleared")
			}
			return nil
		})
	}()
	<-runStarted
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunInitialized did not propagate cancellation")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(transitions) < 2 || !transitions[0] || transitions[len(transitions)-1] {
		t.Fatalf("readiness transitions = %v", transitions)
	}
}

func TestRunInitializedNeverBecomesReadyWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	var transitions []bool
	err := RunInitialized(ctx, func(ready bool) { transitions = append(transitions, ready) }, func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunInitialized() error = %v", err)
	}
	if called {
		t.Fatal("run function called with canceled context")
	}
	if len(transitions) != 1 || transitions[0] {
		t.Fatalf("readiness transitions = %v", transitions)
	}
}

func assertStatus(t *testing.T, address, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		response, err := http.Get("http://" + address + path)
		if err == nil {
			response.Body.Close()
			if response.StatusCode != want {
				t.Fatalf("%s status = %d, want %d", path, response.StatusCode, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", path, err)
		}
		time.Sleep(time.Millisecond)
	}
}
