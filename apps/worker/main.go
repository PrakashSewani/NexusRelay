package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/PrakashSewani/NexusRelay/internal/buildinfo"
	"github.com/PrakashSewani/NexusRelay/internal/config"
	"github.com/PrakashSewani/NexusRelay/internal/httpserver"
	processruntime "github.com/PrakashSewani/NexusRelay/internal/runtime"
)

func main() {
	if writeVersion(os.Args[1:], os.Stdout) {
		return
	}
	if err := processruntime.Run(run); err != nil {
		log.Printf("worker stopped: %v", err)
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

	// The worker has no jobs in Phase 1. This listener is operational-only and
	// exposes liveness/readiness so orchestration can manage the empty runtime.
	health := &httpserver.Health{}
	server := httpserver.New(settings.HTTP.Address, health.Handler(), settings.HTTP.ShutdownGrace)
	return processruntime.RunWithoutReadiness(ctx, health.SetReady, server.Run)
}
