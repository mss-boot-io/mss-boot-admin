package generator

import (
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestGenerateDryRunWriteCheckAndDrift(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))

	module := generatorTestModule()
	module.SourcePath = ".mss/modules/supplier.yaml"

	dryRun, err := Generate(module, Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(dry-run) error = %v", err)
	}
	if !dryRun.DryRun {
		t.Fatal("dry-run plan was not marked as dry-run")
	}
	if !dryRun.Complete {
		t.Fatal("generated module plan did not mark all declared output projections complete")
	}
	if len(dryRun.Projections) == 0 {
		t.Fatal("dry-run plan omitted the field-to-output-kind projection report")
	}
	if dryRun.GeneratorVersion != buildinfo.VersionString() || dryRun.GeneratorCommit != buildinfo.CommitString() {
		t.Fatalf("generator build identity = %q/%q", dryRun.GeneratorVersion, dryRun.GeneratorCommit)
	}
	if dryRun.TemplateRevision != moduleTemplateRevision {
		t.Fatalf("template revision = %q, want %q", dryRun.TemplateRevision, moduleTemplateRevision)
	}
	if len(dryRun.Changes) < 10 {
		t.Fatalf("dry-run change count = %d, want generated backend and registry outputs", len(dryRun.Changes))
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
	for _, generatedPath := range []string{
		"admin/modules/supplier/descriptor_generated.go",
		"admin/modules/supplier/migration_generated.go",
	} {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(generatedPath)))
		if readErr != nil {
			t.Fatalf("read %s: %v", generatedPath, readErr)
		}
		if strings.Contains(string(content), "AutoMigrate") {
			t.Fatalf("%s inferred production DDL with AutoMigrate", generatedPath)
		}
		if strings.HasSuffix(generatedPath, "migration_generated.go") &&
			(!strings.Contains(string(content), "_ = RegisterMigration") || strings.Contains(string(content), "panic(")) {
			t.Fatalf("%s does not retain migration registration errors for preflight", generatedPath)
		}
	}
	apiData, err := os.ReadFile(filepath.Join(root, "admin", "modules", "supplier", "api_generated.go"))
	if err != nil {
		t.Fatalf("read generated API: %v", err)
	}
	if strings.Contains(string(apiData), "func init()") {
		t.Fatal("generated API mounted routes from init")
	}

	secondWrite, err := Generate(module, Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(second write) error = %v", err)
	}
	for _, change := range secondWrite.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("second write action for %s = %s, want unchanged", change.Path, change.Action)
		}
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

func TestRootedNameRejectsEscapes(t *testing.T) {
	for _, path := range []string{"../outside", "../../outside", "/absolute/path"} {
		if _, err := rootedName(path); err == nil {
			t.Fatalf("rootedName(%q) unexpectedly succeeded", path)
		}
	}
	if path, err := rootedName("admin/modules/supplier/model.go"); err != nil || filepath.ToSlash(path) != "admin/modules/supplier/model.go" {
		t.Fatalf("rootedName(valid) = %q, %v", path, err)
	}
}

func TestWriteAtomicRejectsSymlinkedAncestorEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	err = writeAtomic(repository, Change{Path: "linked/output.go", content: []byte("outside\n"), fileMode: 0o644})
	if err == nil {
		t.Fatal("writeAtomic(symlink escape) unexpectedly succeeded")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "output.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rooted write escaped through symlink: %v", statErr)
	}
	if matches, globErr := filepath.Glob(filepath.Join(root, ".mss-generate-*")); globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary output leak = %#v, %v", matches, globErr)
	}
}

func TestWriteAtomicPinsRepositoryRootAcrossPathReplacement(t *testing.T) {
	parent := t.TempDir()
	original := filepath.Join(parent, "repository")
	moved := filepath.Join(parent, "repository-moved")
	outside := t.TempDir()
	if err := os.Mkdir(original, 0o755); err != nil {
		t.Fatalf("Mkdir(repository) error = %v", err)
	}
	repository, err := os.OpenRoot(original)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	if err := os.Rename(original, moved); err != nil {
		t.Fatalf("Rename(repository) error = %v", err)
	}
	if err := os.Symlink(outside, original); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	change := Change{Path: "admin/modules/supplier/model.go", content: []byte("pinned\n"), fileMode: 0o644}
	if err := writeAtomic(repository, change); err != nil {
		t.Fatalf("writeAtomic(pinned root) error = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(moved, filepath.FromSlash(change.Path))); err != nil || string(data) != "pinned\n" {
		t.Fatalf("pinned output = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(outside, filepath.FromSlash(change.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement path received output: %v", err)
	}
}

func TestGenerateCanonicalizesAbsoluteRepositorySourcePath(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := generatorTestModule()
	sourcePath := filepath.Join(root, ".mss", "modules", "supplier.yaml")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	module.SourcePath = sourcePath
	plan, err := Generate(module, Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(absolute inside source) error = %v", err)
	}
	for _, change := range plan.Changes {
		if change.Path != "admin/modules/supplier/model_generated.go" {
			continue
		}
		header := string(change.content)
		if !strings.Contains(header, "from .mss/modules/supplier.yaml") || strings.Contains(header, filepath.ToSlash(root)) {
			t.Fatalf("generated header leaked non-canonical source path: %q", header)
		}
		return
	}
	t.Fatal("generated model output not found")
}

func TestGenerateRejectsExternalOrSymlinkedSourceBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	for _, test := range []struct {
		name   string
		source func(*testing.T, string, []byte) string
	}{
		{
			name: "absolute outside",
			source: func(t *testing.T, _ string, data []byte) string {
				path := filepath.Join(t.TempDir(), "supplier.yaml")
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatalf("WriteFile(outside source) error = %v", err)
				}
				return path
			},
		},
		{
			name: "symlink outside",
			source: func(t *testing.T, root string, data []byte) string {
				outside := filepath.Join(t.TempDir(), "supplier.yaml")
				if err := os.WriteFile(outside, data, 0o644); err != nil {
					t.Fatalf("WriteFile(outside source) error = %v", err)
				}
				linked := filepath.Join(root, ".mss", "modules", "supplier.yaml")
				if err := os.MkdirAll(filepath.Dir(linked), 0o755); err != nil {
					t.Fatalf("MkdirAll(link source) error = %v", err)
				}
				if err := os.Symlink(outside, linked); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				return ".mss/modules/supplier.yaml"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
			module := generatorTestModule()
			data, err := module.YAML()
			if err != nil {
				t.Fatalf("YAML() error = %v", err)
			}
			module.SourcePath = test.source(t, root, data)
			plan, err := Generate(module, Options{Root: root, Write: true})
			if err == nil {
				t.Fatal("Generate(external source) unexpectedly succeeded")
			}
			if len(plan.Changes) != 0 {
				t.Fatalf("external source plan changes = %#v, want none", plan.Changes)
			}
			if _, statErr := os.Stat(filepath.Join(root, "admin", "modules", "supplier")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("external source preflight wrote supplier output: %v", statErr)
			}
		})
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
				Operations: []string{"list", "get", "create", "update", "delete", "export"},
			},
			Permissions: []spec.Permission{
				{Action: "list", DisplayName: "列表"},
				{Action: "read", DisplayName: "详情"},
				{Action: "create", DisplayName: "创建"},
				{Action: "update", DisplayName: "更新"},
				{Action: "delete", DisplayName: "删除"},
				{Action: "export", DisplayName: "导出"},
			},
			Ownership: spec.OwnershipSpec{Mode: "none"},
			Menu: spec.MenuSpec{
				Path:          "/suppliers",
				DisplayName:   "供应商管理",
				DisplayNameEn: "Suppliers",
				Icon:          "shop",
			},
			UI: spec.UISpec{List: true, Form: true, Detail: true, Export: true},
			Tests: spec.TestSpec{
				Unit:             true,
				API:              true,
				E2E:              true,
				PermissionMatrix: true,
			},
			Generation: spec.GenerationSpec{
				MigrationID:              "20260810160001",
				AuthorizationMigrationID: "20260810160002",
			},
		},
	}
	for index := range module.Spec.Permissions {
		module.Spec.Permissions[index].DefaultRoles = []string{"admin"}
	}
	module.Normalize()
	return module
}

func TestGenerateRejectsUnsupportedBackendProjectionBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := generatorTestModule()
	frontend := false
	module.Spec.Generation.Frontend = &frontend
	module.Spec.Generation.FrontendTargets = nil
	module.Spec.Entity.Fields[0].Type = "int64"
	plan, err := Generate(module, Options{Root: root, Write: true})
	var projectionError *ProjectionError
	if !errors.As(err, &projectionError) {
		t.Fatalf("Generate(unsupported) error = %v, want ProjectionError", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("unsupported plan changes = %#v, want none", plan.Changes)
	}
	if _, statErr := os.Stat(filepath.Join(root, "admin")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unsupported projection wrote output: %v", statErr)
	}
}

func TestGenerateRejectsMigrationIDCollisionsBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	for _, test := range []struct {
		name  string
		setup func(*testing.T, string, *spec.Module)
	}{
		{
			name: "another module",
			setup: func(t *testing.T, root string, current *spec.Module) {
				other := generatorTestModule()
				other.Metadata.Name = "customer"
				other.Spec.Entity.GoName = "Customer"
				other.Spec.Entity.Table = "biz_customers"
				other.Spec.API.BasePath = "/customers"
				other.Spec.Menu.Path = "/customers"
				other.Spec.Generation.MigrationID = current.Spec.Generation.MigrationID
				data, err := other.YAML()
				if err != nil {
					t.Fatalf("YAML(other) error = %v", err)
				}
				path := filepath.Join(root, ".mss", "modules", "customer.yaml")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll(module specs) error = %v", err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatalf("WriteFile(other module) error = %v", err)
				}
			},
		},
		{
			name: "another module authorization migration",
			setup: func(t *testing.T, root string, current *spec.Module) {
				other := generatorTestModule()
				other.Metadata.Name = "customer"
				other.Spec.Entity.GoName = "Customer"
				other.Spec.Entity.Table = "biz_customers"
				other.Spec.API.BasePath = "/customers"
				other.Spec.Menu.Path = "/customers"
				other.Spec.Generation.MigrationID = "20260810160003"
				other.Spec.Generation.AuthorizationMigrationID = current.Spec.Generation.AuthorizationMigrationID
				data, err := other.YAML()
				if err != nil {
					t.Fatalf("YAML(other) error = %v", err)
				}
				path := filepath.Join(root, ".mss", "modules", "customer.yaml")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll(module specs) error = %v", err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatalf("WriteFile(other module) error = %v", err)
				}
			},
		},
		{
			name: "file migration",
			setup: func(t *testing.T, root string, current *spec.Module) {
				path := filepath.Join(root, "admin", "cmd", "migrate", "migration", "system", current.Spec.Generation.MigrationID+"_existing.go")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll(migrations) error = %v", err)
				}
				if err := os.WriteFile(path, []byte("package system\n"), 0o644); err != nil {
					t.Fatalf("WriteFile(existing migration) error = %v", err)
				}
			},
		},
		{
			name: "explicit legacy alias",
			setup: func(t *testing.T, root string, current *spec.Module) {
				path := filepath.Join(root, "admin", "cmd", "migrate", "migration", "system", "legacy_aliases.go")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll(migrations) error = %v", err)
				}
				source := "package system\n\nvar aliases = []string{\"" + current.Spec.Generation.MigrationID + "\"}\n"
				if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
					t.Fatalf("WriteFile(existing alias) error = %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
			module := generatorTestModule()
			test.setup(t, root, module)
			plan, err := Generate(module, Options{Root: root, Write: true})
			var collision *MigrationIDCollisionError
			if !errors.As(err, &collision) {
				t.Fatalf("Generate(collision) error = %v, want MigrationIDCollisionError", err)
			}
			if len(plan.Changes) != 0 {
				t.Fatalf("collision plan changes = %#v, want none", plan.Changes)
			}
			if _, statErr := os.Stat(filepath.Join(root, "admin", "modules", "supplier")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("collision preflight wrote supplier output: %v", statErr)
			}
		})
	}
}

func TestGenerateReportsCompleteDocsAndBrowserE2EOutputs(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	plan, err := Generate(generatorTestModule(), Options{Root: root})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	hasProjection := func(path string, status ProjectionStatus) bool {
		for _, projection := range plan.Projections {
			if projection.Path == path && projection.Status == status {
				return true
			}
		}
		return false
	}
	for _, expected := range []struct {
		path   string
		status ProjectionStatus
	}{
		{path: "spec.permissions[0].action", status: ProjectionImplemented},
		{path: "spec.permissions[0].defaultRoles", status: ProjectionImplemented},
		{path: "spec.ownership", status: ProjectionImplemented},
		{path: "spec.ownership.adminBypass", status: ProjectionImplemented},
		{path: "spec.menu", status: ProjectionImplemented},
		{path: "spec.ui", status: ProjectionImplemented},
		{path: "spec.generation.frontend", status: ProjectionImplemented},
		{path: "spec.tests.e2e", status: ProjectionImplemented},
		{path: "spec.generation.docs", status: ProjectionImplemented},
		{path: "spec.generation.authorizationMigrationID", status: ProjectionImplemented},
	} {
		if !hasProjection(expected.path, expected.status) {
			t.Fatalf("projection %s/%s is missing from %#v", expected.path, expected.status, plan.Projections)
		}
	}
	deferred := false
	for _, projection := range plan.Projections {
		if projection.Status == ProjectionDeferred {
			deferred = true
		}
	}
	if deferred || !plan.Complete {
		t.Fatalf("projection truth = deferred:%t complete:%t", deferred, plan.Complete)
	}
	generatedOutputs := map[string]bool{
		"docs/docs/modules/supplier.md":                               false,
		"web/antd-v6/config/routes.generated.ts":                      false,
		"web/antd-v6/e2e/generated/supplier.spec.ts":                  false,
		"web/antd-v6/src/generated/locales/en-US.ts":                  false,
		"web/antd-v6/src/generated/locales/zh-CN.ts":                  false,
		"web/antd-v6/src/generated/modules/supplier/SupplierPage.tsx": false,
		"web/antd-v6/src/generated/modules/supplier/api.ts":           false,
		"web/antd-v6/src/generated/modules/supplier/contract.test.ts": false,
		"web/antd-v6/src/generated/modules/supplier/contract.ts":      false,
		"web/antd-v6/src/generated/modules/supplier/query.ts":         false,
		"web/antd-v6/src/generated/modules/supplier/types.ts":         false,
		"web/antd-v6/src/generated/routes.ts":                         false,
		"web/antd-v6/src/pages/generated/Supplier/index.tsx":          false,
	}
	for _, change := range plan.Changes {
		if _, expected := generatedOutputs[change.Path]; expected {
			generatedOutputs[change.Path] = true
		}
	}
	for path, found := range generatedOutputs {
		if !found {
			t.Fatalf("generated module checkpoint omitted %s", path)
		}
	}
	written, err := Generate(generatorTestModule(), Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(write docs and E2E) error = %v", err)
	}
	if !written.Complete {
		t.Fatal("written generated-module plan was not complete")
	}
	for path, fragments := range map[string][]string{
		"docs/docs/modules/supplier.md": {
			generatedMarker,
			"20260810160001",
			"e2e/generated/supplier.spec.ts",
		},
		"web/antd-v6/e2e/generated/supplier.spec.ts": {
			generatedMarker,
			"MSS_V6_BASE_URL",
			"http://127.0.0.1:18001",
			"mss_admin_session",
		},
	} {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(content), fragment) {
				t.Fatalf("%s omitted %q", path, fragment)
			}
		}
	}
}

func TestGenerateAntDV6TargetIsConfinedAndIdempotent(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := generatorTestModule()
	module.Spec.Generation.FrontendTargets = []string{spec.FrontendTargetAntDV6}

	options := Options{Root: root, Write: true, FrontendTarget: spec.FrontendTargetAntDV6}
	written, err := Generate(module, options)
	if err != nil {
		t.Fatalf("Generate(v6 write) error = %v", err)
	}
	if written.FrontendTarget != spec.FrontendTargetAntDV6 {
		t.Fatalf("frontend target = %q", written.FrontendTarget)
	}
	expected := map[string]bool{
		"web/antd-v6/config/routes.generated.ts":                      false,
		"web/antd-v6/e2e/generated/supplier.spec.ts":                  false,
		"web/antd-v6/src/generated/locales/en-US.ts":                  false,
		"web/antd-v6/src/generated/locales/zh-CN.ts":                  false,
		"web/antd-v6/src/generated/modules/supplier/SupplierPage.tsx": false,
		"web/antd-v6/src/generated/modules/supplier/api.ts":           false,
		"web/antd-v6/src/generated/modules/supplier/contract.test.ts": false,
		"web/antd-v6/src/generated/modules/supplier/contract.ts":      false,
		"web/antd-v6/src/generated/modules/supplier/query.ts":         false,
		"web/antd-v6/src/generated/modules/supplier/types.ts":         false,
		"web/antd-v6/src/generated/routes.ts":                         false,
		"web/antd-v6/src/pages/generated/Supplier/index.tsx":          false,
	}
	for _, change := range written.Changes {
		if _, exists := expected[change.Path]; exists {
			expected[change.Path] = true
		}
	}
	for path, found := range expected {
		if !found {
			t.Fatalf("v6 generation omitted %s", path)
		}
	}

	for path, fragments := range map[string][]string{
		"web/antd-v6/src/generated/modules/supplier/SupplierPage.tsx": {
			"@tanstack/react-query",
			"PageContainer",
			"destroyOnHidden",
			"App.useApp()",
			"name=\"supplier-filters\"",
			"name=\"supplier-editor\"",
			"aria-label={intl.formatMessage",
		},
		"web/antd-v6/src/generated/modules/supplier/contract.ts": {
			"parseSupplierPage",
			"hasPermission",
		},
		"web/antd-v6/e2e/generated/supplier.spec.ts": {
			"mss_admin_session",
			"http://127.0.0.1:18001",
			"X-CSRF-Token",
			"not.toHaveProperty('token')",
			"randomUUID",
			"uniqueFixture",
			"toPass({ timeout: 15_000 })",
		},
	} {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(content), fragment) {
				t.Fatalf("%s omitted %q", path, fragment)
			}
		}
	}

	second, err := Generate(module, options)
	if err != nil {
		t.Fatalf("Generate(v6 second write) error = %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("second v6 action for %s = %s", change.Path, change.Action)
		}
	}
}

func TestGenerateRejectsUnsupportedFrontendTargetBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	plan, err := Generate(generatorTestModule(), Options{
		Root:           root,
		Write:          true,
		FrontendTarget: "unsupported-frontend",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported frontend target") {
		t.Fatalf("Generate(unsupported frontend) error = %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("undeclared target changes = %#v, want none", plan.Changes)
	}
	if _, statErr := os.Stat(filepath.Join(root, "web")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undeclared target wrote frontend output: %v", statErr)
	}
}

func TestGenerateAlignsAuthorizedMenuNamesWithLocaleKeys(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := generatorTestModule()
	module.Spec.Menu.Parent = "/procurement"
	module.Spec.Menu.ParentDisplayName = "采购管理"
	module.Spec.Menu.ParentDisplayNameEn = "Procurement"

	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(menu locale alignment) error = %v", err)
	}
	for path, fragments := range map[string][]string{
		"admin/modules/supplier/authorization_migration_generated.go": {
			"name:       \"supplier\"",
			"Name:   \"procurement\"",
		},
		"web/antd-v6/src/generated/locales/zh-CN.ts": {
			"\"menu.procurement\": \"采购管理\"",
			"\"menu.procurement.supplier\": \"供应商管理\"",
			"\"menu.supplier\": \"供应商管理\"",
		},
		"web/antd-v6/src/generated/locales/en-US.ts": {
			"\"menu.procurement\": \"Procurement\"",
			"\"menu.procurement.supplier\": \"Suppliers\"",
			"\"menu.supplier\": \"Suppliers\"",
		},
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(content), fragment) {
				t.Fatalf("%s omitted aligned menu fragment %q", path, fragment)
			}
		}
	}
}

func TestGenerateProviderMigrationHarnessDisablesInferredForeignKeys(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))

	if _, err := Generate(generatorTestModule(), Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(provider migration harness) error = %v", err)
	}
	path := filepath.Join(root, "admin", "modules", "supplier", "migration_provider_generated_test.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated provider migration harness: %v", err)
	}
	if !strings.Contains(string(content), "DisableForeignKeyConstraintWhenMigrating: true") {
		t.Fatal("generated provider migration harness enabled inferred foreign keys unlike the Admin runtime")
	}
}

func TestGenerateWidensComparableE2EFieldNames(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))

	if _, err := Generate(generatorTestModule(), Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(comparable E2E fields) error = %v", err)
	}
	path := filepath.Join(root, "web", "antd-v6", "e2e", "generated", "supplier.spec.ts")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated E2E: %v", err)
	}
	for _, fragment := range []string{
		`const UNIQUE_FIELD: string = `,
		`const ROW_FIELD: string = `,
		`const UPDATE_FIELD: string = `,
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("generated E2E omitted widened field name %q", fragment)
		}
	}
}

func TestGenerateRejectsUserOwnedManagedPathBeforeAnyWrite(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	conflictPath := filepath.Join(root, "admin", "modules", "supplier", "model_generated.go")
	if err := os.MkdirAll(filepath.Dir(conflictPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(conflictPath, []byte("package supplier\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := Generate(generatorTestModule(), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Generate(conflict) error = %v, want ConflictError", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "admin", "modules", "supplier", "dto_generated.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("conflict preflight allowed partial writes: %v", statErr)
	}
}

func TestGeneratePlansObsoleteManagedOutputsBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	obsolete := []string{
		"admin/modules/supplier/controller_generated.go",
		"admin/modules/supplier/search_generated.go",
	}
	for _, relative := range obsolete {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("// Code generated by mss. DO NOT EDIT.\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", relative, err)
		}
	}
	plan, err := Generate(generatorTestModule(), Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(dry-run obsolete) error = %v", err)
	}
	for _, relative := range obsolete {
		found := false
		for _, change := range plan.Changes {
			if change.Path == relative && change.Action == ActionDelete {
				found = true
			}
		}
		if !found {
			t.Fatalf("obsolete output %s was not planned for deletion", relative)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("dry-run mutated %s: %v", relative, err)
		}
	}
	_, err = Generate(generatorTestModule(), Options{Root: root, Check: true})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("Generate(check obsolete) error = %v, want DriftError", err)
	}
	for _, relative := range obsolete {
		if !containsPath(drift.Paths, relative) {
			t.Fatalf("obsolete check drift omitted %s: %#v", relative, drift.Paths)
		}
	}
	if _, err := Generate(generatorTestModule(), Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(write obsolete) error = %v", err)
	}
	for _, relative := range obsolete {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obsolete output %s still exists: %v", relative, err)
		}
	}
}

func TestGenerateRejectsUserOwnedObsoletePathBeforeAnyWrite(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	obsolete := filepath.Join(root, "admin", "modules", "supplier", "controller_generated.go")
	if err := os.MkdirAll(filepath.Dir(obsolete), 0o755); err != nil {
		t.Fatalf("MkdirAll(obsolete): %v", err)
	}
	if err := os.WriteFile(obsolete, []byte("package supplier\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(obsolete): %v", err)
	}
	plan, err := Generate(generatorTestModule(), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Generate(user-owned obsolete) error = %v, want ConflictError", err)
	}
	if conflict.Path != "admin/modules/supplier/controller_generated.go" {
		t.Fatalf("conflict path = %q", conflict.Path)
	}
	if len(plan.Changes) == 0 {
		t.Fatal("preflight did not report already compared outputs")
	}
	if _, statErr := os.Stat(filepath.Join(root, "admin", "modules", "supplier", "dto_generated.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("obsolete conflict allowed partial writes: %v", statErr)
	}
}

func TestGeneratedUpgradeRemovesLegacyAutoMountedController(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	controller := filepath.Join(root, "admin", "modules", "supplier", "controller_generated.go")
	if err := os.MkdirAll(filepath.Dir(controller), 0o755); err != nil {
		t.Fatalf("MkdirAll(controller): %v", err)
	}
	legacy := "// Code generated by mss. DO NOT EDIT.\npackage supplier\nfunc init() { panic(\"legacy auto mount\") }\n"
	if err := os.WriteFile(controller, []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile(controller): %v", err)
	}
	if _, err := Generate(generatorTestModule(), Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(upgrade) error = %v", err)
	}
	if _, err := os.Stat(controller); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy auto-mounted controller still exists: %v", err)
	}
	api, err := os.ReadFile(filepath.Join(root, "admin", "modules", "supplier", "api_generated.go"))
	if err != nil {
		t.Fatalf("ReadFile(api): %v", err)
	}
	if strings.Contains(string(api), "func init()") {
		t.Fatal("replacement API contains an automatic route mount")
	}
}

func TestGenerateEventShapesCompileOrRejectBeforeWrite(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	t.Run("empty events compile", func(t *testing.T) {
		root := t.TempDir()
		copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
		module := generatorTestModule()
		module.Spec.Events = nil
		plan, err := Generate(module, Options{Root: root})
		if err != nil {
			t.Fatalf("Generate(empty events) error = %v", err)
		}
		for _, change := range plan.Changes {
			if change.Path != "admin/modules/supplier/events_generated.go" {
				continue
			}
			fileSet := token.NewFileSet()
			file, parseErr := parser.ParseFile(fileSet, change.Path, change.content, parser.AllErrors)
			if parseErr != nil {
				t.Fatalf("parse empty-events source: %v", parseErr)
			}
			configuration := types.Config{Importer: importer.Default()}
			if _, typeErr := configuration.Check("supplier", fileSet, []*ast.File{file}, nil); typeErr != nil {
				t.Fatalf("type-check empty-events source: %v", typeErr)
			}
			return
		}
		t.Fatal("events output not found")
	})
	for _, test := range []struct {
		name   string
		events []spec.EventSpec
	}{
		{name: "duplicate trigger", events: []spec.EventSpec{{Name: "supplier.created", When: "created"}, {Name: "supplier.also-created", When: "created"}}},
		{name: "invalid name", events: []spec.EventSpec{{Name: "supplier\"invalid", When: "created"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
			module := generatorTestModule()
			module.Spec.Events = test.events
			plan, err := Generate(module, Options{Root: root, Write: true})
			var validationError *spec.ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("Generate(%s) error = %v, want ValidationError", test.name, err)
			}
			if len(plan.Changes) != 0 {
				t.Fatalf("Generate(%s) changes = %#v, want none", test.name, plan.Changes)
			}
			if _, statErr := os.Stat(filepath.Join(root, "admin")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Generate(%s) wrote before validation: %v", test.name, statErr)
			}
		})
	}
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
