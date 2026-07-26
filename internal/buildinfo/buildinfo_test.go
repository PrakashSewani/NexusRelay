package buildinfo

import (
	"bytes"
	"testing"
)

func TestDefaultsAndEmptyValues(t *testing.T) {
	originalVersion, originalRevision := Version, Revision
	t.Cleanup(func() {
		Version, Revision = originalVersion, originalRevision
	})

	if got := GetVersion(); got != "dev" {
		t.Fatalf("GetVersion() = %q, want dev", got)
	}
	if got := GetRevision(); got != "unknown" {
		t.Fatalf("GetRevision() = %q, want unknown", got)
	}

	Version, Revision = "", ""
	if got := GetVersion(); got != "dev" {
		t.Fatalf("GetVersion() with empty value = %q, want dev", got)
	}
	if got := GetRevision(); got != "unknown" {
		t.Fatalf("GetRevision() with empty value = %q, want unknown", got)
	}
}

func TestWriteVersion(t *testing.T) {
	originalVersion, originalRevision := Version, Revision
	Version, Revision = "1.2.3", "abc123"
	t.Cleanup(func() {
		Version, Revision = originalVersion, originalRevision
	})

	for _, argument := range []string{"-v", "--version", "version"} {
		t.Run(argument, func(t *testing.T) {
			var output bytes.Buffer
			if !WriteVersion([]string{argument}, &output, "nexusrelay-test") {
				t.Fatal("WriteVersion() did not handle version argument")
			}
			if got, want := output.String(), "nexusrelay-test 1.2.3 (revision abc123)\n"; got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestWriteVersionIgnoresOtherArguments(t *testing.T) {
	for _, arguments := range [][]string{nil, {"--help"}, {"--version", "extra"}} {
		var output bytes.Buffer
		if WriteVersion(arguments, &output, "nexusrelay-test") {
			t.Fatalf("WriteVersion(%q) unexpectedly handled arguments", arguments)
		}
		if output.Len() != 0 {
			t.Fatalf("WriteVersion(%q) output = %q", arguments, output.String())
		}
	}
}
