package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestToolingTestUsesConsolidatedAdminRuntimePathWhenOptionalRuntimeIsAbsent(t *testing.T) {
	root := t.TempDir()
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
	}

	if got := toolingTest(root).Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tooling test arguments = %q, want %q", got, want)
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

func TestBackendChecksTargetIndependentAdminModule(t *testing.T) {
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
	if testSpec.Environment["GOWORK"] != "off" {
		t.Fatalf("backend test GOWORK = %q, want off", testSpec.Environment["GOWORK"])
	}

	buildSpec := backendBuild(root)
	if buildSpec.Directory != adminDir {
		t.Fatalf("backend build directory = %q, want %q", buildSpec.Directory, adminDir)
	}
	if want := []string{"go", "build", "./..."}; !reflect.DeepEqual(buildSpec.Args, want) {
		t.Fatalf("backend build arguments = %q, want %q", buildSpec.Args, want)
	}
	if buildSpec.Environment["GOWORK"] != "off" || buildSpec.Environment["CGO_ENABLED"] != "0" {
		t.Fatalf("backend build environment = %#v", buildSpec.Environment)
	}
}

func TestFrontendBuildUsesPortableReleaseProfile(t *testing.T) {
	root := t.TempDir()
	spec := frontendBuild(root)
	if want := []string{"corepack", "pnpm", "build:release"}; !reflect.DeepEqual(spec.Args, want) {
		t.Fatalf("frontend build arguments = %q, want %q", spec.Args, want)
	}
	if spec.Directory != filepath.Join(root, "web", "antd") {
		t.Fatalf("frontend build directory = %q", spec.Directory)
	}
}

func TestFrontendV6ChecksUseIndependentApplicationDirectory(t *testing.T) {
	root := t.TempDir()
	wantDirectory := filepath.Join(root, "web", "antd-v6")
	for _, spec := range []struct {
		name     string
		got      commandSpecView
		wantArgs []string
	}{
		{name: "lint", got: commandSpecView{frontendV6Lint(root).Directory, frontendV6Lint(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "lint"}},
		{name: "test", got: commandSpecView{frontendV6Test(root).Directory, frontendV6Test(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "test:ci"}},
		{name: "build", got: commandSpecView{frontendV6Build(root).Directory, frontendV6Build(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "build:release"}},
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
		t.Fatal("legacy project context unexpectedly enables the v6 frontend")
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
