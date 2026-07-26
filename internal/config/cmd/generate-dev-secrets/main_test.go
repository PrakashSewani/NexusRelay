package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeneratesWithoutDisclosingValues(t *testing.T) {
	directory := filepath.Join(t.TempDir(), ".local-secrets")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--output-dir", directory}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr.String())
	}
	if stdout.String() != "development secret inventory ready at "+directory+"\n" || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRejectsPositionalArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"unexpected"}, &stdout, &stderr); err == nil || !strings.Contains(stderr.String(), "unexpected positional arguments") {
		t.Fatalf("error = %v, stderr = %q", err, stderr.String())
	}
}
