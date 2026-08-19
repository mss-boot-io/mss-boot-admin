package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestValidateThinHostStructureRequiresGlueAndRejectsFoundationCore(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)

	result := validateThinHostStructure(ctx)
	if result.ExitCode != 0 {
		t.Fatalf("valid Thin Host structure = %#v", result)
	}
	for _, expected := range []string{"mss-boot-admin@v1.3.0", "required glue present", "Foundation core source absent"} {
		if !strings.Contains(result.Stdout, expected) {
			t.Fatalf("Thin Host validation output %q does not contain %q", result.Stdout, expected)
		}
	}

	writeThinHostTestFile(t, root, "mss-boot/go.mod", "module copied.example/mss-boot\n")
	result = validateThinHostStructure(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "Foundation core path mss-boot") {
		t.Fatalf("copied Foundation framework was accepted: %#v", result)
	}
}

func TestValidateThinHostStructureUsesCustomProjectLayout(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, "server", "ui/admin", "server/internal/business-modules")
	ctx.Project.Spec.RepositoryLayout["generated"] = "ui/admin/src/business-generated"
	ctx.Project.Spec.RepositoryLayout["businessRoutes"] = "ui/admin/config/business-routes.generated.ts"
	writeThinHostStructure(t, ctx)

	result := validateThinHostStructure(ctx)
	if result.ExitCode != 0 {
		t.Fatalf("custom-layout Thin Host structure = %#v", result)
	}
}

func TestValidateThinHostStructureRejectsDistributionDependencyDriftAndPrivateImports(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	writeThinHostTestFile(t, root, "web/package.json", `{
  "scripts": {
    "dev": "mss-admin-web dev",
    "lint": "mss-admin-web lint",
    "test": "mss-admin-web test",
    "build": "mss-admin-web build"
  },
  "dependencies": {"@mss-boot-io/admin-web": "1.3.1"}
}
`)
	writeThinHostTestFile(t, root, "web/src/generated/routes.ts", "import x from '@/shared/runtime';\nexport default x;\n")

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 {
		t.Fatalf("drifted Thin Host dependencies were accepted: %#v", result)
	}
	for _, expected := range []string{"admin-web@1.3.0", "private Admin Web path @/shared"} {
		if !strings.Contains(result.Error, expected) {
			t.Errorf("Thin Host drift error %q does not contain %q", result.Error, expected)
		}
	}
}

func TestPlanChecksUsesThinHostCommandsAndLayout(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	plan, err := PlanChecks(ctx, Options{Mode: ModeAll})
	if err != nil {
		t.Fatalf("PlanChecks(Thin Host all): %v", err)
	}
	wantIDs := []string{
		"backend-build",
		"backend-test",
		"frontend-build",
		"frontend-lint",
		"frontend-test",
		"git-diff-check",
	}
	if got := thinHostCommandIDs(plan.Checks); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("Thin Host all check IDs = %q, want %q", got, wantIDs)
	}
	for _, forbidden := range []string{"agent-tooling-test", "framework-test", "docs-build"} {
		if _, ok := thinHostCommandByID(plan.Checks, forbidden); ok {
			t.Fatalf("Thin Host all plan contains Foundation-only check %s", forbidden)
		}
	}
	frontend, ok := thinHostCommandByID(plan.Checks, "frontend-build")
	if !ok || frontend.Directory != filepath.Join(root, "web") || !reflect.DeepEqual(frontend.Args, []string{"corepack", "pnpm@10.34.5", "run", "build"}) {
		t.Fatalf("Thin Host frontend build = %#v", frontend)
	}

	modulePlan, err := PlanChecks(ctx, Options{Mode: ModeModule, Module: "supplier"})
	if err != nil {
		t.Fatalf("PlanChecks(Thin Host module): %v", err)
	}
	focused, ok := thinHostCommandByID(modulePlan.Checks, "module-test:supplier")
	if !ok {
		t.Fatalf("Thin Host module plan omitted focused test: %#v", modulePlan.Checks)
	}
	if focused.Directory != root || !reflect.DeepEqual(focused.Args, []string{"go", "test", "./internal/modules/supplier/..."}) || focused.Environment["GOWORK"] != "off" {
		t.Fatalf("Thin Host focused module test = %#v", focused)
	}
	if !containsString(modulePlan.ChangedFiles, "internal/modules/supplier") {
		t.Fatalf("Thin Host module paths = %q", modulePlan.ChangedFiles)
	}
}

func TestValidateContractsUsesConfiguredModulesDirectory(t *testing.T) {
	root := t.TempDir()
	writeThinHostTestFile(t, root, "internal/modules/supplier/module.yaml", "apiVersion: wrong\nkind: AdminModule\n")
	result := validateContracts(root, "internal/modules")
	if result.ExitCode == 0 || !strings.Contains(result.Error, "module") {
		t.Fatalf("invalid configured generated module was ignored: %#v", result)
	}
}

func thinHostVerifyContext(root, backend, frontend, modules string) *project.Context {
	return &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			APIVersion: "mss.io/v1alpha1",
			Kind:       "Project",
			Metadata:   project.Metadata{Name: "orders-admin", Repository: "acme/orders-admin"},
			Spec: project.ProjectSpec{
				Distribution: project.DistributionSpec{
					Name:     "mss-boot-admin",
					Version:  "v1.3.0",
					Backend:  project.DistributionBackendSpec{Module: "github.com/mss-boot-io/mss-boot-admin/admin", Version: "v1.3.0"},
					Frontend: project.DistributionFrontendSpec{Package: "@mss-boot-io/admin-web", Version: "1.3.0"},
				},
				RepositoryLayout: map[string]string{
					"kind":           "thin-host",
					"backend":        backend,
					"frontend":       frontend,
					"modules":        modules,
					"generated":      filepath.ToSlash(filepath.Join(frontend, "src", "generated")),
					"businessRoutes": filepath.ToSlash(filepath.Join(frontend, "config", "business-routes.generated.ts")),
					"specifications": ".mss",
					"documentation":  "docs",
				},
				Backend: project.BackendSpec{
					Module:          "github.com/acme/orders-admin",
					FrameworkModule: "github.com/mss-boot-io/mss-boot-admin/mss-boot",
				},
				Frontend: project.FrontendSpec{PackageManagerVersion: "10.34.5"},
			},
		},
	}
}

func writeThinHostStructure(t *testing.T, ctx *project.Context) {
	t.Helper()
	root := ctx.Root
	layout := ctx.Project.Spec.RepositoryLayout
	backend := layout["backend"]
	frontend := layout["frontend"]
	modules := layout["modules"]
	generated := layout["generated"]
	distribution := ctx.Project.Spec.Distribution
	files := map[string]string{
		".mss/project.yaml":            "project\n",
		".mss/lock.yaml":               "lock\n",
		".mss/blueprint-manifest.json": "{}\n",
		joinRepositoryPath(backend, "go.mod"): "module " + ctx.Project.Spec.Backend.Module + "\n\nrequire (\n\t" +
			distribution.Backend.Module + " " + distribution.Backend.Version + "\n\t" +
			ctx.Project.Spec.Backend.FrameworkModule + " " + distribution.Backend.Version + "\n)\n",
		joinRepositoryPath(backend, "cmd/server/main.go"): "package main\n\nimport (\n\t_ \"" + distribution.Backend.Module + "/app\"\n\t_ \"" + ctx.Project.Spec.Backend.Module + "/internal/modules/all\"\n)\n\nfunc main() {}\n",
		joinRepositoryPath(modules, "all/generated.go"):   "package all\n\nimport _ \"" + distribution.Backend.Module + "/business\"\n",
		joinRepositoryPath(frontend, "package.json"): `{
  "scripts": {
    "dev": "mss-admin-web dev",
    "lint": "mss-admin-web lint",
    "test": "mss-admin-web test",
    "build": "mss-admin-web build"
  },
  "dependencies": {"@mss-boot-io/admin-web": "1.3.0"}
}
`,
		joinRepositoryPath(frontend, "tsconfig.json"):        "{\n  \"extends\": \"./src/.umi/tsconfig.json\"\n}\n",
		joinRepositoryPath(frontend, "config/config.ts"):     "import { defineBusinessAdmin } from '" + distribution.Frontend.Package + "/business';\nimport businessRoutes from './business-routes.generated';\nexport default defineBusinessAdmin({ businessRoutes, routeRegistrations: './src/generated/routes.ts', useUtoopack: true });\n",
		joinRepositoryPath(frontend, "mss-admin.config.ts"):  "export { default } from './config/config';\n",
		layout["businessRoutes"]:                             "export default [];\n",
		joinRepositoryPath(frontend, "src/app.tsx"):          "export { getInitialState, layout, request, innerProvider } from '" + distribution.Frontend.Package + "/runtime/app';\n",
		joinRepositoryPath(frontend, "src/access.ts"):        "export { default } from '" + distribution.Frontend.Package + "/runtime/access';\n",
		joinRepositoryPath(frontend, "src/locales/zh-CN.ts"): "import core from '" + distribution.Frontend.Package + "/runtime/locales/zh-CN';\nimport generated from '../generated/locales/zh-CN';\nexport default { ...core, ...generated };\n",
		joinRepositoryPath(frontend, "src/locales/en-US.ts"): "import core from '" + distribution.Frontend.Package + "/runtime/locales/en-US';\nimport generated from '../generated/locales/en-US';\nexport default { ...core, ...generated };\n",
		joinRepositoryPath(generated, "routes.ts"):           "export default [];\n",
		joinRepositoryPath(generated, "locales/zh-CN.ts"):    "export default {};\n",
		joinRepositoryPath(generated, "locales/en-US.ts"):    "export default {};\n",
	}
	for relative, content := range files {
		writeThinHostTestFile(t, root, relative, content)
	}
}

func writeThinHostTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}

func thinHostCommandIDs(checks []command.Spec) []string {
	ids := make([]string, 0, len(checks))
	for _, check := range checks {
		ids = append(ids, check.ID)
	}
	sort.Strings(ids)
	return ids
}

func thinHostCommandByID(checks []command.Spec, id string) (command.Spec, bool) {
	for _, check := range checks {
		if check.ID == id {
			return check, true
		}
	}
	return command.Spec{}, false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
