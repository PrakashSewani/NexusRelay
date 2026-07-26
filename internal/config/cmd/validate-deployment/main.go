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
	secretRoot := flags.String("secret-root", "", "source directory corresponding to logical /run/secrets paths")
	cloudflareSecretRoot := flags.String("cloudflare-secret-root", "", "source directory for the optional Cloudflare and ACME secrets")
	coreOnly := flags.Bool("core-only", false, "validate the core deployment while optional ingress has a separate profile preflight")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		err := fmt.Errorf("unexpected positional arguments")
		fmt.Fprintf(stderr, "deployment configuration invalid: %v\n", err)
		return err
	}
	values, err := config.ReadEnvironmentFile(*environmentFile)
	if err == nil && *coreOnly {
		values["ENABLE_CLOUDFLARE_TUNNEL"] = "false"
		values["TLS_MODE"] = "disabled"
		values["ACME_EMAIL"] = ""
		values["ACME_DNS_PROVIDER"] = ""
	}
	if err == nil && *secretRoot != "" {
		values, err = config.ResolveDeploymentSecretRoot(values, *secretRoot)
	}
	if err == nil && *cloudflareSecretRoot != "" {
		values, err = config.ResolveCloudflareSecretRoot(values, *cloudflareSecretRoot)
	}
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
