package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/buildinfo"
	"github.com/PrakashSewani/NexusRelay/internal/config"
	"github.com/PrakashSewani/NexusRelay/internal/dependency"
	"github.com/PrakashSewani/NexusRelay/internal/httpserver"
	"github.com/PrakashSewani/NexusRelay/internal/observability"
	processruntime "github.com/PrakashSewani/NexusRelay/internal/runtime"
)

func main() {
	if writeVersion(os.Args[1:], os.Stdout) {
		return
	}
	observability.InstallBootstrapLogger("worker", os.Stderr)
	if err := processruntime.Run(run); err != nil {
		slog.Error("service.stopped", "error_message", observability.SafeError(err))
		os.Exit(1)
	}
}

func writeVersion(arguments []string, output io.Writer) bool {
	return buildinfo.WriteVersion(arguments, output, "nexusrelay-worker")
}

func run(ctx context.Context) error {
	settings, err := config.LoadWorker()
	if err != nil {
		return err
	}
	telemetry, err := observability.New(ctx, "worker", settings.Shared.Version, settings.Observability, os.Stdout)
	if err != nil {
		return err
	}
	defer shutdownTelemetry(telemetry)

	health := &httpserver.Health{}
	server := httpserver.New(settings.HTTP.Address, telemetry.HTTPMiddleware(health.Handler(telemetry.MetricsHandler())), settings.HTTP.ShutdownGrace)
	policy := dependency.ReadinessPolicy{StartupTimeout: settings.Readiness.StartupTimeout, ProbeTimeout: settings.Readiness.ProbeTimeout, ProbeInterval: settings.Readiness.ProbeInterval, RetryMinimum: settings.Readiness.RetryMinimum, RetryMaximum: settings.Readiness.RetryMaximum}
	clients, err := dependency.Open(ctx, settings.Database, settings.Redis, policy.ProbeTimeout)
	if err != nil {
		return err
	}
	defer clients.Close()
	setReady := func(ready bool) { health.SetReady(ready); telemetry.SetReady(ready) }
	return dependency.Run(ctx, policy, clients.Probes(telemetry.ObserveDependency), setReady, server.Run)
}

func shutdownTelemetry(telemetry *observability.Runtime) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := telemetry.Shutdown(ctx); err != nil {
		telemetry.Logger().Error("telemetry.shutdown.failed", "error_message", observability.SafeError(err))
	}
}
