package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const redactedValue = "[REDACTED]"

var allowedEvents = map[string]struct{}{
	"http.request.completed":    {},
	"service.stopped":           {},
	"telemetry.shutdown.failed": {},
}

var allowedAttributes = map[string]struct{}{
	"service": {}, "version": {}, "event": {}, "request_id": {}, "trace_id": {},
	"organization_id": {}, "operation": {}, "route": {}, "status": {}, "duration_ms": {},
	"error_category": {}, "error_code": {}, "error_message": {}, "dependency": {}, "outcome": {},
}

var forbiddenKeyParts = []string{
	"authorization", "cookie", "password", "credential", "secret", "api_key", "apikey",
	"prompt", "completion", "tool_argument", "tool_result", "request_body", "response_body",
	"upstream_body", "image", "audio", "token",
}

type redactingHandler struct {
	next slog.Handler
}

type safeError struct {
	err error
}

func SafeError(err error) slog.LogValuer {
	return safeError{err: err}
}

func (e safeError) LogValue() slog.Value {
	if e.err == nil {
		return slog.StringValue("")
	}
	return slog.StringValue(e.err.Error())
}

func newLogger(output io.Writer, service, version, level, format string) (*slog.Logger, error) {
	var minimum slog.Level
	switch level {
	case "debug":
		minimum = slog.LevelDebug
	case "info":
		minimum = slog.LevelInfo
	case "warn":
		minimum = slog.LevelWarn
	case "error":
		minimum = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level")
	}

	options := &slog.HandlerOptions{Level: minimum, ReplaceAttr: normalizeBuiltInAttribute}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "text":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, fmt.Errorf("unsupported log format")
	}
	handler = &redactingHandler{next: handler}
	return slog.New(handler).With("service", service, "version", version), nil
}

func normalizeBuiltInAttribute(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "timestamp"
		attr.Value = slog.TimeValue(attr.Value.Time().UTC())
	case slog.LevelKey:
		attr.Key = "severity"
	case slog.MessageKey:
		attr.Key = "event"
	}
	return attr
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	message := record.Message
	if _, allowed := allowedEvents[message]; !allowed {
		message = redactedValue
	}
	redacted := slog.NewRecord(record.Time, record.Level, message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttribute(attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for index, attr := range attrs {
		redacted[index] = redactAttribute(attr)
	}
	return &redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

func redactAttribute(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	key := strings.ToLower(attr.Key)
	if _, allowed := allowedAttributes[key]; !allowed || containsForbiddenKeyPart(key) {
		return slog.String(attr.Key, redactedValue)
	}
	if attr.Value.Kind() == slog.KindAny {
		return slog.String(attr.Key, redactedValue)
	}
	return attr
}

func containsForbiddenKeyPart(key string) bool {
	for _, part := range forbiddenKeyParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}
