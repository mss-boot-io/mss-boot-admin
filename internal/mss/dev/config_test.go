package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrdersDependencies(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  runtimeDirectory: .mss/run
  logDirectory: .mss/logs
  services:
    - id: frontend
      directory: web
      command: [pnpm, dev]
      required: true
      dependsOn: [backend]
      health:
        url: http://127.0.0.1:8001/
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
      health:
        url: http://127.0.0.1:8080/healthz
`)
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatalf("create web directory: %v", err)
	}

	config, err := Load(root)
	if err != nil {
		t.Fatalf("load development config: %v", err)
	}
	services, err := config.Services([]string{"frontend"})
	if err != nil {
		t.Fatalf("select frontend service: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected backend dependency and frontend, got %d", len(services))
	}
	if services[0].ID != "backend" || services[1].ID != "frontend" {
		t.Fatalf("unexpected service order: %#v", []string{services[0].ID, services[1].ID})
	}
	if config.StartupTimeout.String() != "1m30s" {
		t.Fatalf("unexpected default startup timeout: %s", config.StartupTimeout)
	}
}

func TestLoadRejectsEscapingPaths(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  runtimeDirectory: ../run
  logDirectory: .mss/logs
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
`)

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected escaping runtime directory to fail validation")
	}
	if !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsDependencyCycles(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
      dependsOn: [frontend]
    - id: frontend
      directory: .
      command: [pnpm, dev]
      required: true
      dependsOn: [backend]
`)

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected cyclic dependencies to fail validation")
	}
	if !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicesRejectsUnknownSelection(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
`)
	config, err := Load(root)
	if err != nil {
		t.Fatalf("load development config: %v", err)
	}
	if _, err := config.Services([]string{"missing"}); err == nil {
		t.Fatal("expected unknown service selection to fail")
	}
}

func TestStartServicesDefaultsToRequiredAndKeepsRollbackExplicit(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      required: true
    - id: frontend
      directory: .
      command: [pnpm, dev]
      required: true
      dependsOn: [backend]
    - id: worker
      directory: .
      command: [go, run, ./cmd/worker]
      required: false
      dependsOn: [backend]
`)
	config, err := Load(root)
	if err != nil {
		t.Fatalf("load development config: %v", err)
	}
	services, err := config.StartServices(nil)
	if err != nil {
		t.Fatalf("select default services: %v", err)
	}
	if len(services) != 2 || services[0].ID != "backend" || services[1].ID != "frontend" {
		t.Fatalf("default services = %#v, want backend and frontend", services)
	}
	optional, err := config.StartServices([]string{"worker"})
	if err != nil {
		t.Fatalf("select optional service: %v", err)
	}
	if len(optional) != 2 || optional[0].ID != "backend" || optional[1].ID != "worker" {
		t.Fatalf("optional services = %#v, want backend and worker", optional)
	}
}

func writeDevelopmentConfig(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, ".mss")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create .mss directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "dev.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write development config: %v", err)
	}
	return root
}
