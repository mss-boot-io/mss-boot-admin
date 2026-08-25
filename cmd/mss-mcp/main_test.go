package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMCPServerForwardsContributorRegistry(t *testing.T) {
	const registry = "http://127.0.0.1:4873"
	server := newMCPServer(t.TempDir(), registry, io.Discard)
	if server.ContributorFrontendRegistryURL != registry {
		t.Fatalf("contributor registry = %q, want %q", server.ContributorFrontendRegistryURL, registry)
	}
}

func TestResolveWorkingRootDefaultsToEmptyCurrentDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	working := t.TempDir()
	if err := os.Chdir(working); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	root, err := resolveWorkingRoot("")
	if err != nil {
		t.Fatalf("resolveWorkingRoot() error = %v", err)
	}
	if root != filepath.Clean(working) {
		t.Fatalf("resolveWorkingRoot() = %q, want %q", root, working)
	}
}

func TestResolveWorkingRootRejectsFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveWorkingRoot(path); err == nil {
		t.Fatal("resolveWorkingRoot() accepted a regular file")
	}
}

func TestResolveWorkingRootKeepsProjectDiscoveryCompatibility(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	contract := filepath.Join(root, ".mss", "project.yaml")
	if err := os.MkdirAll(filepath.Dir(contract), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contract, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested", "directory")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	resolved, err := resolveWorkingRoot("")
	if err != nil {
		t.Fatalf("resolveWorkingRoot() error = %v", err)
	}
	if resolved != filepath.Clean(root) {
		t.Fatalf("resolveWorkingRoot() = %q, want project root %q", resolved, root)
	}
}
