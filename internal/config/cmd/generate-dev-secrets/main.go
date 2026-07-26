package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/PrakashSewani/NexusRelay/internal/config"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("generate-dev-secrets", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputDirectory := flags.String("output-dir", config.DevSecretDirectoryName, "directory for the exact development secret inventory")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		err := fmt.Errorf("unexpected positional arguments")
		if _, writeErr := fmt.Fprintf(stderr, "development secret generation failed: %v\n", err); writeErr != nil {
			return fmt.Errorf("write development secret generation error: %w", writeErr)
		}
		return err
	}
	if err := config.GenerateDevSecrets(*outputDirectory); err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "development secret generation failed: %v\n", err); writeErr != nil {
			return fmt.Errorf("write development secret generation error: %w", writeErr)
		}
		return err
	}
	if _, err := fmt.Fprintf(stdout, "development secret inventory ready at %s\n", *outputDirectory); err != nil {
		return fmt.Errorf("write development secret generation result: %w", err)
	}
	return nil
}
