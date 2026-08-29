package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCorePresentationTrackedOutputsMatchOneCanonicalSource(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	repository, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v", err)
	}
	defer repository.Close()
	layout, err := resolveTargetLayout(nil)
	if err != nil {
		t.Fatalf("resolveTargetLayout(nil) error = %v", err)
	}
	outputs, err := renderCorePresentationOutputs(repository, layout)
	if err != nil {
		t.Fatalf("renderCorePresentationOutputs() error = %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("core output count = %d, want 3", len(outputs))
	}
	for _, output := range outputs {
		actual, readErr := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(output.path)))
		if readErr != nil {
			t.Fatalf("read tracked core output %s: %v", output.path, readErr)
		}
		if string(actual) != string(output.content) {
			t.Fatalf("tracked core output %s is stale", output.path)
		}
		if output.source != ".mss/core-pages/user-list.yaml" {
			t.Fatalf("core output %s source = %q", output.path, output.source)
		}
	}

	var snapshot corePresentationSnapshot
	snapshotData := readGeneratedTestFile(t, repositoryRoot, "admin/presentation/core/manifest.generated.json")
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		t.Fatalf("parse tracked core snapshot: %v", err)
	}
	if !slices.Equal(snapshot.Sources, []string{".mss/core-pages/user-list.yaml"}) || len(snapshot.Manifests) != 1 {
		t.Fatalf("core snapshot provenance = %#v", snapshot)
	}
	projection := &snapshot.Manifests[0]
	canonical, err := canonicalPresentationProjection(projection)
	if err != nil {
		t.Fatalf("canonicalPresentationProjection(user.list) error = %v", err)
	}
	digest := sha256.Sum256(canonical)
	wantHash := "sha256:" + hex.EncodeToString(digest[:])
	if projection.DefinitionHash != wantHash {
		t.Fatalf("snapshot hash = %q, want %q", projection.DefinitionHash, wantHash)
	}
	backend := string(readGeneratedTestFile(t, repositoryRoot, "admin/presentation/core/definitions_generated.go"))
	frontend := string(readGeneratedTestFile(t, repositoryRoot, "web/antd-v6/src/generated/core-presentation-registry.generated.ts"))
	for path, content := range map[string]string{"backend": backend, "frontend": frontend} {
		if !strings.Contains(content, wantHash) || !strings.Contains(content, ".mss/core-pages/user-list.yaml") {
			t.Errorf("%s core projection omitted hash or source provenance", path)
		}
	}
	for _, contract := range []string{
		"export const corePresentationInventory", "export const corePresentationRegistry", `"user.list"`,
		`"user-identity"`, `"user-role"`, `"user-organization"`, `"status-tag"`, `"status-filter"`,
		`"maxSortFields": 0`, `"defaultSort": []`, `"requiredPermissions": [`, `"/users"`,
	} {
		if !strings.Contains(frontend, contract) {
			t.Errorf("generated frontend core registry omitted %q", contract)
		}
	}
	lower := strings.ToLower(frontend)
	for _, forbidden := range []string{
		"password", "confirmpassword", "roleid", "departmentid", "postid", `"root"`,
		"accesstoken", "sessionid", "oauth", "dynamic import", "import(", "fetch(", "axios",
	} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("generated frontend core registry contains forbidden %q", forbidden)
		}
	}
}

func TestGenerateCorePresentationIsIdempotentAndCleansRemovedFoundationSource(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	copyTree(t, filepath.Join(repositoryRoot, ".mss", "core-pages"), filepath.Join(root, ".mss", "core-pages"))
	module := loadPresentationTestModule(t, repositoryRoot)

	first, err := Generate(module, Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(core write) error = %v", err)
	}
	for _, path := range corePresentationOutputGroupPaths(targetLayout{BackendDir: "admin", GeneratedDir: "web/antd-v6/src/generated"}) {
		change, ok := findChange(first.Changes, path)
		if !ok || change.Action != ActionCreate || !change.Managed {
			t.Fatalf("core output %s change = %#v, found=%t", path, change, ok)
		}
	}
	second, err := Generate(module, Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(core second write) error = %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("second core generation changed %s: %s", change.Path, change.Action)
		}
	}
	if _, err := Generate(module, Options{Root: root, Check: true}); err != nil {
		t.Fatalf("Generate(core check) error = %v", err)
	}

	if err := os.Remove(filepath.Join(root, ".mss", "core-pages", "user-list.yaml")); err != nil {
		t.Fatalf("remove temporary core source: %v", err)
	}
	plan, err := Generate(module, Options{Root: root, Check: true})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("Generate(core removed check) plan=%#v error=%v", plan, err)
	}
	wantDeleted := corePresentationOutputGroupPaths(targetLayout{BackendDir: "admin", GeneratedDir: "web/antd-v6/src/generated"})
	for _, path := range wantDeleted {
		if !slices.Contains(drift.Paths, path) {
			t.Errorf("removed core source drift omitted %s: %#v", path, drift.Paths)
		}
	}
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(core removed cleanup) error = %v", err)
	}
	for _, path := range wantDeleted {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("obsolete core output %s remains: %v", path, err)
		}
	}
}

func TestGenerateThinHostNeverCopiesCorePresentationSourceOrOutputs(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, ".mss", "core-pages"), filepath.Join(root, ".mss", "core-pages"))
	module := generatorTestModule()
	module.SourcePath = ".mss/modules/example-supplier.yaml"
	document := thinHostProjectDocument()
	plan, err := Generate(module, Options{Root: root, Write: true, Project: &document})
	if err != nil {
		t.Fatalf("Generate(thin host with copied core source) error = %v", err)
	}
	for _, change := range plan.Changes {
		if strings.Contains(change.Path, "core-presentation") || strings.Contains(change.Path, "presentation/core") {
			t.Errorf("Thin Host planned Foundation core output: %#v", change)
		}
	}
	for _, path := range []string{
		"admin/presentation/core/definitions_generated.go",
		"admin/presentation/core/manifest.generated.json",
		"web/antd-v6/src/generated/core-presentation-registry.generated.ts",
		"web/src/generated/core-presentation-registry.generated.ts",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Thin Host contains Foundation core output %s: %v", path, err)
		}
	}
}
