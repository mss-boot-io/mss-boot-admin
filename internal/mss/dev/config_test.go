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
	if services[0].Health.LaunchHeader != defaultLaunchHeader || services[1].Health.LaunchHeader != defaultLaunchHeader {
		t.Fatalf("health launch header defaults were not normalized: %#v", services)
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

func TestLoadRejectsSymlinkOrReparseEscapes(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  runtimeDirectory: .mss/run
  logDirectory: .mss/logs
  services:
    - id: backend
      directory: external-service
      command: [go, run, .]
      required: true
`)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, "external-service")); err != nil {
		t.Skipf("create symlink or reparse point: %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected external service-directory link to fail validation")
	}
	if !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsRuntimeAndLogSymlinkEscapes(t *testing.T) {
	for _, relative := range []string{".mss/run", ".mss/logs"} {
		t.Run(filepath.Base(relative), func(t *testing.T) {
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
			if err := os.Symlink(t.TempDir(), filepath.Join(root, filepath.FromSlash(relative))); err != nil {
				t.Skipf("create symlink or reparse point: %v", err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), "escapes repository root") {
				t.Fatalf("expected %s escape to fail validation, got %v", relative, err)
			}
		})
	}
}

func TestEnsureDirectoriesRejectsAncestorSwappedToSymlink(t *testing.T) {
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
	config, err := Load(root)
	if err != nil {
		t.Fatalf("load development config: %v", err)
	}
	external := t.TempDir()
	originalMSS := filepath.Join(root, ".mss")
	parkedMSS := filepath.Join(root, ".mss-original")
	if err := os.Rename(originalMSS, parkedMSS); err != nil {
		t.Fatalf("park .mss directory: %v", err)
	}
	if err := os.Symlink(external, originalMSS); err != nil {
		t.Skipf("create symlink or reparse point: %v", err)
	}

	err = ensureDirectories(config)
	if err == nil {
		t.Fatal("expected swapped .mss ancestor to fail confinement")
	}
	if !strings.Contains(err.Error(), "symlink or reparse point") && !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(external, "run")); !os.IsNotExist(statErr) {
		t.Fatalf("directory creation escaped repository before failure: %v", statErr)
	}
}

func TestServiceDirectorySwapToSymlinkFailsRuntimeRevalidation(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: service
      command: [go, run, .]
      required: true
`)
	serviceDirectory := filepath.Join(root, "service")
	if err := os.MkdirAll(serviceDirectory, 0o755); err != nil {
		t.Fatalf("create service directory: %v", err)
	}
	config, err := Load(root)
	if err != nil {
		t.Fatalf("load development config: %v", err)
	}
	parked := filepath.Join(root, "service-original")
	if err := os.Rename(serviceDirectory, parked); err != nil {
		t.Fatalf("park service directory: %v", err)
	}
	if err := os.Symlink(t.TempDir(), serviceDirectory); err != nil {
		t.Skipf("create symlink or reparse point: %v", err)
	}
	service, _ := config.Service("backend")
	if err := verifyStableConfinedPath(config.Root, config.ResolveDirectory(service)); err == nil {
		t.Fatal("runtime revalidation accepted swapped service directory")
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

func TestLoadRejectsInitialAdministratorPasswordEnvironment(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      environment:
        mss_admin_initial_password: forbidden
      required: true
`)

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected one-use administrator password in dev config to fail validation")
	}
	if !strings.Contains(err.Error(), initialAdminPasswordEnvironment) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsConfiguredHealthNonceEnvironment(t *testing.T) {
	root := writeDevelopmentConfig(t, `apiVersion: mss.io/v1alpha1
kind: DevelopmentEnvironment
metadata:
  project: test-project
spec:
  services:
    - id: backend
      directory: .
      command: [go, run, .]
      environment:
        MSS_DEV_HEALTH_NONCE: fixed-is-not-allowed
      required: true
`)

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), healthNonceEnvironment) {
		t.Fatalf("expected configured health nonce to fail validation, got %v", err)
	}
}

func TestLoadRejectsInvalidHealthLaunchHeader(t *testing.T) {
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
      health:
        url: http://127.0.0.1:8080/healthz
        launchHeader: "bad header"
`)

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "launchHeader") {
		t.Fatalf("expected invalid launch header to fail validation, got %v", err)
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
