package dev

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestCrossCompileSupportedLifecyclePlatforms is opt-in so ordinary focused
// tests stay fast. Release/CI qualification enables MSS_DEV_CROSS_COMPILE=1.
func TestCrossCompileSupportedLifecyclePlatforms(t *testing.T) {
	if os.Getenv("MSS_DEV_CROSS_COMPILE") != "1" {
		t.Skip("set MSS_DEV_CROSS_COMPILE=1 for lifecycle platform compile gates")
	}
	for _, target := range []struct {
		goos   string
		goarch string
	}{
		{goos: "darwin", goarch: "amd64"},
		{goos: "darwin", goarch: "arm64"},
		{goos: "windows", goarch: "amd64"},
	} {
		t.Run(target.goos+"-"+target.goarch, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			output := filepath.Join(t.TempDir(), "dev.test")
			if target.goos == "windows" {
				output += ".exe"
			}
			command := exec.CommandContext(ctx, "go", "test", "-run=^$", "-c", "-o", output, ".")
			command.Env = mergeEnvironment(os.Environ(), map[string]string{
				"CGO_ENABLED": "0",
				"GOARCH":      target.goarch,
				"GOFLAGS":     "-mod=readonly",
				"GOOS":        target.goos,
				"GOWORK":      "off",
			})
			combined, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("cross-compile %s/%s: %v\n%s", target.goos, target.goarch, err, combined)
			}
		})
	}
}
