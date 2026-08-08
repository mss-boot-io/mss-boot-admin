package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
