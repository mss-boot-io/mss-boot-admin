package verify

import (
	"reflect"
	"testing"
)

func TestToolingTestUsesConsolidatedAdminRuntimePath(t *testing.T) {
	spec := toolingTest("/repo")
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
	}
	if !reflect.DeepEqual(spec.Args, want) {
		t.Fatalf("tooling test args = %#v, want %#v", spec.Args, want)
	}
}
