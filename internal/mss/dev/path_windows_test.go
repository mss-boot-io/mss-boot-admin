//go:build windows

package dev

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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

func TestVerifyStableConfinedPathAcceptsEquivalentWindowsShortPath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "stable-child")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create stable child: %v", err)
	}
	longRoot, err := windows.UTF16PtrFromString(root)
	if err != nil {
		t.Fatal(err)
	}
	required, err := windows.GetShortPathName(longRoot, nil, 0)
	if err != nil || required == 0 {
		t.Skipf("Windows short paths are unavailable: size=%d err=%v", required, err)
	}
	buffer := make([]uint16, required)
	written, err := windows.GetShortPathName(longRoot, &buffer[0], uint32(len(buffer)))
	if err != nil {
		t.Skipf("resolve Windows short path: %v", err)
	}
	shortRoot := windows.UTF16ToString(buffer[:written])
	if strings.EqualFold(shortRoot, root) {
		t.Skip("Windows volume did not produce a distinct 8.3 path")
	}
	if err := verifyStableConfinedPath(shortRoot, filepath.Join(shortRoot, "stable-child")); err != nil {
		t.Fatalf("equivalent Windows short path failed stable confinement: %v", err)
	}
}
