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
