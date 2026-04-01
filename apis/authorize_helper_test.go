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

func TestHasEmptyAuthorizeRoleID(t *testing.T) {
	tests := []struct {
		name   string
		roleID string
		want   bool
	}{
		{name: "empty", roleID: "", want: true},
		{name: "spaces", roleID: "   ", want: true},
		{name: "value", roleID: "role-1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasEmptyAuthorizeRoleID(tt.roleID)
			if got != tt.want {
				t.Fatalf("unexpected empty-role detection: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestBuildMenuAuthorizeRules(t *testing.T) {
	rules := buildMenuAuthorizeRules("role-1", []string{"/menu/a", "/menu/b"})
	if len(rules) != 2 {
		t.Fatalf("unexpected menu rule length: got=%d want=2", len(rules))
	}
	if rules[0].V0 != "role-1" || rules[0].V1 != "MENU" || rules[0].V2 != "/menu/a" {
		t.Fatalf("unexpected first menu rule: %#v", rules[0])
	}
	if rules[1].V2 != "/menu/b" {
		t.Fatalf("unexpected second menu path: got=%q want=%q", rules[1].V2, "/menu/b")
	}
}

func TestBuildRoleAuthorizeRulesDeduplicate(t *testing.T) {
	menus := []*models.Menu{
		{
			Path:   "/menu/a",
			Type:   "MENU",
			Method: "GET",
			Children: []*models.Menu{
				{Path: "/api/a", Type: "API", Method: "GET"},
				{Path: "/api/a", Type: "API", Method: "GET"},
			},
		},
		{
			Path:   "/menu/a",
			Type:   "MENU",
			Method: "GET",
		},
	}
	rules := buildRoleAuthorizeRules("role-1", menus)
	if len(rules) != 2 {
		t.Fatalf("unexpected role rule length: got=%d want=2", len(rules))
	}
	if rules[0].V0 != "role-1" {
		t.Fatalf("unexpected role id on first rule: got=%q", rules[0].V0)
	}
	if rules[0].V1 != "MENU" || rules[0].V2 != "/menu/a" || rules[0].V3 != "GET" {
		t.Fatalf("unexpected first role rule: %#v", rules[0])
	}
	if rules[1].V1 != "API" || rules[1].V2 != "/api/a" || rules[1].V3 != "GET" {
		t.Fatalf("unexpected second role rule: %#v", rules[1])
	}
}

func TestBuildRoleAuthorizeRulesPersistsMethod(t *testing.T) {
	menus := []*models.Menu{
		{
			Path:   "/menu/trace",
			Type:   "MENU",
			Method: "POST",
			Children: []*models.Menu{
				{Path: "/api/trace", Type: "API", Method: "PUT"},
			},
		},
	}
	rules := buildRoleAuthorizeRules("role-2", menus)
	if len(rules) != 2 {
		t.Fatalf("unexpected role rule length: got=%d want=2", len(rules))
	}
	if rules[0].V3 != "POST" {
		t.Fatalf("menu method not persisted: got=%q want=%q", rules[0].V3, "POST")
	}
	if rules[1].V3 != "PUT" {
		t.Fatalf("api method not persisted: got=%q want=%q", rules[1].V3, "PUT")
	}
}
