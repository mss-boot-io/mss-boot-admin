package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestWriteReportsUsesPrivateFilePermissions(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{Root: root}
	report := Report{Project: "private-report", Root: root, Success: true}
	if err := WriteReports(ctx, report); err != nil {
		t.Fatalf("WriteReports() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for _, name := range []string{"verify.json", "verify.md"} {
		info, err := os.Stat(filepath.Join(root, ".mss", "reports", name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, got)
		}
	}
}

func TestToolingTestUsesConsolidatedAdminRuntimePathWhenOptionalRuntimeIsAbsent(t *testing.T) {
	root := t.TempDir()
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
	}

	spec := toolingTest(root)
	if got := spec.Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tooling test arguments = %q, want %q", got, want)
	}
	if got, want := spec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("tooling test GOWORK = %q, want %q", got, want)
	}
	if got := spec.Environment["GOFLAGS"]; got != "-mod=readonly" {
		t.Fatalf("tooling test GOFLAGS = %q, want -mod=readonly", got)
	}
}

func TestToolingTestIncludesExistingOptionalModuleRuntime(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "modules", "runtime"), 0o755); err != nil {
		t.Fatalf("create optional module runtime: %v", err)
	}
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
		"./modules/runtime/...",
	}

	if got := toolingTest(root).Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tooling test arguments = %q, want %q", got, want)
	}
}

func TestBackendChecksUseTheFoundationWorkspaceBeforePublicDependencyQualification(t *testing.T) {
	root := t.TempDir()
	adminDir := filepath.Join(root, "admin")

	testSpec := backendTest(root)
	if testSpec.Directory != adminDir {
		t.Fatalf("backend test directory = %q, want %q", testSpec.Directory, adminDir)
	}
	if want := []string{
		"go", "test",
		"-coverprofile=" + filepath.Join(root, ".mss", "reports", "admin-coverage.out"),
		"./...",
	}; !reflect.DeepEqual(testSpec.Args, want) {
		t.Fatalf("backend test arguments = %q, want %q", testSpec.Args, want)
	}
	if got, want := testSpec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("backend test GOWORK = %q, want %q", got, want)
	}
	if testSpec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("backend test environment = %#v", testSpec.Environment)
	}

	buildSpec := backendBuild(root)
	if buildSpec.Directory != adminDir {
		t.Fatalf("backend build directory = %q, want %q", buildSpec.Directory, adminDir)
	}
	if want := []string{"go", "build", "./..."}; !reflect.DeepEqual(buildSpec.Args, want) {
		t.Fatalf("backend build arguments = %q, want %q", buildSpec.Args, want)
	}
	if buildSpec.Environment["GOWORK"] != filepath.Join(root, "go.work") ||
		buildSpec.Environment["GOFLAGS"] != "-mod=readonly" || buildSpec.Environment["CGO_ENABLED"] != "0" {
		t.Fatalf("backend build environment = %#v", buildSpec.Environment)
	}
}

func TestFocusedFoundationModulePinsFoundationWorkspace(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{
		Root: root,
		Project: project.ProjectDocument{Spec: project.ProjectSpec{RepositoryLayout: map[string]string{
			"kind":    "foundation",
			"backend": "admin",
			"modules": "admin/modules",
		}}},
	}

	spec, _, err := focusedModuleTest(ctx, "supplier")
	if err != nil {
		t.Fatalf("focusedModuleTest() error = %v", err)
	}
	if got, want := spec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("focused Foundation GOWORK = %q, want %q", got, want)
	}
	if spec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("focused Foundation environment = %#v", spec.Environment)
	}
}

func TestFrameworkCheckIsIndependentAndReadOnly(t *testing.T) {
	spec := frameworkTest(t.TempDir())
	if spec.Environment["GOWORK"] != "off" || spec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("framework test environment = %#v", spec.Environment)
	}
}

func TestPresentationThinHostContractUsesRepositoryExternalConsumers(t *testing.T) {
	root := t.TempDir()
	spec := presentationThinHostContract(root)
	if got, want := spec.Directory, root; got != want {
		t.Fatalf("presentation Thin Host directory = %q, want %q", got, want)
	}
	if got, want := spec.Args, []string{"bash", "tools/compatibility/test-presentation-thin-host-contract.sh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation Thin Host arguments = %q, want %q", got, want)
	}
	if spec.Environment["CI"] != "true" {
		t.Fatalf("presentation Thin Host environment = %#v", spec.Environment)
	}
}

func TestPresentationThinHostContractSensitivity(t *testing.T) {
	for _, path := range []string{
		".mss/core-pages/user-list.yaml",
		".mss/modules/example-supplier.yaml",
		"admin/presentation/core/definitions_generated.go",
		"admin/modules/supplier/presentation_manifest.generated.json",
		"cmd/mss/main.go",
		"internal/mss/generator/core_presentation.go",
		"internal/mss/spec/admin_presentation_inventory.go",
		"templates/application/.mss/project.yaml",
		"templates/module/frontend/page.tsx.tmpl",
		"tools/compatibility/test-presentation-thin-host-contract.sh",
		"web/antd-v6/package.json",
		"web/antd-v6/src/generated/core-presentation-registry.generated.ts",
		"web/antd-v6/src/modules/operations/tablePresentation.ts",
		"web/antd-v6/src/shared/presentation/runtime.ts",
	} {
		if !presentationThinHostContractSensitive(path) {
			t.Errorf("presentation Thin Host contract did not select %q", path)
		}
	}
	for _, path := range []string{
		"docs/docs/release.md",
		"mss-boot/cache/cache.go",
		"web/antd-v6/README.md",
	} {
		if presentationThinHostContractSensitive(path) {
			t.Errorf("presentation Thin Host contract unexpectedly selected %q", path)
		}
	}
}

func TestFrontendBuildUsesPortableReleaseProfile(t *testing.T) {
	root := t.TempDir()
	spec := frontendBuild(root)
	if want := []string{"corepack", "pnpm@10.34.5", "build:release"}; !reflect.DeepEqual(spec.Args, want) {
		t.Fatalf("frontend build arguments = %q, want %q", spec.Args, want)
	}
	if spec.Directory != filepath.Join(root, "web", "antd-v6") {
		t.Fatalf("frontend build directory = %q", spec.Directory)
	}
}

func TestFrontendChecksUseOnlyV6ApplicationDirectory(t *testing.T) {
	root := t.TempDir()
	wantDirectory := filepath.Join(root, "web", "antd-v6")
	for _, spec := range []struct {
		name     string
		got      commandSpecView
		wantArgs []string
	}{
		{name: "lint", got: commandSpecView{frontendLint(root).Directory, frontendLint(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "lint"}},
		{name: "test", got: commandSpecView{frontendTest(root).Directory, frontendTest(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "test:ci"}},
		{name: "build", got: commandSpecView{frontendBuild(root).Directory, frontendBuild(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "build:release"}},
	} {
		if spec.got.directory != wantDirectory {
			t.Fatalf("%s directory = %q, want %q", spec.name, spec.got.directory, wantDirectory)
		}
		if !reflect.DeepEqual(spec.got.args, spec.wantArgs) {
			t.Fatalf("%s arguments = %q, want %q", spec.name, spec.got.args, spec.wantArgs)
		}
	}
}

func TestFrontendV6FullChecksRequireConfiguredApplication(t *testing.T) {
	ctx := &project.Context{}
	if hasFrontendApplication(ctx, "web/antd-v6") {
		t.Fatal("empty project context unexpectedly enables the V6 frontend")
	}
	ctx.Project.Spec.Frontend.Applications = []project.FrontendApplicationSpec{
		{ID: "antd-v6", Path: "web/antd-v6"},
	}
	if !hasFrontendApplication(ctx, "web/antd-v6") {
		t.Fatal("configured v6 frontend was not detected")
	}
}

type commandSpecView struct {
	directory string
	args      []string
}

func TestValidateContractsRejectsInvalidFeature(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, ".mss", "features")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("create feature directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(featureDir, "invalid.yaml"),
		[]byte("apiVersion: mss.io/v1alpha1\nkind: Feature\nmetadata: {}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write invalid feature: %v", err)
	}

	result := validateContracts(root)
	if result.ExitCode == 0 {
		t.Fatalf("invalid FeatureSpec was ignored: %#v", result)
	}
}
