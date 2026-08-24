//go:build windows

package dev

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsWindowsJunctionEscape(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  runtimeDirectory: .mss/run
  logDirectory: .mss/logs
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
`)
	junction := filepath.Join(root, ".mss", "run")
	external := t.TempDir()
	output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, external).CombinedOutput()
	if err != nil {
		t.Skipf("create Windows junction: %v: %s", err, output)
	}
	_, err = Load(root)
	if err == nil || !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("expected Windows junction escape to fail validation, got %v", err)
	}
}
