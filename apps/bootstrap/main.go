package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/PrakashSewani/NexusRelay/internal/buildinfo"
)

var errNotImplemented = errors.New("owner bootstrap is reserved for Phase 4 and is not implemented")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Printf("bootstrap stopped: %v", err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if buildinfo.WriteVersion(arguments, output, "nexusrelay-bootstrap") {
		return nil
	}
	if len(arguments) == 1 {
		switch arguments[0] {
		case "-h", "--help", "help":
			writeUsage(output)
			return nil
		case "owner":
			// Fail before reading command-only environment values or secret files.
			return errNotImplemented
		}
	}
	writeUsage(output)
	if len(arguments) == 0 {
		return errors.New("a bootstrap command is required")
	}
	return fmt.Errorf("unknown bootstrap command %q", arguments[0])
}

func writeUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: nexusrelay-bootstrap <command>")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  owner     Reserved initial-owner bootstrap command (not implemented)")
	fmt.Fprintln(output, "  help      Show this help")
	fmt.Fprintln(output, "  version   Show the binary version")
}
