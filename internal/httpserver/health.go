package httpserver

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
)

type Health struct {
	ready atomic.Bool
}

func (h *Health) SetReady(ready bool) {
	h.ready.Store(ready)
}

func (h *Health) Ready() bool {
	return h.ready.Load()
}

func (h *Health) Handler() http.Handler {
	router := chi.NewRouter()
	router.Get("/health/live", h.live)
	router.Get("/health/ready", h.readiness)
	return router
}

func (h *Health) live(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w, http.StatusOK, "live")
}

func (h *Health) readiness(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		writeHealth(w, http.StatusServiceUnavailable, "not_ready")
		return
	}
	writeHealth(w, http.StatusOK, "ready")
}

func writeHealth(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
