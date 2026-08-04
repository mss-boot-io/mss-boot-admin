package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestGenerateDryRunWriteCheckAndDrift(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))

	module := generatorTestModule()
	module.SourcePath = ".mss/admin/modules/supplier.yaml"

	dryRun, err := Generate(module, Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(dry-run) error = %v", err)
	}
	if !dryRun.DryRun {
		t.Fatal("dry-run plan was not marked as dry-run")
	}
	if len(dryRun.Changes) < 10 {
		t.Fatalf("dry-run change count = %d, want generated backend, frontend, docs, and registries", len(dryRun.Changes))
	}
	for _, change := range dryRun.Changes {
		if change.Action != ActionCreate {
			t.Fatalf("initial action for %s = %s, want create", change.Path, change.Action)
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(change.Path))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("dry-run unexpectedly wrote %s, stat error = %v", change.Path, statErr)
		}
	}

	written, err := Generate(module, Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(write) error = %v", err)
	}
	if written.DryRun {
		t.Fatal("write plan was marked dry-run")
	}
	modelPath := filepath.Join(root, filepath.FromSlash("admin/modules/supplier/model_generated.go"))
	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read generated model: %v", err)
	}
	if !strings.Contains(string(modelData), generatedMarker) || !strings.Contains(string(modelData), "type Supplier struct") {
		t.Fatalf("unexpected generated model:\n%s", modelData)
	}

	checked, err := Generate(module, Options{Root: root, Check: true})
	if err != nil {
		t.Fatalf("Generate(check) error = %v", err)
	}
	for _, change := range checked.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("idempotency action for %s = %s, want unchanged", change.Path, change.Action)
		}
	}

	if err := os.WriteFile(modelPath, append(modelData, []byte("// stale\n")...), 0o644); err != nil {
		t.Fatalf("mutate generated model: %v", err)
	}
	_, err = Generate(module, Options{Root: root, Check: true})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("Generate(check stale) error = %v, want DriftError", err)
	}
	if !containsPath(drift.Paths, "admin/modules/supplier/model_generated.go") {
		t.Fatalf("drift paths = %#v", drift.Paths)
	}
}

func TestSafePathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"../outside", "../../outside", "/absolute/path"} {
		if _, err := safePath(root, path); err == nil {
			t.Fatalf("safePath(%q) unexpectedly succeeded", path)
		}
	}
	if path, err := safePath(root, "admin/modules/supplier/model.go"); err != nil || !strings.HasPrefix(path, root) {
		t.Fatalf("safePath(valid) = %q, %v", path, err)
	}
}

func TestGenerateRejectsInvalidSpec(t *testing.T) {
	module := generatorTestModule()
	module.Metadata.Name = "Invalid_Name"
	_, err := Generate(module, Options{Root: t.TempDir()})
	var validationError *spec.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Generate(invalid) error = %v, want ValidationError", err)
	}
}

func generatorTestModule() *spec.Module {
	module := &spec.Module{
		APIVersion: spec.ModuleAPIVersion,
		Kind:       spec.ModuleKind,
		Metadata: spec.ModuleMetadata{
			Name:        "supplier",
			DisplayName: "供应商管理",
			Description: "管理供应商基础资料。",
		},
		Spec: spec.ModuleSpec{
			Entity: spec.EntitySpec{
				GoName: "Supplier",
				Table:  "biz_suppliers",
				IDType: "uuid",
				Fields: []spec.FieldSpec{
					{
						Name:        "code",
						Column:      "code",
						GoName:      "Code",
						DisplayName: "供应商编码",
						Type:        "string",
						Required:    true,
						Unique:      true,
						Searchable:  true,
					},
					{
						Name:        "enabled",
						Column:      "enabled",
						GoName:      "Enabled",
						DisplayName: "启用状态",
						Type:        "bool",
						Required:    true,
						Filterable:  true,
						Default:     true,
					},
				},
			},
			API: spec.APISpec{
				BasePath:   "/suppliers",
				Version:    "v1",
				Operations: []string{"list", "get", "create", "update", "delete"},
			},
			Permissions: []spec.Permission{
				{Action: "list", DisplayName: "列表"},
				{Action: "read", DisplayName: "详情"},
				{Action: "create", DisplayName: "创建"},
				{Action: "update", DisplayName: "更新"},
				{Action: "delete", DisplayName: "删除"},
			},
			Ownership: spec.OwnershipSpec{Mode: "none"},
			Menu: spec.MenuSpec{
				Path:          "/suppliers",
				DisplayName:   "供应商管理",
				DisplayNameEn: "Suppliers",
				Icon:          "shop",
			},
			UI: spec.UISpec{List: true, Form: true, Detail: true},
			Tests: spec.TestSpec{
				Unit:             true,
				API:              true,
				E2E:              true,
				PermissionMatrix: true,
			},
		},
	}
	module.Normalize()
	return module
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copy template tree: %v", err)
	}
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

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if filepath.ToSlash(path) == target {
			return true
		}
	}
	return false
}
