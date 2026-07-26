package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const instrumentationName = "github.com/PrakashSewani/NexusRelay/internal/observability"

type Runtime struct {
	logger           *slog.Logger
	registry         *prometheus.Registry
	httpMetric       *prometheus.CounterVec
	httpLatency      *prometheus.HistogramVec
	dependencyMetric *prometheus.HistogramVec
	dependencyReady  *prometheus.GaugeVec
	serviceReady     prometheus.Gauge
	provider         *sdktrace.TracerProvider
	tracer           trace.Tracer
	propagator       propagation.TextMapPropagator
}

func InstallBootstrapLogger(service string, output io.Writer) {
	logger, err := newLogger(output, service, "unknown", "info", "json")
	if err == nil {
		slog.SetDefault(logger)
	}
}

func New(ctx context.Context, service, version string, settings config.Observability, output io.Writer) (*Runtime, error) {
	logger, err := newLogger(output, service, version, settings.LogLevel, settings.LogFormat)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		logger:     logger,
		propagator: propagation.TraceContext{},
	}
	if settings.Metrics {
		runtime.configureMetrics(service, version)
	}
	if err := runtime.configureTracing(ctx, service, version, settings); err != nil {
		return nil, err
	}

	slog.SetDefault(logger)
	otel.SetTextMapPropagator(runtime.propagator)
	return runtime, nil
}

func (r *Runtime) Logger() *slog.Logger {
	return r.logger
}

func (r *Runtime) MetricsHandler() http.Handler {
	if r.registry == nil {
		return nil
	}
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (r *Runtime) SetReady(ready bool) {
	if r.serviceReady != nil {
		if ready {
			r.serviceReady.Set(1)
		} else {
			r.serviceReady.Set(0)
		}
	}
}

func (r *Runtime) ObserveDependency(name string, elapsed time.Duration, err error) {
	if r.dependencyMetric == nil {
		return
	}
	dependency := boundedDependency(name)
	outcome := "success"
	ready := 1.0
	if err != nil {
		outcome = "failure"
		ready = 0
	}
	r.dependencyMetric.WithLabelValues(dependency, outcome).Observe(elapsed.Seconds())
	r.dependencyReady.WithLabelValues(dependency).Set(ready)
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r.provider == nil {
		return nil
	}
	if err := r.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down trace provider: %w", err)
	}
	return nil
}

func (r *Runtime) configureMetrics(service, version string) {
	r.registry = prometheus.NewRegistry()
	r.httpMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   "nexusrelay",
		Name:        "http_requests_total",
		Help:        "Operational HTTP requests completed by route and status class.",
		ConstLabels: prometheus.Labels{"service": service, "version": version},
	}, []string{"route", "status"})
	r.httpLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "nexusrelay",
		Name:        "http_request_duration_seconds",
		Help:        "Operational HTTP request duration by route.",
		ConstLabels: prometheus.Labels{"service": service, "version": version},
		Buckets:     prometheus.DefBuckets,
	}, []string{"route"})
	r.dependencyMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "nexusrelay",
		Name:        "dependency_probe_duration_seconds",
		Help:        "Dependency readiness probe duration by bounded dependency and outcome.",
		ConstLabels: prometheus.Labels{"service": service, "version": version},
		Buckets:     prometheus.DefBuckets,
	}, []string{"dependency", "outcome"})
	r.dependencyReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "nexusrelay",
		Name:        "dependency_ready",
		Help:        "Whether the last readiness probe for a bounded dependency succeeded.",
		ConstLabels: prometheus.Labels{"service": service, "version": version},
	}, []string{"dependency"})
	r.serviceReady = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "nexusrelay",
		Name:        "service_ready",
		Help:        "Whether the service currently reports ready.",
		ConstLabels: prometheus.Labels{"service": service, "version": version},
	})
	r.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		r.httpMetric,
		r.httpLatency,
		r.dependencyMetric,
		r.dependencyReady,
		r.serviceReady,
	)
}

func (r *Runtime) configureTracing(ctx context.Context, service, version string, settings config.Observability) error {
	if !settings.OTel {
		provider := noop.NewTracerProvider()
		r.tracer = provider.Tracer(instrumentationName)
		otel.SetTracerProvider(provider)
		return nil
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(settings.OTLPEndpoint.String()),
		otlptracehttp.WithHTTPClient(httpClient),
		otlptracehttp.WithHeaders(map[string]string{}),
	)
	if err != nil {
		return fmt.Errorf("configure OTLP/HTTP trace exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", service),
		attribute.String("service.version", version),
	))
	if err != nil {
		return fmt.Errorf("configure trace resource: %w", err)
	}
	r.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	r.tracer = r.provider.Tracer(instrumentationName)
	otel.SetTracerProvider(r.provider)
	return nil
}

func boundedDependency(name string) string {
	switch name {
	case "postgresql", "redis":
		return name
	default:
		return "unknown"
	}
}
