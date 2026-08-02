package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Metadata is the supported Agent Skill front matter.
type Metadata struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

// Skill is one repository-local Agent Skill.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Directory   string `json:"directory"`
	Path        string `json:"path"`
	Body        string `json:"body,omitempty"`
}

// Issue is a deterministic skill validation diagnostic.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Report contains discovered skills and all validation issues.
type Report struct {
	Root   string  `json:"root"`
	Valid  bool    `json:"valid"`
	Skills []Skill `json:"skills"`
	Issues []Issue `json:"issues,omitempty"`
}

// ValidationError exposes all skill contract violations.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s [%s]: %s", issue.Path, issue.Code, issue.Message))
	}
	return strings.Join(parts, "; ")
}

// Discover loads and validates all repository-local skills under .agents/skills.
func Discover(root string) (Report, error) {
	if strings.TrimSpace(root) == "" {
		return Report{}, errors.New("repository root is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, fmt.Errorf("resolve repository root: %w", err)
	}
	skillsRoot := filepath.Join(absoluteRoot, ".agents", "skills")
	report := Report{Root: absoluteRoot, Valid: true}

	entries, err := os.ReadDir(skillsRoot)
	if errors.Is(err, os.ErrNotExist) {
		report.Valid = false
		report.Issues = append(report.Issues, Issue{
			Path:    ".agents/skills",
			Code:    "skills-directory-missing",
			Message: "repository-local skills directory does not exist",
		})
		return report, &ValidationError{Issues: report.Issues}
	}
	if err != nil {
		return Report{}, fmt.Errorf("read skills directory: %w", err)
	}

	nameOwner := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := entry.Name()
		relativePath := filepath.ToSlash(filepath.Join(".agents", "skills", directory, "SKILL.md"))
		absolutePath := filepath.Join(skillsRoot, directory, "SKILL.md")
		data, readErr := os.ReadFile(absolutePath)
		if errors.Is(readErr, os.ErrNotExist) {
			report.Issues = append(report.Issues, Issue{
				Path:    filepath.ToSlash(filepath.Join(".agents", "skills", directory)),
				Code:    "skill-file-missing",
				Message: "skill directory must contain SKILL.md",
			})
			continue
		}
		if readErr != nil {
			return Report{}, fmt.Errorf("read %s: %w", relativePath, readErr)
		}

		skill, issues := parseSkill(relativePath, directory, data)
		report.Issues = append(report.Issues, issues...)
		if skill.Name != "" {
			if previous, exists := nameOwner[skill.Name]; exists {
				report.Issues = append(report.Issues, Issue{
					Path:    relativePath,
					Code:    "duplicate-skill-name",
					Message: fmt.Sprintf("skill name %q is already declared by %s", skill.Name, previous),
				})
			} else {
				nameOwner[skill.Name] = relativePath
			}
		}
		report.Skills = append(report.Skills, skill)
	}

	sort.SliceStable(report.Skills, func(i, j int) bool {
		return report.Skills[i].Name < report.Skills[j].Name
	})
	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Path == report.Issues[j].Path {
			return report.Issues[i].Code < report.Issues[j].Code
		}
		return report.Issues[i].Path < report.Issues[j].Path
	})
	report.Valid = len(report.Issues) == 0
	if !report.Valid {
		return report, &ValidationError{Issues: report.Issues}
	}
	return report, nil
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text returns a compact human-readable validation summary.
func (r Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "skills root: %s\n", filepath.ToSlash(r.Root))
	fmt.Fprintf(&builder, "valid: %t\n", r.Valid)
	fmt.Fprintf(&builder, "skills: %d\n\n", len(r.Skills))
	for _, skill := range r.Skills {
		fmt.Fprintf(&builder, "- %s: %s\n", skill.Name, skill.Description)
	}
	if len(r.Issues) > 0 {
		builder.WriteString("\nissues:\n")
		for _, issue := range r.Issues {
			fmt.Fprintf(&builder, "- %s [%s]: %s\n", issue.Path, issue.Code, issue.Message)
		}
	}
	return builder.String()
}

func parseSkill(path, directory string, data []byte) (Skill, []Issue) {
	skill := Skill{
		Directory: filepath.ToSlash(filepath.Join(".agents", "skills", directory)),
		Path:      path,
	}
	var issues []Issue
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	metadataText, body, err := splitFrontMatter(text)
	if err != nil {
		issues = append(issues, Issue{Path: path, Code: "invalid-front-matter", Message: err.Error()})
		return skill, issues
	}
	var metadata Metadata
	decoder := yaml.NewDecoder(strings.NewReader(metadataText))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		issues = append(issues, Issue{Path: path, Code: "invalid-front-matter-yaml", Message: err.Error()})
		return skill, issues
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	skill.Name = metadata.Name
	skill.Description = metadata.Description
	skill.Body = strings.TrimSpace(body)

	if metadata.Name == "" {
		issues = append(issues, Issue{Path: path, Code: "skill-name-required", Message: "front matter name is required"})
	} else {
		if !skillNamePattern.MatchString(metadata.Name) {
			issues = append(issues, Issue{Path: path, Code: "invalid-skill-name", Message: "name must be lower-case kebab-case"})
		}
		if metadata.Name != directory {
			issues = append(issues, Issue{
				Path:    path,
				Code:    "skill-directory-mismatch",
				Message: fmt.Sprintf("front matter name %q must match directory %q", metadata.Name, directory),
			})
		}
	}
	if metadata.Description == "" {
		issues = append(issues, Issue{Path: path, Code: "skill-description-required", Message: "front matter description is required"})
	} else if len([]rune(metadata.Description)) > 1024 {
		issues = append(issues, Issue{Path: path, Code: "skill-description-too-long", Message: "description must not exceed 1024 characters"})
	}
	if skill.Body == "" {
		issues = append(issues, Issue{Path: path, Code: "skill-body-required", Message: "skill instructions are required after front matter"})
	}
	for _, forbidden := range []struct {
		value string
		code  string
		msg   string
	}{
		{value: "/home/lwx/", code: "personal-absolute-path", msg: "personal absolute paths are not allowed"},
		{value: "mss-boot-admin-antd directory", code: "stale-standalone-repository", msg: "refer to the monorepo frontend path instead of the former standalone repository"},
		{value: "mss-boot-docs/", code: "stale-standalone-repository", msg: "refer to the monorepo docs path instead of the former standalone repository"},
	} {
		if strings.Contains(text, forbidden.value) {
			issues = append(issues, Issue{Path: path, Code: forbidden.code, Message: forbidden.msg})
		}
	}
	return skill, issues
}

func splitFrontMatter(content string) (string, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", errors.New("SKILL.md must start with YAML front matter delimited by ---")
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", "", errors.New("closing YAML front matter delimiter is missing")
	}
	metadata := rest[:end]
	body := rest[end+len("\n---\n"):]
	return metadata, body, nil
}
