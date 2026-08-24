//go:build windows

package mcp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationPlanRejectsWindowsDriveRelativeDestination(t *testing.T) {
	root := t.TempDir()
	volume := filepath.VolumeName(root)
	if volume == "" {
		t.Fatal("Windows temporary directory has no volume")
	}
	if _, err := resolveApplicationPlanDestination(root, volume+"outside"); err == nil || !strings.Contains(err.Error(), "working root") {
		t.Fatalf("drive-relative application destination error = %v", err)
	}
}
