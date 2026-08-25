package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
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

func TestValidateThinHostStructureRequiresExplicitBusinessExtensionFacades(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	writeThinHostTestFile(t, root, "internal/modules/registry.go", "package modules\n")
	writeThinHostTestFile(t, root, "web/src/route-registrations.ts", "export default [];\n")
	writeThinHostTestFile(t, root, "web/src/locales/zh-CN.ts", "export default {};\n")
	writeThinHostTestFile(t, root, "web/src/locales/en-US.ts", "export default {};\n")
	if err := os.Remove(filepath.Join(root, "web", "src", "business", "locales", "en-US.ts")); err != nil {
		t.Fatalf("remove handwritten English locale seam: %v", err)
	}

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 {
		t.Fatalf("Thin Host without explicit business composition was accepted: %#v", result)
	}
	for _, expected := range []string{
		"append(all.Modules(), custom.Modules()...)",
		"duplicate business UI route path",
		"duplicate business server route path",
		"../business/locales/zh-CN",
		"../business/locales/en-US",
		"web/src/business/locales/en-US.ts",
	} {
		if !strings.Contains(result.Error, expected) {
			t.Errorf("Thin Host composition error %q does not contain %q", result.Error, expected)
		}
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

func TestValidateThinHostStructureRejectsPrivateRegistryCredential(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	writeThinHostTestFile(t, root, "web/.npmrc", "@mss-boot-io:registry=https://npm.pkg.github.com\n//npm.pkg.github.com/:_authToken=committed-token\n")

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "public npm registry") {
		t.Fatalf("committed package token was accepted: %#v", result)
	}
}

func TestValidateThinHostStructureRequiresFrozenDistributionChecksums(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	writeThinHostTestFile(t, root, "go.sum", "example.invalid/module v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	writeThinHostTestFile(t, root, "web/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 {
		t.Fatalf("incomplete frozen dependency inputs were accepted: %#v", result)
	}
	for _, expected := range []string{
		"go.sum must contain an exact non-placeholder checksum for github.com/mss-boot-io/mss-boot-admin/admin v1.3.0",
		"go.sum must contain an exact non-placeholder checksum for github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.0/go.mod",
		"pnpm-lock.yaml must pin frontend specifier @mss-boot-io/admin-web@1.3.0",
		"pnpm-lock.yaml must contain the frozen package snapshot for @mss-boot-io/admin-web@1.3.0",
	} {
		if !strings.Contains(result.Error, expected) {
			t.Errorf("frozen dependency error %q does not contain %q", result.Error, expected)
		}
	}
}

func TestValidateThinHostStructureRejectsMissingFrontendTarballIntegrity(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	writeThinHostTestFile(t, root, "web/pnpm-lock.yaml", "lockfileVersion: '9.0'\nimporters:\n  .:\n    dependencies:\n      '@mss-boot-io/admin-web':\n        specifier: 1.3.0\n        version: 1.3.0\npackages:\n  '@mss-boot-io/admin-web@1.3.0':\n    resolution: {}\n")

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "sha512 tarball integrity") {
		t.Fatalf("missing frontend tarball integrity was accepted: %#v", result)
	}
}

func TestValidateThinHostStructureRejectsUnreadableRequiredFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide portable owner-read permission semantics")
	}
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	writeThinHostStructure(t, ctx)
	npmrcPath := filepath.Join(root, "web", ".npmrc")
	if err := os.Chmod(npmrcPath, 0); err != nil {
		t.Fatalf("make npmrc unreadable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(npmrcPath, 0o600)
	})

	result := validateThinHostStructure(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "read required Thin Host file web/.npmrc") {
		t.Fatalf("unreadable required npmrc was accepted: %#v", result)
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
		"git-worktree-check",
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
	if focused.Directory != root || !reflect.DeepEqual(focused.Args, []string{"go", "test", "./internal/modules/supplier/..."}) ||
		focused.Environment["GOWORK"] != "off" || focused.Environment["GOFLAGS"] != "-mod=readonly" {
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

func TestThinHostGeneratedModuleDriftFailsVerification(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	data, err := spec.RenderModuleTemplate("inventory", "Inventory")
	if err != nil {
		t.Fatalf("render module template: %v", err)
	}
	specPath := filepath.Join(root, ".mss", "modules", "inventory.yaml")
	writeThinHostTestFile(t, root, ".mss/modules/inventory.yaml", string(data))
	module, err := spec.LoadModule(specPath)
	if err != nil {
		t.Fatalf("load module: %v", err)
	}
	module.SourcePath = ".mss/modules/inventory.yaml"
	if _, err := generator.Generate(module, generator.Options{
		Root: root, Write: true, Check: false, Project: &ctx.Project,
	}); err != nil {
		t.Fatalf("generate current module: %v", err)
	}
	if result := validateThinHostGeneratedModules(ctx); result.ExitCode != 0 {
		t.Fatalf("current module reported drift: %#v", result)
	}

	module.Metadata.DisplayName = "Inventory changed"
	changed, err := module.YAML()
	if err != nil {
		t.Fatalf("marshal changed module: %v", err)
	}
	if err := os.WriteFile(specPath, changed, 0o644); err != nil {
		t.Fatalf("write changed module: %v", err)
	}
	result := validateThinHostGeneratedModules(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "generated module output is stale") {
		t.Fatalf("stale module projections were accepted: %#v", result)
	}
}

func TestThinHostGeneratedModuleOrphanFailsAfterLastSpecIsDeleted(t *testing.T) {
	root := t.TempDir()
	ctx := thinHostVerifyContext(root, ".", "web", "internal/modules")
	data, err := spec.RenderModuleTemplate("inventory", "Inventory")
	if err != nil {
		t.Fatalf("render module template: %v", err)
	}
	specPath := filepath.Join(root, ".mss", "modules", "inventory.yaml")
	writeThinHostTestFile(t, root, ".mss/modules/inventory.yaml", string(data))
	module, err := spec.LoadModule(specPath)
	if err != nil {
		t.Fatalf("load module: %v", err)
	}
	module.SourcePath = ".mss/modules/inventory.yaml"
	if _, err := generator.Generate(module, generator.Options{
		Root: root, Write: true, Project: &ctx.Project,
	}); err != nil {
		t.Fatalf("generate module: %v", err)
	}
	if err := os.Remove(specPath); err != nil {
		t.Fatalf("remove final module specification: %v", err)
	}
	result := validateThinHostGeneratedModules(ctx)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "references missing AdminModule specifications") || !strings.Contains(result.Error, ".mss/modules/inventory.yaml") {
		t.Fatalf("orphaned generated module was accepted: %#v", result)
	}
}

func TestUnbornRepositoryDiffAndUntrackedTextChecks(t *testing.T) {
	root := t.TempDir()
	git := exec.Command("git", "init", "-b", "main")
	git.Dir = root
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	writeThinHostTestFile(t, root, "README.md", "# Fresh Thin Host\n")
	check := diffCheck(root)
	if !reflect.DeepEqual(check.Args, []string{"git", "diff", "--cached", "--check", "--"}) {
		t.Fatalf("unborn diff check args = %#v", check.Args)
	}
	if result := command.Run(context.Background(), check); result.ExitCode != 0 {
		t.Fatalf("unborn diff check failed: %#v", result)
	}
	if result := validateUntrackedWorkspaceText(root); result.ExitCode != 0 {
		t.Fatalf("clean untracked file failed: %#v", result)
	}
	stage := exec.Command("git", "add", "--", "README.md")
	stage.Dir = root
	if output, err := stage.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	checks := diffChecks(root)
	if len(checks) != 2 || checks[1].ID != "git-worktree-check" {
		t.Fatalf("unborn diff checks = %#v", checks)
	}
	for _, stagedCheck := range checks {
		if result := command.Run(context.Background(), stagedCheck); result.ExitCode != 0 {
			t.Fatalf("clean unborn staged check failed: %#v", result)
		}
	}

	writeThinHostTestFile(t, root, "README.md", "# Fresh Thin Host\n<<<<<<< local\nconflict  \n=======\n")
	if result := validateUntrackedWorkspaceText(root); result.ExitCode != 0 {
		t.Fatalf("tracked worktree file was incorrectly treated as untracked: %#v", result)
	}
	result := command.Run(context.Background(), checks[1])
	if result.ExitCode == 0 || !strings.Contains(result.Stdout+result.Stderr+result.Error, "trailing whitespace") {
		t.Fatalf("unsafe staged-then-modified file was accepted: %#v", result)
	}

	writeThinHostTestFile(t, root, "UNTRACKED.md", "<<<<<<< local\nconflict  \n=======\n")
	result = validateUntrackedWorkspaceText(root)
	if result.ExitCode == 0 || !strings.Contains(result.Error, "unresolved conflict marker") || !strings.Contains(result.Error, "trailing whitespace") {
		t.Fatalf("unsafe untracked file was accepted: %#v", result)
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
	modulesRelative, err := filepath.Rel(filepath.FromSlash(backend), filepath.FromSlash(modules))
	if err != nil {
		t.Fatalf("resolve modules import path: %v", err)
	}
	modulesImport := strings.TrimSuffix(ctx.Project.Spec.Backend.Module, "/") + "/" + filepath.ToSlash(modulesRelative)
	files := map[string]string{
		".mss/project.yaml":            "project\n",
		".mss/lock.yaml":               "lock\n",
		".mss/blueprint-manifest.json": "{}\n",
		".mss/dev.yaml":                "apiVersion: mss.io/v1alpha1\nkind: DevelopmentEnvironment\n",
		"README.md":                    "# Thin Host\n",
		joinRepositoryPath(backend, "go.mod"): "module " + ctx.Project.Spec.Backend.Module + "\n\nrequire (\n\t" +
			distribution.Backend.Module + " " + distribution.Backend.Version + "\n\t" +
			ctx.Project.Spec.Backend.FrameworkModule + " " + distribution.Backend.Version + "\n)\n",
		joinRepositoryPath(backend, "go.sum"): distribution.Backend.Module + " " + distribution.Backend.Version + " h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			distribution.Backend.Module + " " + distribution.Backend.Version + "/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			ctx.Project.Spec.Backend.FrameworkModule + " " + distribution.Backend.Version + " h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			ctx.Project.Spec.Backend.FrameworkModule + " " + distribution.Backend.Version + "/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n",
		joinRepositoryPath(backend, "cmd/server/main.go"): "package main\n\nimport (\n\t_ \"" + distribution.Backend.Module + "/app\"\n\t\"" + modulesImport + "\"\n)\n\nfunc main() { _ = modules.Modules() }\n",
		joinRepositoryPath(modules, "registry.go"):        "package modules\n\nimport (\n\t\"" + modulesImport + "/all\"\n\t\"" + modulesImport + "/custom\"\n\t\"" + distribution.Backend.Module + "/business\"\n)\n\nfunc Modules() []business.Module { return append(all.Modules(), custom.Modules()...) }\n",
		joinRepositoryPath(modules, "all/generated.go"):   "package all\n\nimport _ \"" + distribution.Backend.Module + "/business\"\n",
		joinRepositoryPath(modules, "custom/modules.go"):  "package custom\n\nimport \"" + distribution.Backend.Module + "/business\"\n\nfunc Modules() []business.Module { return []business.Module{} }\n",
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
		joinRepositoryPath(frontend, ".npmrc"):                              "registry=https://registry.npmjs.org/\nsave-exact=true\n",
		joinRepositoryPath(frontend, "pnpm-lock.yaml"):                      "lockfileVersion: '9.0'\nimporters:\n  .:\n    dependencies:\n      '@mss-boot-io/admin-web':\n        specifier: 1.3.0\n        version: 1.3.0\npackages:\n  '@mss-boot-io/admin-web@1.3.0':\n    resolution: {integrity: sha512-" + strings.Repeat("A", 86) + "==}\n",
		joinRepositoryPath(frontend, "tsconfig.json"):                       "{\n  \"extends\": \"./src/.umi/tsconfig.json\"\n}\n",
		joinRepositoryPath(frontend, "config/config.ts"):                    "import { defineBusinessAdmin } from '" + distribution.Frontend.Package + "/business';\nimport businessRoutes from './business-routes';\nexport default defineBusinessAdmin({ businessRoutes, routeRegistrations: './src/route-registrations.ts', useUtoopack: true });\n",
		joinRepositoryPath(frontend, "config/business-routes.ts"):           "import type { AdminBusinessRoute } from '" + distribution.Frontend.Package + "/business';\nimport customBusinessRoutes from '../src/business/routes.config';\nimport generatedBusinessRoutes from './business-routes.generated';\nconst businessRoutes: AdminBusinessRoute[] = [...generatedBusinessRoutes, ...customBusinessRoutes];\nexport default businessRoutes;\n",
		joinRepositoryPath(frontend, "mss-admin.config.ts"):                 "export { default } from './config/config';\n",
		layout["businessRoutes"]:                                            "export default [];\n",
		joinRepositoryPath(frontend, "src/app.tsx"):                         "export { getInitialState, layout, request, innerProvider } from '" + distribution.Frontend.Package + "/runtime/app';\n",
		joinRepositoryPath(frontend, "src/access.ts"):                       "export { default } from '" + distribution.Frontend.Package + "/runtime/access';\n",
		joinRepositoryPath(frontend, "src/route-registrations.ts"):          "import type { RouteRegistration } from '" + distribution.Frontend.Package + "/runtime';\nimport generatedRouteRegistrations from './generated/routes';\nimport customRouteRegistrations from './business/route-registrations';\nconst ui = 'duplicate business UI route path';\nconst server = 'duplicate business server route path';\nexport default [...generatedRouteRegistrations, ...customRouteRegistrations] as readonly RouteRegistration[];\n",
		joinRepositoryPath(frontend, "src/business/routes.config.ts"):       "export default [];\n",
		joinRepositoryPath(frontend, "src/business/route-registrations.ts"): "export default [];\n",
		joinRepositoryPath(frontend, "src/business/locales/zh-CN.ts"):       "const messages = {};\nexport default messages;\n",
		joinRepositoryPath(frontend, "src/business/locales/en-US.ts"):       "const messages = {};\nexport default messages;\n",
		joinRepositoryPath(frontend, "src/locales/zh-CN.ts"):                "import coreMessages from '" + distribution.Frontend.Package + "/runtime/locales/zh-CN';\nimport generatedMessages from '../generated/locales/zh-CN';\nimport customMessages from '../business/locales/zh-CN';\nexport default { ...coreMessages, ...generatedMessages, ...customMessages };\n",
		joinRepositoryPath(frontend, "src/locales/en-US.ts"):                "import coreMessages from '" + distribution.Frontend.Package + "/runtime/locales/en-US';\nimport generatedMessages from '../generated/locales/en-US';\nimport customMessages from '../business/locales/en-US';\nexport default { ...coreMessages, ...generatedMessages, ...customMessages };\n",
		joinRepositoryPath(generated, "routes.ts"):                          "export default [];\n",
		joinRepositoryPath(generated, "locales/zh-CN.ts"):                   "export default {};\n",
		joinRepositoryPath(generated, "locales/en-US.ts"):                   "export default {};\n",
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
