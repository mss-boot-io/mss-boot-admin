package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectContext(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".mss/project.yaml", `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: test-admin
  repository: acme/test-admin
spec:
  foundationVersion: 1.0.0
  repositoryLayout:
    backend: .
    framework: mss-boot
    frontend: web/antd
    documentation: docs
    specifications: .mss
  backend:
    module: example.com/test-admin
`)
	writeTestFile(t, root, ".mss/capabilities.yaml", `apiVersion: mss.io/v1alpha1
kind: CapabilityCatalog
metadata:
  project: test-admin
spec:
  statuses:
    stable: supported
  capabilities:
    - id: z.last
      displayName: Last
      status: stable
    - id: a.first
      displayName: First
      status: stable
`)
	writeTestFile(t, root, ".mss/commands.yaml", `apiVersion: mss.io/v1alpha1
kind: CommandCatalog
metadata:
  project: test-admin
spec:
  commands:
    verify:
      command: go test ./...
      description: test
      category: verification
`)

	ctx, err := Load(filepath.Join(root, "web", "antd"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx.Root != root {
		t.Fatalf("Root = %q, want %q", ctx.Root, root)
	}
	if ctx.Project.Metadata.Name != "test-admin" {
		t.Fatalf("project name = %q", ctx.Project.Metadata.Name)
	}
	if len(ctx.Capabilities.Spec.Capabilities) != 2 {
		t.Fatalf("capability count = %d", len(ctx.Capabilities.Spec.Capabilities))
	}
	if got := ctx.Capabilities.Spec.Capabilities[0].ID; got != "a.first" {
		t.Fatalf("capabilities were not sorted, first = %q", got)
	}
	data, err := ctx.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if !strings.Contains(string(data), `"test-admin"`) {
		t.Fatalf("JSON output does not contain project name: %s", data)
	}
}

func TestLoadRejectsEscapingRepositoryPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".mss/project.yaml", `apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: unsafe
  repository: acme/unsafe
spec:
  foundationVersion: 1.0.0
  repositoryLayout:
    backend: .
    framework: ../outside
    frontend: web/antd
    documentation: docs
    specifications: .mss
  backend:
    module: example.com/unsafe
`)
	writeTestFile(t, root, ".mss/capabilities.yaml", `apiVersion: mss.io/v1alpha1
kind: CapabilityCatalog
spec:
  capabilities: []
`)
	writeTestFile(t, root, ".mss/commands.yaml", `apiVersion: mss.io/v1alpha1
kind: CommandCatalog
spec:
  commands:
    verify:
      command: go test ./...
      category: verification
`)

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("Load() error = %v, want path escape error", err)
	}
}

func TestFindRootFailsOutsideProject(t *testing.T) {
	_, err := FindRoot(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "project root not found") {
		t.Fatalf("FindRoot() error = %v", err)
	}
}

func TestDecodeProjectDocumentRequiresGenerationBaseline(t *testing.T) {
	document, err := DecodeProjectDocument([]byte(`apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: fixture
  repository: acme/foundation
spec:
  foundationVersion: 1.2.3
`))
	if err != nil {
		t.Fatalf("DecodeProjectDocument() error = %v", err)
	}
	if document.Metadata.Repository != "acme/foundation" || document.Spec.FoundationVersion != "1.2.3" {
		t.Fatalf("unexpected project identity: %#v", document)
	}
	_, err = DecodeProjectDocument([]byte(`apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: fixture
  repository: acme/foundation
spec: {}
`))
	if err == nil || !strings.Contains(err.Error(), "foundationVersion") {
		t.Fatalf("DecodeProjectDocument() error = %v, want missing foundation version", err)
	}
}

func TestDecodeProjectDocumentRejectsYAMLGraphFeatures(t *testing.T) {
	_, err := DecodeProjectDocument([]byte(`apiVersion: mss.io/v1alpha1
kind: Project
metadata: &metadata
  name: fixture
  repository: acme/foundation
spec:
  foundationVersion: 1.2.3
`))
	if err == nil || !strings.Contains(err.Error(), "anchors and aliases") {
		t.Fatalf("DecodeProjectDocument() error = %v, want YAML graph rejection", err)
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
