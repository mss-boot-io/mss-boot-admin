package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRepositorySkills(t *testing.T) {
	root := findRepositoryRoot(t)
	report, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if !report.Valid {
		t.Fatalf("Discover() valid = false, issues = %#v", report.Issues)
	}
	if len(report.Skills) < 10 {
		t.Fatalf("skill count = %d, want at least the initial skill set", len(report.Skills))
	}
	for index, skill := range report.Skills {
		if skill.Name == "" || skill.Description == "" || skill.Body == "" {
			t.Fatalf("incomplete skill at index %d: %#v", index, skill)
		}
		if index > 0 && report.Skills[index-1].Name >= skill.Name {
			t.Fatalf("skills are not sorted: %q then %q", report.Skills[index-1].Name, skill.Name)
		}
	}
	data, err := report.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if !strings.Contains(string(data), `"mss-add-module"`) {
		t.Fatalf("JSON output does not contain mss-add-module: %s", data)
	}
}

func TestDiscoverReportsMissingSkillFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills", "empty-skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	report, err := Discover(root)
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Discover() error = %v, want ValidationError", err)
	}
	if report.Valid || !hasIssue(report.Issues, "skill-file-missing") {
		t.Fatalf("report = %#v", report)
	}
}

func TestDiscoverValidatesFrontMatterAndDirectory(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "expected-name", `---
name: different-name
description: test skill
---

# Test

Instructions.
`)
	report, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() unexpectedly succeeded")
	}
	if !hasIssue(report.Issues, "skill-directory-mismatch") {
		t.Fatalf("issues = %#v", report.Issues)
	}
}

func TestDiscoverRejectsPersonalAbsolutePath(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "unsafe-skill", `---
name: unsafe-skill
description: unsafe example
---

Run from /home/lwx/project.
`)
	report, err := Discover(root)
	if err == nil || !hasIssue(report.Issues, "personal-absolute-path") {
		t.Fatalf("Discover() error = %v, issues = %#v", err, report.Issues)
	}
}

func TestSplitFrontMatterRequiresDelimiters(t *testing.T) {
	for _, content := range []string{
		"# no front matter\n",
		"---\nname: missing-close\n",
	} {
		if _, _, err := splitFrontMatter(content); err == nil {
			t.Fatalf("splitFrontMatter(%q) unexpectedly succeeded", content)
		}
	}
}

func writeSkill(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, ".agents", "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func hasIssue(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".mss", "project.yaml")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("repository root not found")
		}
		current = parent
	}
}
