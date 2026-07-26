package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunSucceedsOnlyForOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	if err := run([]string{"-url", server.URL}, &bytes.Buffer{}, server.Client()); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsUnhealthyAndInvalidInputs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	if err := run([]string{"-url", server.URL}, &bytes.Buffer{}, server.Client()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("error = %v", err)
	}
	if err := run([]string{"-url", "://invalid"}, &bytes.Buffer{}, http.DefaultClient); err == nil {
		t.Fatal("invalid URL accepted")
	}
	if err := run([]string{"-timeout", "0s"}, &bytes.Buffer{}, http.DefaultClient); err == nil {
		t.Fatal("zero timeout accepted")
	}
}

func TestRunHonorsTimeoutWithoutDisclosingTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(50 * time.Millisecond) }))
	defer server.Close()
	err := run([]string{"-url", server.URL, "-timeout", "1ms"}, &bytes.Buffer{}, server.Client())
	if err == nil || err.Error() != "health probe request failed" {
		t.Fatalf("error = %v", err)
	}
}
