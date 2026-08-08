package apis

import "testing"

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
