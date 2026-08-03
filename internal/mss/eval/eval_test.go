package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogRejectsDuplicateCases(t *testing.T) {
	catalog := &Catalog{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "AgentEvaluationCatalog",
		Metadata:   CatalogMetadata{Project: "fixture", Version: "0.1.0"},
		Spec: CatalogSpec{Cases: []Case{
			{ID: "duplicate", Title: "One", Checks: []CheckSpec{{Type: "mcp-tools"}}},
			{ID: "duplicate", Title: "Two", Checks: []CheckSpec{{Type: "mcp-tools"}}},
		}},
	}
	if err := catalog.Validate(); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("expected duplicate case error, got %v", err)
	}
}

func TestRunMCPToolsEvaluation(t *testing.T) {
	root := writeEvaluationFixture(t)
	report, err := Run(context.Background(), root, []string{"mcp-project-tools"})
	if err != nil {
		t.Fatalf("run evaluation: %v", err)
	}
	if !report.Success || len(report.Cases) != 1 || !report.Cases[0].Success {
		t.Fatalf("unexpected evaluation report: %#v", report)
	}
	for _, relative := range []string{
		".mss/reports/evals/latest.json",
		".mss/reports/evals/latest.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing report %s: %v", relative, err)
		}
	}
	latest, err := ReadLatest(root)
	if err != nil {
		t.Fatalf("read latest report: %v", err)
	}
	if latest.Project != "fixture" || !latest.Success {
		t.Fatalf("unexpected latest report: %#v", latest)
	}
}

func TestCatalogListRejectsUnknownCase(t *testing.T) {
	catalog := &Catalog{Spec: CatalogSpec{Cases: []Case{{ID: "known"}}}}
	if _, err := catalog.List([]string{"missing"}); err == nil {
		t.Fatal("expected unknown evaluation case to fail")
	}
}

func writeEvaluationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".mss/project.yaml": `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: fixture
spec:
  repositoryLayout:
    backend: .
    framework: framework
    frontend: web
    documentation: docs
    specifications: .mss
  backend:
    module: example.com/fixture
`,
		".mss/capabilities.yaml": `apiVersion: mss.io/v1alpha1
kind: CapabilityCatalog
metadata:
  project: fixture
spec:
  capabilities: []
`,
		".mss/commands.yaml": `apiVersion: mss.io/v1alpha1
kind: CommandCatalog
metadata:
  project: fixture
spec:
  commands:
    context:
      command: mss context
      description: context
      category: agent
`,
		".mss/evals/catalog.yaml": `apiVersion: mss.io/v1alpha1
kind: AgentEvaluationCatalog
metadata:
  project: fixture
  version: 0.1.0
spec:
  cases:
    - id: mcp-project-tools
      title: MCP tools
      description: MCP tools remain available.
      checks:
        - type: mcp-tools
          minimum: 7
`,
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
	return root
}
