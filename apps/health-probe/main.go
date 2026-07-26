package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const defaultProbeURL = "http://127.0.0.1:8080/health/ready"

func main() {
	if err := run(os.Args[1:], os.Stderr, http.DefaultClient); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("nexusrelay-health-probe", flag.ContinueOnError)
	flags.SetOutput(output)
	probeURL := flags.String("url", defaultProbeURL, "health endpoint URL")
	timeout := flags.Duration("timeout", 3*time.Second, "probe timeout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, *probeURL, nil)
	if err != nil {
		return fmt.Errorf("health probe URL is invalid")
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("health probe request failed")
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close health probe response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health probe returned status %d", response.StatusCode)
	}
	return nil
}
