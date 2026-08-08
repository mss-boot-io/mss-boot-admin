// Package buildinfo exposes release metadata injected by the build workflow.
package buildinfo

import "strings"

// Version and Commit are overridden with -ldflags for published binaries.
var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
)

// VersionString returns the machine-readable version without build provenance.
func VersionString() string {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "devel"
	}
	return version
}

// String returns a traceable version string for human-facing CLI surfaces.
func String() string {
	version := VersionString()
	commit := strings.TrimSpace(Commit)
	if commit == "" || commit == "unknown" {
		return version
	}
	return version + " (commit " + commit + ")"
}
