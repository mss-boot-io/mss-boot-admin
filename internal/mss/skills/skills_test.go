package skills

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var thinHostSkillNames = []string{
	"mss-add-field",
	"mss-add-module",
	"mss-add-permission",
	"mss-debug-fullstack",
	"mss-review-change",
	"mss-thin-host",
	"mss-upgrade-foundation",
}

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

func TestDiscoverThinHostSkillsMatchesBlueprintContract(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	templateRoot := filepath.Join(repositoryRoot, "templates", "application")
	report, err := Discover(templateRoot)
	if err != nil {
		t.Fatalf("Discover(template) error = %v", err)
	}
	if !report.Valid {
		t.Fatalf("Discover(template) valid = false, issues = %#v", report.Issues)
	}

	discoveredNames := make([]string, 0, len(report.Skills))
	for _, skill := range report.Skills {
		discoveredNames = append(discoveredNames, skill.Name)
	}
	if !slices.Equal(discoveredNames, thinHostSkillNames) {
		t.Fatalf("Thin Host skills = %#v, want %#v", discoveredNames, thinHostSkillNames)
	}

	blueprintData, err := os.ReadFile(filepath.Join(repositoryRoot, ".mss", "blueprints", "management-system.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(management-system blueprint) error = %v", err)
	}
	var blueprintContract struct {
		Spec struct {
			RequiredFiles []string `yaml:"requiredFiles"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(blueprintData, &blueprintContract); err != nil {
		t.Fatalf("Unmarshal(management-system blueprint) error = %v", err)
	}

	requiredSkillNames := make([]string, 0, len(thinHostSkillNames))
	for _, requiredFile := range blueprintContract.Spec.RequiredFiles {
		const prefix = ".agents/skills/"
		const suffix = "/SKILL.md"
		if !strings.HasPrefix(requiredFile, prefix) || !strings.HasSuffix(requiredFile, suffix) {
			continue
		}
		requiredSkillNames = append(requiredSkillNames, strings.TrimSuffix(strings.TrimPrefix(requiredFile, prefix), suffix))
	}
	sort.Strings(requiredSkillNames)
	if !slices.Equal(requiredSkillNames, thinHostSkillNames) {
		t.Fatalf("management-system required skills = %#v, want %#v", requiredSkillNames, thinHostSkillNames)
	}
}

func TestThinHostSkillsUseOnlyPublicAdopterContracts(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	templateRoot := filepath.Join(repositoryRoot, "templates", "application")
	report, err := Discover(templateRoot)
	if err != nil {
		t.Fatalf("Discover(template) error = %v", err)
	}

	forbidden := []string{
		"go run ./cmd/mss",
		"admin/modules/",
		"web/antd-v6",
		"docs/docs/",
		"templates/",
		".mss/blueprints/",
		".mss/modules/example-supplier.yaml",
		".mss/schemas/",
		"schema.json",
		"$mss-project-onboarding",
		"$mss-add-workflow",
		"mss workflow",
		"--foundation",
	}
	for _, skill := range report.Skills {
		data, readErr := os.ReadFile(filepath.Join(templateRoot, filepath.FromSlash(skill.Path)))
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", skill.Path, readErr)
		}
		content := string(data)
		for _, staleReference := range forbidden {
			if strings.Contains(content, staleReference) {
				t.Errorf("%s contains non-adopter reference %q", skill.Path, staleReference)
			}
		}
	}

	moduleSkill := readThinHostSkill(t, templateRoot, "mss-add-module")
	for _, required := range []string{
		"mss spec init <name> --kind module",
		"--output .mss/modules/<name>.yaml --write",
		"`string`, `enum`, `bool`",
		"spec.ownership.mode: none",
	} {
		if !strings.Contains(moduleSkill, required) {
			t.Errorf("mss-add-module is missing current contract %q", required)
		}
	}

	fieldSkill := readThinHostSkill(t, templateRoot, "mss-add-field")
	if count := strings.Count(fieldSkill, "mss verify --module <module>"); count != 1 {
		t.Errorf("mss-add-field focused verify count = %d, want 1", count)
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

func readThinHostSkill(t *testing.T, templateRoot, name string) string {
	t.Helper()
	path := filepath.Join(templateRoot, ".agents", "skills", name, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
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
