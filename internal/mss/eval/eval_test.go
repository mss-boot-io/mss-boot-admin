package eval

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/doctor"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
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

func TestCatalogRejectsInvertedBlueprintBounds(t *testing.T) {
	catalog := &Catalog{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "AgentEvaluationCatalog",
		Metadata:   CatalogMetadata{Project: "fixture", Version: "0.1.0"},
		Spec: CatalogSpec{Cases: []Case{{
			ID:    "application-blueprint",
			Title: "Application Blueprint",
			Checks: []CheckSpec{{
				Type:    "application-blueprint-plan",
				Minimum: 64,
				Maximum: 30,
			}},
		}}},
	}
	if err := catalog.Validate(); err == nil || !strings.Contains(err.Error(), "minimum must not exceed maximum") {
		t.Fatalf("expected inverted Blueprint bounds error, got %v", err)
	}
}

func TestCatalogRejectsMaximumForUnboundedCheck(t *testing.T) {
	catalog := &Catalog{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "AgentEvaluationCatalog",
		Metadata:   CatalogMetadata{Project: "fixture", Version: "0.1.0"},
		Spec: CatalogSpec{Cases: []Case{{
			ID:     "mcp-tools",
			Title:  "MCP tools",
			Checks: []CheckSpec{{Type: "mcp-tools", Maximum: 20}},
		}}},
	}
	if err := catalog.Validate(); err == nil || !strings.Contains(err.Error(), "maximum is supported only") {
		t.Fatalf("expected unsupported maximum error, got %v", err)
	}
}

func TestApplicationBlueprintSizeEnforcesThinHostBounds(t *testing.T) {
	tests := []struct {
		name       string
		totalFiles int
		want       string
	}{
		{name: "within bounds", totalFiles: 31},
		{name: "incomplete host", totalFiles: 29, want: "expected at least 30"},
		{name: "copied foundation sources", totalFiles: 500, want: "expected at most 64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateApplicationBlueprintSize(test.totalFiles, 30, 64)
			if test.want == "" {
				if err != nil {
					t.Fatalf("validate Thin Host file count: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate Thin Host file count error = %v, want %q", err, test.want)
			}
		})
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

func TestDoctorEvaluationUsesAgentScope(t *testing.T) {
	root := writeEvaluationFixture(t)
	projectContext, err := project.Load(root)
	if err != nil {
		t.Fatalf("load project fixture: %v", err)
	}
	details, err := checkDoctor(context.Background(), projectContext)
	if err != nil {
		t.Fatalf("check Agent doctor: %v", err)
	}
	components, ok := details["components"].([]doctor.Component)
	if !ok {
		t.Fatalf("doctor components have unexpected type: %#v", details["components"])
	}
	if want := []doctor.Component{doctor.ComponentAgent}; !reflect.DeepEqual(components, want) {
		t.Fatalf("doctor components = %#v, want %#v", components, want)
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
  repository: example/fixture
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
		".mss/lock.yaml": `apiVersion: mss.io/v1alpha1
kind: FoundationLock
metadata:
  project: fixture
spec:
  foundation:
    repository: example/fixture
    version: 0.1.0
    channel: development
  blueprint:
    name: management-system
    version: 0.1.0
  contracts:
    project: v1alpha1
  generatedBy:
    tool: mss
    version: 0.1.0-dev
  modules: {}
  upgrades: []
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
