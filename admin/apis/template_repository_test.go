package apis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitHubRepositoryURLCanonicalizesAllowedURLs(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/example/template",
		"https://github.com/example/template.git",
		" HTTPS://GITHUB.COM/example/template.git ",
	} {
		repository, err := parseGitHubRepositoryURL(raw)
		if err != nil {
			t.Fatalf("parseGitHubRepositoryURL(%q) error = %v", raw, err)
		}
		if repository.Owner != "example" || repository.Name != "template" ||
			repository.WebURL != "https://github.com/example/template" ||
			repository.CloneURL != "https://github.com/example/template.git" {
			t.Fatalf("parseGitHubRepositoryURL(%q) = %#v", raw, repository)
		}
	}
}

func TestParseGitHubRepositoryURLRejectsCredentialExfiltrationTargets(t *testing.T) {
	invalid := []string{
		"http://github.com/example/template",
		"https://evil.example/example/template",
		"https://github.com.evil.example/example/template",
		"https://user@github.com/example/template",
		"https://github.com:443/example/template",
		"https://github.com/example/template?redirect=https://evil.example",
		"https://github.com/example/template#fragment",
		"https://github.com/example/template/extra",
		"https://github.com/example%2ftemplate",
		"https://github.com/example/template%2fextra",
		"git@github.com:example/template.git",
		"//github.com/example/template",
		"https://github.com/-invalid/template",
		"https://github.com/example/.git",
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			if repository, err := parseGitHubRepositoryURL(raw); err == nil {
				t.Fatalf("parseGitHubRepositoryURL(%q) = %#v, want rejection", raw, repository)
			}
		})
	}
}

func TestSafeTemplateRelativePathRejectsWorkspaceEscapes(t *testing.T) {
	invalid := []string{
		"..",
		"../outside",
		"nested/../../outside",
		"/absolute",
		`..\outside`,
		"C:outside",
		".git/config",
		"nested/.GIT/config",
		"name\x00with-nul",
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			if value, err := safeTemplateRelativePath(raw, "."); err == nil {
				t.Fatalf("safeTemplateRelativePath(%q) = %q, want rejection", raw, value)
			}
		})
	}

	value, err := safeTemplateRelativePath("nested/../service", ".")
	if err != nil || value != "service" {
		t.Fatalf("safe relative path = %q, %v; want service", value, err)
	}
}

func TestRemoveTemplateWorkspaceIsConfined(t *testing.T) {
	outside := t.TempDir()
	marker := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write outside marker: %v", err)
	}
	if err := removeTemplateWorkspace(outside); err == nil {
		t.Fatal("removeTemplateWorkspace accepted an outside directory")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("outside marker was removed: %v", err)
	}

	workspace, err := newTemplateWorkspace()
	if err != nil {
		t.Fatalf("newTemplateWorkspace() error = %v", err)
	}
	insideMarker := filepath.Join(workspace, "generated.txt")
	if err = os.WriteFile(insideMarker, []byte("generated"), 0o600); err != nil {
		t.Fatalf("write workspace marker: %v", err)
	}
	if err = removeTemplateWorkspace(workspace); err != nil {
		t.Fatalf("removeTemplateWorkspace(valid) error = %v", err)
	}
	if _, err = os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists after cleanup: %v", err)
	}
}
