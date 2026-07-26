package main

import (
	"bytes"
	"testing"
)

func TestVersionArgument(t *testing.T) {
	var output bytes.Buffer
	if !writeVersion([]string{"--version"}, &output) {
		t.Fatal("version argument was not handled")
	}
	if got, want := output.String(), "nexusrelay-control-plane dev (revision unknown)\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
