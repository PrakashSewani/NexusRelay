package buildinfo

import (
	"fmt"
	"io"
)

const (
	defaultVersion  = "dev"
	defaultRevision = "unknown"
)

// Version and Revision are set by the build with -ldflags -X.
var (
	Version  = defaultVersion
	Revision = defaultRevision
)

// GetVersion returns the compiled version or the development default.
func GetVersion() string {
	if Version == "" {
		return defaultVersion
	}
	return Version
}

// GetRevision returns the compiled revision or the unknown default.
func GetRevision() string {
	if Revision == "" {
		return defaultRevision
	}
	return Revision
}

// WriteVersion handles a standalone version argument and writes one concise line.
func WriteVersion(arguments []string, output io.Writer, name string) bool {
	if len(arguments) != 1 {
		return false
	}
	switch arguments[0] {
	case "-v", "--version", "version":
		fmt.Fprintf(output, "%s %s (revision %s)\n", name, GetVersion(), GetRevision())
		return true
	default:
		return false
	}
}
