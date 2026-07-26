package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHelpAndVersion(t *testing.T) {
	for _, test := range []struct {
		argument string
		want     string
	}{
		{argument: "--help", want: "Usage: nexusrelay-bootstrap"},
		{argument: "--version", want: "nexusrelay-bootstrap dev (revision unknown)\n"},
	} {
		t.Run(test.argument, func(t *testing.T) {
			var output bytes.Buffer
			if err := run([]string{test.argument}, &output); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if test.argument == "--version" && output.String() != test.want {
				t.Fatalf("output = %q, want %q", output.String(), test.want)
			}
			if test.argument != "--version" && !strings.Contains(output.String(), test.want) {
				t.Fatalf("output = %q", output.String())
			}
		})
	}
}

func TestOwnerFailsBeforeReadingInputs(t *testing.T) {
	t.Setenv("BOOTSTRAP_PASSWORD_FILE", "/path/that/must/not/be/read")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "plaintext-owner@example.com")
	if err := run([]string{"owner"}, &bytes.Buffer{}); !errors.Is(err, errNotImplemented) {
		t.Fatalf("run() error = %v", err)
	}
}

func TestCommandValidation(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("run() without a command unexpectedly succeeded")
	}
	if err := run([]string{"unknown"}, &bytes.Buffer{}); err == nil {
		t.Fatal("run() with an unknown command unexpectedly succeeded")
	}
}
