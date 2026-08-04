package doctor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestParseComponentsReturnsStableDeduplicatedOrder(t *testing.T) {
	components, err := ParseComponents([]string{"agent", "docs", "agent", "backend"})
	if err != nil {
		t.Fatalf("parse components: %v", err)
	}
	want := []Component{ComponentBackend, ComponentDocs, ComponentAgent}
	if !reflect.DeepEqual(components, want) {
		t.Fatalf("components = %#v, want %#v", components, want)
	}
}

func TestParseComponentsAllOverridesSpecificSelections(t *testing.T) {
	components, err := ParseComponents([]string{"frontend", "all", "agent"})
	if err != nil {
		t.Fatalf("parse components: %v", err)
	}
	want := []Component{ComponentAll}
	if !reflect.DeepEqual(components, want) {
		t.Fatalf("components = %#v, want %#v", components, want)
	}
}

func TestParseComponentsRejectsUnknownComponent(t *testing.T) {
	if _, err := ParseComponents([]string{"database"}); err == nil {
		t.Fatal("expected unsupported component error")
	}
}

func TestRunAgentScopeDoesNotRequireFrontendOrDocsToolchains(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		".mss/project.yaml",
		".mss/capabilities.yaml",
		".mss/commands.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent for %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}

	projectContext := &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{Name: "doctor-test"},
		},
	}
	report := Run(context.Background(), projectContext, WithComponents(ComponentAgent))

	if !reflect.DeepEqual(report.Components, []Component{ComponentAgent}) {
		t.Fatalf("report components = %#v", report.Components)
	}
	checks := make(map[string]Check, len(report.Checks))
	for _, check := range report.Checks {
		checks[check.ID] = check
	}
	for _, required := range []string{
		"file:.mss/project.yaml",
		"file:.mss/capabilities.yaml",
		"file:.mss/commands.yaml",
		"tool:git",
		"tool:go",
	} {
		if _, ok := checks[required]; !ok {
			t.Errorf("required Agent check %q is missing", required)
		}
	}
	for _, excluded := range []string{
		"file:web/antd/pnpm-lock.yaml",
		"file:docs/pnpm-lock.yaml",
		"tool:node",
		"tool:pnpm",
		"port:frontend-port",
	} {
		if _, ok := checks[excluded]; ok {
			t.Errorf("unrelated check %q must not be part of Agent scope", excluded)
		}
	}
}
