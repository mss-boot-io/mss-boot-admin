package apis

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/models"
)

func TestSanitizeAuthorizePaths(t *testing.T) {
	paths := []string{" /user/list ", "", " /user/list", "/role/list", "   ", "/role/list"}
	want := []string{"/user/list", "/role/list"}
	got := sanitizeAuthorizePaths(paths)
	if len(got) != len(want) {
		t.Fatalf("unexpected sanitize length: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected sanitize value at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestMissingAuthorizePaths(t *testing.T) {
	paths := []string{"/menu/a", "/menu/b", "/menu/c"}
	loaded := map[string]struct{}{
		"/menu/a": {},
		"/menu/c": {},
	}
	missing := missingAuthorizePaths(paths, loaded)
	if len(missing) != 1 {
		t.Fatalf("unexpected missing length: got=%d want=1", len(missing))
	}
	if missing[0] != "/menu/b" {
		t.Fatalf("unexpected missing path: got=%q want=%q", missing[0], "/menu/b")
	}
}

func TestAuthorizePathSet(t *testing.T) {
	paths := []string{"/menu/a", "/menu/b", "/menu/a"}
	set := authorizePathSet(paths)
	if len(set) != 2 {
		t.Fatalf("unexpected set length: got=%d want=2", len(set))
	}
	if _, ok := set["/menu/a"]; !ok {
		t.Fatalf("missing expected path %q", "/menu/a")
	}
	if _, ok := set["/menu/b"]; !ok {
		t.Fatalf("missing expected path %q", "/menu/b")
	}
}

func TestFilterAuthorizeMenusByPathSet(t *testing.T) {
	menus := []*models.Menu{
		{Path: "/menu/a"},
		{Path: "/menu/b"},
		{Path: "/menu/c"},
	}
	pathSet := map[string]struct{}{
		"/menu/b": {},
		"/menu/c": {},
	}
	filtered := filterAuthorizeMenusByPathSet(menus, pathSet)
	if len(filtered) != 2 {
		t.Fatalf("unexpected filtered length: got=%d want=2", len(filtered))
	}
	if filtered[0].Path != "/menu/b" || filtered[1].Path != "/menu/c" {
		t.Fatalf("unexpected filtered order or values: got=%q,%q", filtered[0].Path, filtered[1].Path)
	}
}

func TestResolveAuthorizeRoleID(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		path     string
		expected string
	}{
		{name: "prefer request role id", request: " role-1 ", path: "role-2", expected: "role-1"},
		{name: "fallback to path role id", request: "  ", path: " role-2 ", expected: "role-2"},
		{name: "empty when both invalid", request: " ", path: " ", expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAuthorizeRoleID(tt.request, tt.path)
			if got != tt.expected {
				t.Fatalf("unexpected role id: got=%q want=%q", got, tt.expected)
			}
		})
	}
}
