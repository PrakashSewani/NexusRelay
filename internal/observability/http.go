package observability

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func (r *Runtime) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		route := boundedRoute(request.Method, request.URL.Path)
		parent := r.propagator.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
		ctx, span := r.tracer.Start(parent, route, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String("http.request.method", boundedMethod(request.Method)),
			attribute.String("http.route", route),
		))
		defer span.End()

		writer := &statusWriter{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(writer, request.WithContext(ctx))
		elapsed := time.Since(started)
		status := boundedStatus(writer.status)
		span.SetAttributes(attribute.Int("http.response.status_code", writer.status))
		if writer.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "server_error")
		}
		if r.httpMetric != nil {
			r.httpMetric.WithLabelValues(route, status).Inc()
			r.httpLatency.WithLabelValues(route).Observe(elapsed.Seconds())
		}
		attributes := []any{"route", route, "status", writer.status, "duration_ms", elapsed.Milliseconds()}
		if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
			attributes = append(attributes, "trace_id", spanContext.TraceID().String())
		}
		r.logger.Log(ctx, slog.LevelInfo, "http.request.completed", attributes...)
	})
}

func boundedRoute(method, path string) string {
	switch {
	case method == http.MethodGet && path == "/health/live":
		return "health.live"
	case method == http.MethodGet && path == "/health/ready":
		return "health.ready"
	case method == http.MethodGet && path == "/metrics":
		return "metrics"
	default:
		return "unknown"
	}
}

func boundedMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	default:
		return "OTHER"
	}
}

func boundedStatus(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
