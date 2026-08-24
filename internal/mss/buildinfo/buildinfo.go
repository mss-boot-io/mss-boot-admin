// Package buildinfo exposes release metadata injected by the build workflow.
package buildinfo

import (
	"errors"
	"runtime/debug"
	"strings"
	"time"
)

const Repository = "mss-boot-io/mss-boot-admin"

const developmentVersion = "0.1.0-dev"

// Version and Commit are overridden with -ldflags for published binaries.
var (
	Version   = developmentVersion
	Commit    = "unknown"
	Timestamp = "unknown"
)

// Provenance is the immutable source identity carried by a release binary.
// Embedded Distribution operations deliberately use only values injected by
// the release build; debug.ReadBuildInfo is a display fallback, not authority.
type Provenance struct {
	Repository string
	Version    string
	Commit     string
	Timestamp  string
}

// VersionString returns the machine-readable version without build provenance.
func VersionString() string {
	version := strings.TrimSpace(Version)
	if version == "" || version == "devel" || version == developmentVersion {
		if info, ok := debug.ReadBuildInfo(); ok {
			candidate := strings.TrimSpace(info.Main.Version)
			if candidate != "" && candidate != "(devel)" {
				version = candidate
			}
		}
	}
	if version == "" {
		return "devel"
	}
	return version
}

// CommitString returns the exact build commit when one was injected. Unknown
// development provenance is represented as an empty value rather than a
// synthetic commit identity.
func CommitString() string {
	commit := strings.TrimSpace(Commit)
	if commit == "" || commit == "unknown" {
		commit, _ = cleanBuildSetting("vcs.revision")
	}
	return commit
}

// TimestampString returns the release timestamp, falling back to the Go VCS
// build setting only for human-facing version output.
func TimestampString() string {
	timestamp := strings.TrimSpace(Timestamp)
	if timestamp == "" || timestamp == "unknown" {
		timestamp, _ = cleanBuildSetting("vcs.time")
	}
	return timestamp
}

// ReleaseProvenance validates the values explicitly injected by a release
// build. It intentionally does not accept display fallbacks so an ad-hoc or
// dirty binary can never claim an embedded Foundation release identity.
func ReleaseProvenance() (Provenance, error) {
	provenance := Provenance{
		Repository: Repository,
		Version:    strings.TrimSpace(Version),
		Commit:     strings.ToLower(strings.TrimSpace(Commit)),
		Timestamp:  strings.TrimSpace(Timestamp),
	}
	var problems []string
	if provenance.Version == "" || provenance.Version == "devel" || strings.HasSuffix(provenance.Version, "-dev") {
		problems = append(problems, "release version is missing")
	}
	if len(provenance.Commit) != 40 || !isLowerHex(provenance.Commit) {
		problems = append(problems, "release commit must be a full 40-character hexadecimal commit")
	}
	parsed, err := time.Parse(time.RFC3339, provenance.Timestamp)
	if err != nil || parsed.Format(time.RFC3339) == "" {
		problems = append(problems, "release timestamp must be RFC3339")
	}
	if len(problems) > 0 {
		return Provenance{}, errors.New(strings.Join(problems, "; "))
	}
	return provenance, nil
}

// String returns a traceable version string for human-facing CLI surfaces.
func String() string {
	version := VersionString()
	commit := CommitString()
	timestamp := TimestampString()
	if commit == "" && timestamp == "" {
		return version
	}
	parts := make([]string, 0, 2)
	if commit != "" {
		parts = append(parts, "commit "+commit)
	}
	if timestamp != "" {
		parts = append(parts, "timestamp "+timestamp)
	}
	return version + " (" + strings.Join(parts, ", ") + ")"
}

func cleanBuildSetting(key string) (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	dirty := false
	value := ""
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.modified":
			dirty = setting.Value == "true"
		case key:
			value = strings.TrimSpace(setting.Value)
		}
	}
	if dirty || value == "" {
		return "", false
	}
	return value, true
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
