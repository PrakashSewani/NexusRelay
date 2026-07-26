package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthSemantics(t *testing.T) {
	health := &Health{}
	handler := health.Handler()

	assertHealth(t, handler, "/health/live", http.StatusOK, `"live"`)
	assertHealth(t, handler, "/health/ready", http.StatusServiceUnavailable, `"not_ready"`)
	health.SetReady(true)
	assertHealth(t, handler, "/health/ready", http.StatusOK, `"ready"`)
	health.SetReady(false)
	assertHealth(t, handler, "/health/live", http.StatusOK, `"live"`)
}

func assertHealth(t *testing.T, handler http.Handler, path string, wantStatus int, wantBody string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s status = %d, want %d", path, response.Code, wantStatus)
	}
	if !strings.Contains(response.Body.String(), wantBody) {
		t.Fatalf("%s body = %q", path, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%s did not disable caching", path)
	}
}
