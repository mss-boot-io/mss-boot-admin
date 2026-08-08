package pkg

import "testing"

func TestFormatBuildVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{name: "release", version: "v0.8.0", commit: "f401308", want: "v0.8.0 (commit f401308)"},
		{name: "development", version: "devel", commit: "unknown", want: "devel"},
		{name: "empty values", want: "devel"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatBuildVersion(test.version, test.commit); got != test.want {
				t.Fatalf("FormatBuildVersion(%q, %q) = %q, want %q", test.version, test.commit, got, test.want)
			}
		})
	}
}
