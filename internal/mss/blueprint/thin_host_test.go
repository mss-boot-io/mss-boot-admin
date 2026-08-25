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

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
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
	if plan.TotalFiles > 20 {
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
	assertContains(t, filepath.Join(destination, "cmd", "server", "main.go"), `"github.com/acme/orders-admin/internal/modules"`)
	assertContains(t, filepath.Join(destination, "internal", "modules", "registry.go"), `"github.com/acme/orders-admin/internal/modules/custom"`)
	assertContains(t, filepath.Join(destination, "internal", "modules", "registry.go"), "append(all.Modules(), custom.Modules()...)")
	assertContains(t, filepath.Join(destination, "internal", "modules", "all", "generated.go"), `"github.com/mss-boot-io/mss-boot-admin/admin/business"`)
	assertContains(t, filepath.Join(destination, "internal", "modules", "custom", "modules.go"), "return []business.Module{}")
	assertContains(t, filepath.Join(destination, "web", "config", "business-routes.ts"), "...customBusinessRoutes")
	assertContains(t, filepath.Join(destination, "web", "src", "route-registrations.ts"), "duplicate business UI route path")
	assertContains(t, filepath.Join(destination, "web", "src", "route-registrations.ts"), "duplicate business server route path")
	assertContains(t, filepath.Join(destination, "web", "src", "locales", "zh-CN.ts"), "../business/locales/zh-CN")
	assertContains(t, filepath.Join(destination, "web", "src", "locales", "en-US.ts"), "../business/locales/en-US")
	assertContains(t, filepath.Join(destination, "web", "src", "business", "locales", "zh-CN.ts"), "export default messages")
	assertContains(t, filepath.Join(destination, "web", "src", "business", "locales", "en-US.ts"), "export default messages")
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "kind: thin-host")
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "foundationVersion: 0.1.0")
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

func TestGenerateThinHostQuotesDisplayNameOnlyInYAMLContext(t *testing.T) {
	root := writeThinHostBlueprintFixture(t)
	tests := []struct {
		name        string
		displayName string
	}{
		{name: "colon", displayName: "ACME: Admin"},
		{name: "hash", displayName: "ACME # Admin"},
		{name: "single-quote", displayName: "Owner's Admin"},
		{name: "double-quote", displayName: `ACME "Admin"`},
		{name: "newline", displayName: "ACME\nAdmin"},
		{name: "unicode", displayName: "示例管理后台"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applicationName := "display-" + strings.ReplaceAll(test.name, "-", "")
			destination := filepath.Join(t.TempDir(), applicationName)
			_, err := Generate(context.Background(), Options{
				FoundationRoot: root,
				Destination:    destination,
				Write:          true,
				Application: Application{
					Name:        applicationName,
					DisplayName: test.displayName,
					Module:      "github.com/acme/" + applicationName,
					Repository:  "acme/" + applicationName,
				},
			})
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			projectData, err := os.ReadFile(filepath.Join(destination, ".mss", "project.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			document, err := project.DecodeProjectDocument(projectData)
			if err != nil {
				t.Fatalf("DecodeProjectDocument() error = %v\n%s", err, projectData)
			}
			if document.Metadata.DisplayName != test.displayName {
				t.Fatalf("displayName = %q, want %q", document.Metadata.DisplayName, test.displayName)
			}
			readme, err := os.ReadFile(filepath.Join(destination, "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(readme), "# "+test.displayName+"\n"; got != want {
				t.Fatalf("AGENTS.md = %q, want raw human-readable value %q", got, want)
			}
		})
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
		"antd":                       "6.6.1",
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
	if document.DevDependencies["vite"] != "8.2.2" {
		t.Fatalf("Thin Host test Vite = %q, want 8.2.2", document.DevDependencies["vite"])
	}
}

func TestApplicationTemplateSeparatesManagedFacadesFromBusinessOwnedRegistries(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
	blueprint, err := Load(root, "management-system")
	if err != nil {
		t.Fatalf("load management-system Blueprint: %v", err)
	}
	if blueprint.Metadata.Version != "0.6.0" {
		t.Fatalf("management-system Blueprint version = %q, want 0.6.0", blueprint.Metadata.Version)
	}

	managed := []string{
		"templates/application/internal/modules/registry.go.tmpl",
		"templates/application/web/config/business-routes.ts",
		"templates/application/web/src/route-registrations.ts",
		"templates/application/web/src/locales/zh-CN.ts",
		"templates/application/web/src/locales/en-US.ts",
	}
	for _, relative := range managed {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil {
			t.Fatalf("read managed facade %s: %v", relative, readErr)
		}
		if !strings.Contains(string(data), "Code generated by mss application template") {
			t.Errorf("managed facade %s has no generated header", relative)
		}
	}

	businessOwned := []string{
		"templates/application/internal/modules/custom/modules.go.tmpl",
		"templates/application/web/src/business/routes.config.ts",
		"templates/application/web/src/business/route-registrations.ts",
		"templates/application/web/src/business/locales/zh-CN.ts",
		"templates/application/web/src/business/locales/en-US.ts",
	}
	for _, relative := range businessOwned {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil {
			t.Fatalf("read business-owned registry %s: %v", relative, readErr)
		}
		if strings.Contains(string(data), "Code generated by mss") || strings.Contains(string(data), "DO NOT EDIT") {
			t.Errorf("business-owned registry %s carries a generated-file header", relative)
		}
	}

	for _, locale := range []string{"zh-CN", "en-US"} {
		relative := "templates/application/web/src/locales/" + locale + ".ts"
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil {
			t.Fatalf("read managed locale facade %s: %v", relative, readErr)
		}
		content := string(data)
		for _, importPath := range []string{
			"/runtime/locales/" + locale,
			"../generated/locales/" + locale,
			"../business/locales/" + locale,
		} {
			if !strings.Contains(content, importPath) {
				t.Fatalf("managed locale facade %s does not import %q", relative, importPath)
			}
		}
		previous := -1
		for _, marker := range []string{
			"...coreMessages",
			"...generatedMessages",
			"...customMessages",
		} {
			current := strings.Index(content, marker)
			if current < 0 || current <= previous {
				t.Fatalf("managed locale facade %s does not compose core -> generated -> custom at %q", relative, marker)
			}
			previous = current
		}
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
  requiredFiles: [AGENTS.md, go.mod, cmd/server/main.go, internal/modules/registry.go, internal/modules/all/generated.go, internal/modules/custom/modules.go, web/config/business-routes.ts, web/src/route-registrations.ts, web/src/business/routes.config.ts, web/src/business/route-registrations.ts, web/src/locales/zh-CN.ts, web/src/locales/en-US.ts, web/src/business/locales/zh-CN.ts, web/src/business/locales/en-US.ts, .mss/project.yaml]
  textExtensions: [.go, .md, .mod, .ts, .yaml]
  textNames: [AGENTS.md]
`)
	writeFixtureFile(t, root, "templates/application/AGENTS.md", "# __MSS_APP_DISPLAY_NAME__\n")
	writeFixtureFile(t, root, "templates/application/go.mod.tmpl", `module __MSS_APP_MODULE__

go 1.26.0

require (
	__MSS_DISTRIBUTION_BACKEND_MODULE__ __MSS_DISTRIBUTION_BACKEND_VERSION__
	github.com/mss-boot-io/mss-boot-admin/mss-boot __MSS_DISTRIBUTION_BACKEND_VERSION__
)
`)
	writeFixtureFile(t, root, "templates/application/cmd/server/main.go.tmpl", `package main

import (
	adminapp "__MSS_DISTRIBUTION_BACKEND_MODULE__/app"
	"__MSS_APP_MODULE__/internal/modules"
)

func main() { _ = adminapp.ExecuteContext; _ = modules.Modules }
`)
	writeFixtureFile(t, root, "templates/application/internal/modules/registry.go.tmpl", `package modules

import (
	"__MSS_APP_MODULE__/internal/modules/all"
	"__MSS_APP_MODULE__/internal/modules/custom"
	"__MSS_DISTRIBUTION_BACKEND_MODULE__/business"
)

func Modules() []business.Module { return append(all.Modules(), custom.Modules()...) }
`)
	writeFixtureFile(t, root, "templates/application/internal/modules/all/generated.go.tmpl", `package all

import "__MSS_DISTRIBUTION_BACKEND_MODULE__/business"

func Modules() []business.Module { return nil }
`)
	writeFixtureFile(t, root, "templates/application/internal/modules/custom/modules.go.tmpl", `package custom

import "__MSS_DISTRIBUTION_BACKEND_MODULE__/business"

func Modules() []business.Module { return []business.Module{} }
`)
	writeFixtureFile(t, root, "templates/application/web/config/business-routes.ts", `import customBusinessRoutes from '../src/business/routes.config';
import generatedBusinessRoutes from './business-routes.generated';

export default [...generatedBusinessRoutes, ...customBusinessRoutes];
`)
	writeFixtureFile(t, root, "templates/application/web/src/route-registrations.ts", `import customRouteRegistrations from './business/route-registrations';
import generatedRouteRegistrations from './generated/routes';

const registrations = [...generatedRouteRegistrations, ...customRouteRegistrations];
const duplicateUI = 'duplicate business UI route path';
const duplicateServer = 'duplicate business server route path';
export default registrations;
`)
	writeFixtureFile(t, root, "templates/application/web/src/business/routes.config.ts", "export default [];\n")
	writeFixtureFile(t, root, "templates/application/web/src/business/route-registrations.ts", "export default [];\n")
	writeFixtureFile(t, root, "templates/application/web/src/business/locales/zh-CN.ts", "const messages = {};\nexport default messages;\n")
	writeFixtureFile(t, root, "templates/application/web/src/business/locales/en-US.ts", "const messages = {};\nexport default messages;\n")
	writeFixtureFile(t, root, "templates/application/web/src/locales/zh-CN.ts", `import coreMessages from '__MSS_DISTRIBUTION_FRONTEND_PACKAGE__/runtime/locales/zh-CN';
import customMessages from '../business/locales/zh-CN';
import generatedMessages from '../generated/locales/zh-CN';

export default { ...coreMessages, ...generatedMessages, ...customMessages };
`)
	writeFixtureFile(t, root, "templates/application/web/src/locales/en-US.ts", `import coreMessages from '__MSS_DISTRIBUTION_FRONTEND_PACKAGE__/runtime/locales/en-US';
import customMessages from '../business/locales/en-US';
import generatedMessages from '../generated/locales/en-US';

export default { ...coreMessages, ...generatedMessages, ...customMessages };
`)
	writeFixtureFile(t, root, "templates/application/.mss/project.yaml", `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: __MSS_APP_NAME__
  displayName: __MSS_APP_DISPLAY_NAME_YAML__
  repository: __MSS_APP_REPOSITORY__
spec:
  foundationVersion: 0.1.0
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
