package buildinfo

import "testing"

func TestString(t *testing.T) {
	originalVersion, originalCommit := Version, Commit
	t.Cleanup(func() {
		Version, Commit = originalVersion, originalCommit
	})

	Version, Commit = "v1.0.0-rc.1", "f401308"
	if got, want := VersionString(), "v1.0.0-rc.1"; got != want {
		t.Fatalf("VersionString() = %q, want %q", got, want)
	}
	if got, want := String(), "v1.0.0-rc.1 (commit f401308)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}

	Version, Commit = "", ""
	if got, want := String(), "devel"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
