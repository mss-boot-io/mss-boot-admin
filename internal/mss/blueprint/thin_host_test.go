package blueprint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateThinHostSelectsOnlyApplicationTemplates(t *testing.T) {
	root := writeThinHostBlueprintFixture(t)
	destination := filepath.Join(t.TempDir(), "orders-admin")
	options := Options{
		FoundationRoot: root,
		Destination:    destination,
		Application: Application{
			Name:        "orders-admin",
			DisplayName: "Orders Administration",
			Module:      "github.com/acme/orders-admin",
			Repository:  "acme/orders-admin",
		},
	}

	plan, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("Generate(thin dry-run) error = %v", err)
	}
	if !plan.DryRun || !plan.Success || plan.BlueprintVersion != "0.4.0" {
		t.Fatalf("thin dry-run plan = %#v", plan)
	}
	if plan.Distribution.Version != "v1.3.0" || plan.Distribution.Frontend.Version != "1.3.0" {
		t.Fatalf("thin distribution = %#v", plan.Distribution)
	}
	if plan.TotalFiles > 12 {
		t.Fatalf("thin template unexpectedly selected %d files", plan.TotalFiles)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created destination: %v", err)
	}

	options.Write = true
	written, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("Generate(thin write) error = %v", err)
	}
	if written.DryRun || !written.Success {
		t.Fatalf("thin write plan = %#v", written)
	}
	assertContains(t, filepath.Join(destination, "go.mod"), "module github.com/acme/orders-admin")
	assertContains(t, filepath.Join(destination, "go.mod"), "github.com/mss-boot-io/mss-boot-admin/admin v1.3.0")
	assertContains(t, filepath.Join(destination, "go.mod"), "github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.0")
	assertContains(t, filepath.Join(destination, "cmd", "server", "main.go"), `adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"`)
	assertContains(t, filepath.Join(destination, "cmd", "server", "main.go"), `"github.com/acme/orders-admin/internal/modules/all"`)
	assertContains(t, filepath.Join(destination, "internal", "modules", "all", "generated.go"), `"github.com/mss-boot-io/mss-boot-admin/admin/business"`)
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "kind: thin-host")
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "version: v1.3.0")
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "module: github.com/mss-boot-io/mss-boot-admin/admin")
	for _, forbidden := range []string{"admin", "mss-boot", "web/antd-v6", "docs/package.json", ".mss/release-policy.yaml"} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(forbidden))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("thin host contains forbidden Foundation source %s: %v", forbidden, err)
		}
	}
	snapshot, err := ReadSnapshot(destination, "")
	if err != nil {
		t.Fatalf("ReadSnapshot(thin host) error = %v", err)
	}
	if snapshot.Lock.Spec.Distribution.Version != "v1.3.0" {
		t.Fatalf("thin lock distribution = %#v", snapshot.Lock.Spec.Distribution)
	}
	for managed := range snapshot.Manifest.Files {
		if strings.HasPrefix(managed, "templates/application/") {
			t.Fatalf("manifest leaked Foundation template source path %s", managed)
		}
	}

	second, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("Generate(thin repeat) error = %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("repeat thin generation changed %s: %s", change.Path, change.Action)
		}
	}
}

func TestApplicationTemplatePinsOneFrontendRuntimeWithoutPatches(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "templates", "application", "web", "package.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Thin Host package template: %v", err)
	}
	var document struct {
		DevDependencies map[string]string `json:"devDependencies"`
		Pnpm            struct {
			Overrides           map[string]string `json:"overrides"`
			PatchedDependencies map[string]string `json:"patchedDependencies"`
		} `json:"pnpm"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse Thin Host package template: %v", err)
	}
	expected := map[string]string{
		"react":                      "19.2.8",
		"react-dom":                  "19.2.8",
		"antd":                       "6.6.0",
		"@ant-design/pro-components": "3.1.14-6",
		"@tanstack/react-query":      "5.101.4",
		"axios":                      "0.33.0",
	}
	if len(document.Pnpm.Overrides) != len(expected) {
		t.Fatalf("Thin Host runtime overrides = %#v, want %#v", document.Pnpm.Overrides, expected)
	}
	for name, version := range expected {
		if document.Pnpm.Overrides[name] != version {
			t.Errorf("Thin Host override %s = %q, want %q", name, document.Pnpm.Overrides[name], version)
		}
	}
	if document.Pnpm.PatchedDependencies != nil {
		t.Fatalf("Thin Host must not inherit package patches: %#v", document.Pnpm.PatchedDependencies)
	}
	if document.DevDependencies["vite"] != "8.2.1" {
		t.Fatalf("Thin Host test Vite = %q, want 8.2.1", document.DevDependencies["vite"])
	}
}

func writeThinHostBlueprintFixture(t *testing.T) string {
	t.Helper()
	root := writeBlueprintFixture(t)
	writeFixtureFile(t, root, ".mss/blueprints/management-system.yaml", `apiVersion: mss.io/v1alpha1
kind: ApplicationBlueprint
metadata:
  name: management-system
  displayName: Thin Management System
  version: 0.4.0
spec:
  sourceMode: git-tracked
  templateRoot: templates/application
  sourceModule: github.com/mss-boot-io/mss-boot-admin
  sourceProjectName: mss-boot-admin
  distribution:
    name: mss-boot-admin
    version: v1.3.0
    backend:
      module: github.com/mss-boot-io/mss-boot-admin/admin
      version: v1.3.0
    frontend:
      package: "@mss-boot-io/admin-web"
      version: 1.3.0
  defaultOutputDirectory: .mss/output
  manifestPath: .mss/blueprint-manifest.json
  lockPath: .mss/lock.yaml
  requiredFiles: [AGENTS.md, go.mod, cmd/server/main.go, internal/modules/all/generated.go, .mss/project.yaml]
  textExtensions: [.go, .md, .mod, .yaml]
  textNames: [AGENTS.md]
`)
	writeFixtureFile(t, root, "templates/application/AGENTS.md", "# __MSS_APP_DISPLAY_NAME__\n")
	writeFixtureFile(t, root, "templates/application/go.mod", `module __MSS_APP_MODULE__

go 1.26.0

require (
	__MSS_DISTRIBUTION_BACKEND_MODULE__ __MSS_DISTRIBUTION_BACKEND_VERSION__
	github.com/mss-boot-io/mss-boot-admin/mss-boot __MSS_DISTRIBUTION_BACKEND_VERSION__
)
`)
	writeFixtureFile(t, root, "templates/application/cmd/server/main.go", `package main

import (
	adminapp "__MSS_DISTRIBUTION_BACKEND_MODULE__/app"
	_ "__MSS_APP_MODULE__/internal/modules/all"
)

func main() { _ = adminapp.ExecuteContext }
`)
	writeFixtureFile(t, root, "templates/application/internal/modules/all/generated.go", `package all

import "__MSS_DISTRIBUTION_BACKEND_MODULE__/business"

func Modules() []business.Module { return nil }
`)
	writeFixtureFile(t, root, "templates/application/.mss/project.yaml", `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: __MSS_APP_NAME__
  displayName: __MSS_APP_DISPLAY_NAME__
  repository: __MSS_APP_REPOSITORY__
spec:
  foundationVersion: 0.4.0
  distribution:
    name: __MSS_DISTRIBUTION_NAME__
    version: __MSS_DISTRIBUTION_VERSION__
    backend:
      module: __MSS_DISTRIBUTION_BACKEND_MODULE__
      version: __MSS_DISTRIBUTION_BACKEND_VERSION__
    frontend:
      package: "__MSS_DISTRIBUTION_FRONTEND_PACKAGE__"
      version: __MSS_DISTRIBUTION_FRONTEND_VERSION__
  repositoryLayout:
    kind: thin-host
    backend: .
    frontend: web
    modules: internal/modules
    generated: web/src/generated
    businessRoutes: web/config/business-routes.generated.ts
    specifications: .mss
  backend:
    module: __MSS_APP_MODULE__
`)
	releasePolicyPath := filepath.Join(root, ".mss", "release-policy.yaml")
	releasePolicy, err := os.ReadFile(releasePolicyPath)
	if err != nil {
		t.Fatalf("read Thin Host fixture release policy: %v", err)
	}
	releasePolicy = []byte(strings.ReplaceAll(string(releasePolicy), "v1.1.0", "v1.3.0"))
	if err := os.WriteFile(releasePolicyPath, releasePolicy, 0o644); err != nil {
		t.Fatalf("write Thin Host fixture release policy: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "test: add thin application templates")
	return root
}
