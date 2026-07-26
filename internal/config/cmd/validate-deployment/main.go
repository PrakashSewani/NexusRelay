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
	flags := flag.NewFlagSet("validate-deployment", flag.ContinueOnError)
	flags.SetOutput(stderr)
	environmentFile := flags.String("env-file", "", "path to the complete NexusRelay deployment environment file")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		err := fmt.Errorf("unexpected positional arguments")
		fmt.Fprintf(stderr, "deployment configuration invalid: %v\n", err)
		return err
	}
	values, err := config.ReadEnvironmentFile(*environmentFile)
	if err == nil {
		err = config.ValidateComplete(values)
	}
	if err != nil {
		fmt.Fprintf(stderr, "deployment configuration invalid: %v\n", err)
		return err
	}
	fmt.Fprintln(stdout, "deployment configuration valid")
	return nil
}
