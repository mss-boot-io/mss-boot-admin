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
