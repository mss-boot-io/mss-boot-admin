package blueprint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesCompleteIdempotentApplication(t *testing.T) {
	root := writeBlueprintFixture(t)
	destination := filepath.Join(root, ".mss", "output", "customer-admin")
	options := Options{
		FoundationRoot: root,
		Blueprint:      "management-system",
		Destination:    destination,
		Application: Application{
			Name:        "customer-admin",
			DisplayName: "Customer Administration",
			Module:      "github.com/acme/customer-admin",
			Repository:  "acme/customer-admin",
		},
	}

	dryRun, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("dry-run application generation: %v", err)
	}
	if !dryRun.DryRun || !dryRun.Success || dryRun.TotalFiles < 12 {
		t.Fatalf("unexpected dry-run plan: %#v", dryRun)
	}
	if dryRun.Identities.Foundation.Version != "1.1.0" || dryRun.Identities.Foundation.Channel != "candidate" || !sha256Pattern.MatchString(dryRun.Identities.Snapshot.SHA256) {
		t.Fatalf("dry-run omitted independent snapshot identities: %#v", dryRun.Identities)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry-run created destination: %v", err)
	}

	options.Write = true
	written, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("write application: %v", err)
	}
	if written.DryRun || !written.Success {
		t.Fatalf("unexpected write plan: %#v", written)
	}

	assertContains(t, filepath.Join(destination, "go.mod"), "module github.com/acme/customer-admin")
	assertContains(t, filepath.Join(destination, "admin", "go.mod"), "module github.com/acme/customer-admin/admin")
	assertContains(t, filepath.Join(destination, "admin/main.go"), `"github.com/acme/customer-admin/admin/internal/example"`)
	assertContains(t, filepath.Join(destination, "admin/main.go"), `"github.com/acme/customer-admin/mss-boot/pkg/config"`)
	assertContains(t, filepath.Join(destination, "mss-boot", "go.mod"), "module github.com/acme/customer-admin/mss-boot")
	assertContains(t, filepath.Join(destination, ".mss", "project.yaml"), "repository: acme/customer-admin")
	assertContains(t, filepath.Join(destination, ".mss", "lock.yaml"), "repository: mss-boot-io/mss-boot-admin")

	binary, err := os.ReadFile(filepath.Join(destination, "web", "antd", "public", "fixture.bin"))
	if err != nil {
		t.Fatalf("read generated binary: %v", err)
	}
	if !bytes.Equal(binary, []byte{0, 1, 2, 255}) {
		t.Fatalf("binary was transformed: %#v", binary)
	}
	if _, err := os.Stat(filepath.Join(destination, ".mss", "reports", "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("excluded report was generated: %v", err)
	}

	manifest, err := ReadManifest(destination, "")
	if err != nil {
		t.Fatalf("read generated manifest: %v", err)
	}
	if manifest.Metadata.Project != "customer-admin" || manifest.Metadata.Module != "github.com/acme/customer-admin" {
		t.Fatalf("unexpected manifest metadata: %#v", manifest.Metadata)
	}
	if manifest.Metadata.FoundationRepository != "mss-boot-io/mss-boot-admin" || manifest.Metadata.FoundationCommit == "" {
		t.Fatalf("unexpected foundation metadata: %#v", manifest.Metadata)
	}
	if _, exists := manifest.Files["go.mod"]; !exists {
		t.Fatal("manifest does not record go.mod")
	}
	lockBefore, err := os.Stat(filepath.Join(destination, filepath.FromSlash(manifest.Records.LockPath)))
	if err != nil {
		t.Fatalf("stat lock before repeat generation: %v", err)
	}
	manifestBefore, err := os.Stat(filepath.Join(destination, filepath.FromSlash(manifest.Records.ManifestPath)))
	if err != nil {
		t.Fatalf("stat manifest before repeat generation: %v", err)
	}

	second, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatalf("repeat application generation: %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("repeat generation is not idempotent: %#v", change)
		}
	}
	lockAfter, err := os.Stat(filepath.Join(destination, filepath.FromSlash(manifest.Records.LockPath)))
	if err != nil {
		t.Fatalf("stat lock after repeat generation: %v", err)
	}
	manifestAfter, err := os.Stat(filepath.Join(destination, filepath.FromSlash(manifest.Records.ManifestPath)))
	if err != nil {
		t.Fatalf("stat manifest after repeat generation: %v", err)
	}
	if !os.SameFile(lockBefore, lockAfter) || !os.SameFile(manifestBefore, manifestAfter) {
		t.Fatal("repeat generation rewrote unchanged snapshot records")
	}
}

func TestGenerateInitializesPinnedMinimalGitRepository(t *testing.T) {
	root := writeBlueprintFixture(t)
	destination := filepath.Join(t.TempDir(), "git-admin")
	_, err := Generate(context.Background(), Options{
		FoundationRoot: root,
		Destination:    destination,
		Application: Application{
			Name:        "git-admin",
			DisplayName: "Git Administration",
			Module:      "github.com/acme/git-admin",
			Repository:  "acme/git-admin",
		},
		Write:         true,
		InitializeGit: true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	command := exec.Command("git", "-C", destination, "symbolic-ref", "--short", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "main" {
		t.Fatalf("generated Git repository HEAD: output=%q err=%v", output, err)
	}
}

func TestGenerateRejectsModifiedDestination(t *testing.T) {
	root := writeBlueprintFixture(t)
	destination := filepath.Join(root, ".mss", "output", "conflict-admin")
	options := Options{
		FoundationRoot: root,
		Destination:    destination,
		Application: Application{
			Name:       "conflict-admin",
			Module:     "github.com/acme/conflict-admin",
			Repository: "acme/conflict-admin",
		},
		Write: true,
	}
	if _, err := Generate(context.Background(), options); err != nil {
		t.Fatalf("initial application generation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "admin/main.go"), []byte("package changed\n"), 0o644); err != nil {
		t.Fatalf("modify generated file: %v", err)
	}
	plan, err := Generate(context.Background(), options)
	if err == nil {
		t.Fatal("expected modified destination to fail generation")
	}
	if plan.Success {
		t.Fatalf("conflicting plan reported success: %#v", plan)
	}
	var found bool
	for _, change := range plan.Changes {
		if change.Path == "admin/main.go" && change.Action == ActionConflict {
			found = true
		}
	}
	if !found {
		t.Fatalf("main.go conflict not reported: %#v", plan.Changes)
	}
}

func TestGenerateRejectsUnknownDestinationFile(t *testing.T) {
	root := writeBlueprintFixture(t)
	destination := filepath.Join(root, ".mss", "output", "unknown-admin")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "private.txt"), []byte("custom"), 0o644); err != nil {
		t.Fatalf("write unknown file: %v", err)
	}
	plan, err := Generate(context.Background(), Options{
		FoundationRoot: root,
		Destination:    destination,
		Application: Application{
			Name:       "unknown-admin",
			Module:     "github.com/acme/unknown-admin",
			Repository: "acme/unknown-admin",
		},
	})
	if err != nil {
		t.Fatalf("dry-run should return a conflict plan without an execution error: %v", err)
	}
	if plan.Success {
		t.Fatal("unknown destination file was not reported as a conflict")
	}
}

func TestLoadRejectsUnsafeBlueprintPath(t *testing.T) {
	root := writeBlueprintFixture(t)
	path := filepath.Join(root, ".mss", "blueprints", "management-system.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blueprint: %v", err)
	}
	data = bytes.Replace(data, []byte("defaultOutputDirectory: .mss/output"), []byte("defaultOutputDirectory: ../outside"), 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("rewrite blueprint: %v", err)
	}
	if _, err := Load(root, "management-system"); err == nil || !strings.Contains(err.Error(), "confined") {
		t.Fatalf("expected unsafe output path error, got %v", err)
	}
}

func writeBlueprintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{
		".mss/blueprints/management-system.yaml": []byte(`apiVersion: mss.io/v1alpha1
kind: ApplicationBlueprint
metadata:
  name: management-system
  displayName: Agent-Native Management System
  version: 0.1.0
spec:
  sourceMode: git-tracked
  sourceModule: github.com/mss-boot-io/mss-boot-admin
  sourceProjectName: mss-boot-admin
  defaultOutputDirectory: .mss/output
  manifestPath: .mss/blueprint-manifest.json
  lockPath: .mss/lock.yaml
  requiredFiles:
    - AGENTS.md
    - go.mod
    - go.work
    - admin/go.mod
    - admin/main.go
    - Makefile
    - .mss/project.yaml
    - .mss/release-policy.yaml
    - .mss/capabilities.yaml
    - .mss/commands.yaml
    - web/antd-v6/package.json
    - docs/package.json
    - mss-boot/go.mod
  excludePrefixes:
    - .mss/reports/
    - .mss/output/
  textExtensions: [.go, .json, .md, .mod, .sum, .yaml, .yml]
  textNames: [Makefile]
`),
		"AGENTS.md":     []byte("# mss-boot-admin Agent Contract\n"),
		"go.mod":        []byte("module github.com/mss-boot-io/mss-boot-admin\n\ngo 1.26.0\n"),
		"go.work":       []byte("go 1.26.0\n\nuse (\n\t.\n\t./admin\n\t./mss-boot\n)\n"),
		"admin/go.mod":  []byte("module github.com/mss-boot-io/mss-boot-admin/admin\n\ngo 1.26.0\n\nrequire github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0\n\nreplace github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0 => ../mss-boot\n"),
		"admin/main.go": []byte("package main\n\nimport (\n\t_ \"github.com/mss-boot-io/mss-boot-admin/admin/internal/example\"\n\t_ \"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config\"\n)\n\nfunc main() {}\n"),
		"Makefile":      []byte("PROJECT:=mss-boot-admin\n"),
		".mss/project.yaml": []byte(`apiVersion: mss.io/v1alpha1
kind: Project
metadata:
  name: mss-boot-admin
  displayName: mss-boot Agent-Native Management Foundation
  repository: mss-boot-io/mss-boot-admin
spec:
  foundationVersion: 1.0.0
  repositoryLayout:
    backend: admin
    framework: mss-boot
		frontend: web/antd-v6
    documentation: docs
    specifications: .mss
  backend:
    module: github.com/mss-boot-io/mss-boot-admin/admin
    frameworkModule: github.com/mss-boot-io/mss-boot-admin/mss-boot
`),
		".mss/capabilities.yaml": []byte("apiVersion: mss.io/v1alpha1\nkind: CapabilityCatalog\nmetadata:\n  project: mss-boot-admin\nspec:\n  capabilities: []\n"),
		".mss/commands.yaml":     []byte("apiVersion: mss.io/v1alpha1\nkind: CommandCatalog\nmetadata:\n  project: mss-boot-admin\nspec:\n  commands:\n    context:\n      command: go run ./cmd/mss context\n      description: Context\n      category: agent\n"),
		".mss/release-policy.yaml": []byte(`apiVersion: mss.io/v1alpha1
kind: ReleasePolicy
metadata:
  name: fixture
spec:
  mode: development-first
  currentStableVersion: v1.0.0
  currentStableCommit: 0000000000000000000000000000000000000000
  nextPublicVersion: v1.1.0
  publicationWorkflowsReady: false
  publicPrereleases: false
  rootTagTemplate: "{version}"
  frameworkTagTemplate: "mss-boot/{version}"
  frontendTagTemplate: "web/antd-v6/{version}"
`),
		".mss/lock.yaml":                 []byte("apiVersion: mss.io/v1alpha1\nkind: FoundationLock\nmetadata:\n  project: mss-boot-admin\n"),
		"web/antd-v6/package.json":       []byte(`{"name":"mss-boot-admin-antd-v6"}`),
		"docs/package.json":              []byte(`{"name":"mss-boot-docs"}`),
		"mss-boot/go.mod":                []byte("module github.com/mss-boot-io/mss-boot-admin/mss-boot\n\ngo 1.26.0\n"),
		"web/antd-v6/public/fixture.bin": {0, 1, 2, 255},
		".mss/reports/ignored.txt":       []byte("ignore me"),
	}
	for relative, data := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-08-02T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-02T12:00:00Z")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, string(output))
	}
}

func assertContains(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), expected) {
		t.Fatalf("%s does not contain %q:\n%s", path, expected, string(data))
	}
}

func TestManifestJSONRoundTrip(t *testing.T) {
	manifest := Manifest{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "BlueprintManifest",
		Metadata: ManifestMetadata{
			Project:   "test",
			Module:    "github.com/acme/test",
			Blueprint: "management-system",
		},
		Files: map[string]ManifestFile{"go.mod": {SHA256: "abc", Mode: 0o644, Size: 10}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if decoded.Files["go.mod"].Mode != 0o644 {
		t.Fatalf("mode did not round-trip: %#o", decoded.Files["go.mod"].Mode)
	}
}
