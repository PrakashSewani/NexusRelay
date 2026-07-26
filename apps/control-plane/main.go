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
		log.Printf("control-plane stopped: %v", err)
		os.Exit(1)
	}
}

func writeVersion(arguments []string, output io.Writer) bool {
	return buildinfo.WriteVersion(arguments, output, "nexusrelay-control-plane")
}

func run(ctx context.Context) error {
	settings, err := config.LoadControlPlane()
	if err != nil {
		return err
	}
	health := &httpserver.Health{}
	server := httpserver.New(settings.HTTP.Address, health.Handler(), settings.HTTP.ShutdownGrace)
	return processruntime.RunWithoutReadiness(ctx, health.SetReady, server.Run)
}
