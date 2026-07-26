package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/config"
	"go.opentelemetry.io/otel/trace"
)

const privacySentinel = "nexusrelay-privacy-sentinel-do-not-export"

func TestLoggerRedactsPrivacySentinelsBeforeSerialization(t *testing.T) {
	var output bytes.Buffer
	logger, err := newLogger(&output, "gateway", "test", "debug", "json")
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("request.completed",
		"route", "health.ready",
		"authorization", privacySentinel,
		"prompt", privacySentinel,
		"error", privacySentinel,
		"unknown", privacySentinel,
	)
	logger.Info(privacySentinel, "status", 200)
	if strings.Contains(output.String(), privacySentinel) {
		t.Fatalf("structured logs disclosed privacy sentinel: %s", output.String())
	}
	if strings.Count(output.String(), redactedValue) < 5 {
		t.Fatalf("expected centralized redaction: %s", output.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log is not JSON: %v", err)
		}
		for _, field := range []string{"timestamp", "severity", "service", "version", "event"} {
			if _, ok := record[field]; !ok {
				t.Fatalf("log omitted %s: %v", field, record)
			}
		}
		if timestamp, ok := record["timestamp"].(string); !ok || !strings.HasSuffix(timestamp, "Z") {
			t.Fatalf("log timestamp is not UTC: %v", record["timestamp"])
		}
	}
}

func TestSafeErrorRequiresExplicitBoundary(t *testing.T) {
	var output bytes.Buffer
	logger, err := newLogger(&output, "gateway", "test", "info", "json")
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("service.stopped", "error_message", SafeError(context.Canceled))
	if !strings.Contains(output.String(), context.Canceled.Error()) {
		t.Fatalf("explicitly safe error was not logged: %s", output.String())
	}
	output.Reset()
	logger.Error("service.stopped", "error", errors.New(privacySentinel))
	if strings.Contains(output.String(), privacySentinel) {
		t.Fatalf("raw error bypassed redaction: %s", output.String())
	}
}

func TestMetricsUseOnlyBoundedLabels(t *testing.T) {
	runtime, err := New(context.Background(), "gateway", "test", config.Observability{LogLevel: "error", LogFormat: "json", Metrics: true}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ObserveDependency(privacySentinel, time.Millisecond, context.Canceled)
	handler := runtime.HTTPMiddleware(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/"+privacySentinel, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	metricResponse := httptest.NewRecorder()
	runtime.MetricsHandler().ServeHTTP(metricResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricResponse.Body.String()
	if strings.Contains(body, privacySentinel) {
		t.Fatalf("metrics disclosed unbounded label value: %s", body)
	}
	for _, label := range []string{`dependency="unknown"`, `route="unknown"`, `status="4xx"`} {
		if !strings.Contains(body, label) {
			t.Fatalf("metrics omitted bounded label %s", label)
		}
	}
}

func TestDisabledTracingMakesNoOutboundRequest(t *testing.T) {
	var requests int
	collector := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer collector.Close()
	endpoint := mustURL(t, collector.URL+"/v1/traces")
	runtime, err := New(context.Background(), "worker", "test", config.Observability{LogLevel: "error", LogFormat: "json", OTLPEndpoint: endpoint}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	runtime.HTTPMiddleware(http.NotFoundHandler()).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("disabled tracing sent %d requests", requests)
	}
}

func TestW3CExtractionAndOTLPExportExcludeSensitiveHTTPData(t *testing.T) {
	var mutex sync.Mutex
	var bodies [][]byte
	var headers []http.Header
	collector := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		mutex.Lock()
		bodies = append(bodies, body)
		headers = append(headers, request.Header.Clone())
		mutex.Unlock()
		response.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization="+privacySentinel)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "Cookie="+privacySentinel)

	runtime, err := New(context.Background(), "control-plane", "test", config.Observability{
		LogLevel: "error", LogFormat: "json", OTel: true, OTLPEndpoint: mustURL(t, collector.URL+"/v1/traces"),
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	const traceID = "0af7651916cd43dd8448eb211c80319c"
	request := httptest.NewRequest(http.MethodGet, "/health/live", strings.NewReader(privacySentinel))
	request.Header.Set("traceparent", "00-"+traceID+"-b7ad6b7169203331-01")
	request.Header.Set("tracestate", "vendor=value")
	request.Header.Set("Authorization", "Bearer "+privacySentinel)
	request.Header.Set("Cookie", "session="+privacySentinel)
	request.Header.Set("X-Prompt", privacySentinel)
	var continued trace.TraceID
	runtime.HTTPMiddleware(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		continued = trace.SpanContextFromContext(request.Context()).TraceID()
		response.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(httptest.NewRecorder(), request)

	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runtime.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	if continued.String() != traceID {
		t.Fatalf("continued trace ID = %s, want %s", continued, traceID)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(bodies) == 0 {
		t.Fatal("OTLP collector received no trace export")
	}
	for index, body := range bodies {
		if bytes.Contains(body, []byte(privacySentinel)) {
			t.Fatalf("trace export %d disclosed sensitive data", index)
		}
		if headers[index].Get("Authorization") != "" || headers[index].Get("Cookie") != "" {
			t.Fatalf("trace export used ambient authentication headers: %v", headers[index])
		}
	}
}

func TestLoggerRejectsUnsupportedConfiguration(t *testing.T) {
	for _, settings := range []struct{ level, format string }{{"verbose", "json"}, {"info", "xml"}} {
		if _, err := newLogger(io.Discard, "gateway", "test", settings.level, settings.format); err == nil {
			t.Fatalf("accepted level=%s format=%s", settings.level, settings.format)
		}
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
