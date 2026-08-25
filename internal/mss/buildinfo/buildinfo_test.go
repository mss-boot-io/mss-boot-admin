package buildinfo

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	originalVersion, originalCommit, originalTimestamp := Version, Commit, Timestamp
	t.Cleanup(func() {
		Version, Commit, Timestamp = originalVersion, originalCommit, originalTimestamp
	})

	Version, Commit, Timestamp = "v1.0.0-rc.1", "f401308", "2026-08-25T12:34:56Z"
	if got, want := VersionString(), "v1.0.0-rc.1"; got != want {
		t.Fatalf("VersionString() = %q, want %q", got, want)
	}
	if got, want := String(), "v1.0.0-rc.1 (commit f401308, timestamp 2026-08-25T12:34:56Z)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got, want := CommitString(), "f401308"; got != want {
		t.Fatalf("CommitString() = %q, want %q", got, want)
	}

	Version, Commit, Timestamp = "", "", ""
	if got, want := String(), "devel"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got := CommitString(); got != "" {
		t.Fatalf("CommitString() = %q, want empty unknown provenance", got)
	}
}

func TestReleaseProvenanceRequiresExplicitCompleteIdentity(t *testing.T) {
	originalVersion, originalCommit, originalTimestamp := Version, Commit, Timestamp
	t.Cleanup(func() {
		Version, Commit, Timestamp = originalVersion, originalCommit, originalTimestamp
	})

	Version = "v1.3.3"
	Commit = strings.Repeat("a", 40)
	Timestamp = "2026-08-25T12:34:56+08:00"
	provenance, err := ReleaseProvenance()
	if err != nil {
		t.Fatalf("ReleaseProvenance() error = %v", err)
	}
	if provenance.Repository != Repository || provenance.Version != "v1.3.3" || provenance.Commit != strings.Repeat("a", 40) || provenance.Timestamp != Timestamp {
		t.Fatalf("ReleaseProvenance() = %#v", provenance)
	}

	for _, test := range []struct {
		name      string
		version   string
		commit    string
		timestamp string
		want      string
	}{
		{name: "development version", version: "0.1.0-dev", commit: strings.Repeat("a", 40), timestamp: "2026-08-25T12:34:56Z", want: "release version"},
		{name: "short commit", version: "v1.3.3", commit: "deadbeef", timestamp: "2026-08-25T12:34:56Z", want: "40-character"},
		{name: "invalid timestamp", version: "v1.3.3", commit: strings.Repeat("a", 40), timestamp: "yesterday", want: "RFC3339"},
	} {
		t.Run(test.name, func(t *testing.T) {
			Version, Commit, Timestamp = test.version, test.commit, test.timestamp
			if _, err := ReleaseProvenance(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ReleaseProvenance() error = %v, want %q", err, test.want)
			}
		})
	}
}
