package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestGeneratePresentationCanonicalProjectionUsesJSONStringifyEscaping(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := loadPresentationTestModule(t, repositoryRoot)
	module.Spec.Presentation.Title.EnUS = "Admin & <Console>\u2028Paragraph\u2029Literal\\u2028"

	projection, canonical, err := normalizedPresentationProjection(module)
	if err != nil || projection == nil {
		t.Fatalf("normalizedPresentationProjection(special title) = %#v, %v", projection, err)
	}
	projectedCanonical, err := canonicalPresentationProjection(projection)
	if err != nil {
		t.Fatalf("canonicalPresentationProjection(special title) error = %v", err)
	}
	if !bytes.Equal(projectedCanonical, canonical) {
		t.Fatalf("projected canonical bytes diverged from normalized manifest:\nprojected: %s\nnormalized: %s", projectedCanonical, canonical)
	}
	expectedTitle := []byte(`"en-US":"Admin & <Console>` + "\u2028" + `Paragraph` + "\u2029" + `Literal\\u2028"`)
	if !bytes.Contains(canonical, expectedTitle) {
		t.Fatalf("canonical JSON did not preserve JSON.stringify-compatible title bytes: %s", canonical)
	}
	digest := sha256.Sum256(canonical)
	wantHash := "sha256:" + hex.EncodeToString(digest[:])
	if projection.DefinitionHash != wantHash {
		t.Fatalf("projection hash = %q, want %q", projection.DefinitionHash, wantHash)
	}
	if _, err := Generate(module, Options{Root: root}); err != nil {
		t.Fatalf("Generate(special title) error = %v", err)
	}
}

func TestGeneratePresentationArtifactsHaveOneV2IdentityAndAreIdempotent(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := loadPresentationTestModule(t, repositoryRoot)

	plan, err := Generate(module, Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(presentation dry-run) error = %v", err)
	}
	required := []string{
		"admin/modules/supplier/presentation_generated.go",
		"admin/modules/supplier/presentation_generated_test.go",
		"admin/modules/supplier/presentation_manifest.generated.json",
		"admin/modules/all/presentation_generated.go",
		"web/antd-v6/src/generated/modules/supplier/presentation.generated.ts",
		"web/antd-v6/src/generated/modules/supplier/presentation.generated.test.ts",
		"web/antd-v6/src/generated/modules/supplier/presentation.adapter.generated.tsx",
		"web/antd-v6/src/generated/presentation-registry.generated.ts",
	}
	for _, path := range required {
		change, ok := findChange(plan.Changes, path)
		if !ok || change.Action != ActionCreate || !change.Managed {
			t.Fatalf("presentation output %s = %#v, found=%t", path, change, ok)
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("dry-run wrote %s: %v", path, statErr)
		}
	}

	projection, canonical, err := normalizedPresentationProjection(module)
	if err != nil || projection == nil {
		t.Fatalf("normalizedPresentationProjection() = %#v, %v", projection, err)
	}
	wantDigest := sha256.Sum256(canonical)
	wantHash := "sha256:" + hex.EncodeToString(wantDigest[:])
	if projection.DefinitionVersion != "2" || projection.DefinitionHash != wantHash {
		t.Fatalf("presentation identity = %q/%q, want 2/%q", projection.DefinitionVersion, projection.DefinitionHash, wantHash)
	}
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation write) error = %v", err)
	}

	snapshotData := readGeneratedTestFile(t, root, "admin/modules/supplier/presentation_manifest.generated.json")
	var snapshot presentationSnapshot
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		t.Fatalf("parse presentation snapshot: %v\n%s", err, snapshotData)
	}
	if !strings.Contains(snapshot.Generated, generatedMarker) {
		t.Fatalf("snapshot ownership marker = %q", snapshot.Generated)
	}
	if snapshot.Manifest.DefinitionHash != wantHash || snapshot.Manifest.PageKey != projection.PageKey {
		t.Fatalf("snapshot identity = %q/%q", snapshot.Manifest.PageKey, snapshot.Manifest.DefinitionHash)
	}
	for _, path := range required {
		data := readGeneratedTestFile(t, root, path)
		contractTest := strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts")
		if !contractTest && !strings.Contains(string(data), wantHash) && !strings.HasSuffix(path, "presentation_manifest.generated.json") {
			t.Fatalf("generated presentation output %s omitted hash %s", path, wantHash)
		}
	}
	backend := string(readGeneratedTestFile(t, root, "admin/modules/supplier/presentation_generated.go"))
	if strings.Contains(backend, "func init()") || strings.Contains(backend, "MustNewRegistry") {
		t.Fatalf("backend projection registered itself implicitly:\n%s", backend)
	}
	adapter := string(readGeneratedTestFile(t, root, "web/antd-v6/src/generated/modules/supplier/presentation.adapter.generated.tsx"))
	for _, requiredBinding := range []string{"boolean-filter", "copyable-code", "date-time", "viewAdapter", "dynamic import"} {
		if !strings.Contains(adapter+string(readGeneratedTestFile(t, root, "web/antd-v6/src/generated/presentation-registry.generated.ts")), requiredBinding) {
			t.Errorf("generated frontend projection omitted %q", requiredBinding)
		}
	}
	if strings.Contains(adapter, "fetch(") || strings.Contains(adapter, "import(") || strings.Contains(adapter, "axios") {
		t.Fatalf("adapter metadata contains executable transport or dynamic import:\n%s", adapter)
	}
	if strings.Contains(adapter, `"enumValues": null`) {
		t.Fatalf("adapter metadata emitted a nullable enum value collection:\n%s", adapter)
	}
	view := buildPresentationViewAdapter(projection)
	for _, field := range view.Fields {
		if field.ValueType != "enum" && field.EnumValues == nil {
			t.Fatalf("view adapter field %s has a nil enum value collection", field.Field)
		}
	}
	business, err := buildPresentationBusinessAdapter(module.Metadata.Name, projection)
	if err != nil {
		t.Fatalf("buildPresentationBusinessAdapter() error = %v", err)
	}
	wantQueryBindings := []presentationBusinessQueryBinding{
		{Field: "code", Parameter: "q", Kind: "keyword"},
		{Field: "country", Parameter: "country", Kind: "filter"},
		{Field: "creditLevel", Parameter: "creditLevel", Kind: "filter"},
		{Field: "enabled", Parameter: "enabled", Kind: "filter"},
	}
	if len(business.DataSources) != 1 || !reflect.DeepEqual(business.DataSources[0].QueryBindings, wantQueryBindings) {
		t.Fatalf("Supplier query bindings = %#v, want %#v", business.DataSources, wantQueryBindings)
	}
	wantActionPermissions := map[string][]string{
		"supplier.create": {"/suppliers", "/suppliers/permissions/create"},
		"supplier.delete": {"/suppliers", "/suppliers/permissions/delete"},
		"supplier.export": {"/suppliers", "/suppliers/permissions/export"},
		"supplier.read":   {"/suppliers", "/suppliers/permissions/read"},
		"supplier.update": {"/suppliers", "/suppliers/permissions/update"},
	}
	wantActionOperations := map[string]string{
		"supplier.create": "create",
		"supplier.delete": "delete",
		"supplier.export": "export",
		"supplier.read":   "get",
		"supplier.update": "update",
	}
	if business.DataSources[0].Operation != "list" {
		t.Errorf("Supplier data source operation = %q, want list", business.DataSources[0].Operation)
	}
	if len(business.Actions) != len(wantActionPermissions) {
		t.Fatalf("Supplier action count = %d, want %d", len(business.Actions), len(wantActionPermissions))
	}
	for _, action := range business.Actions {
		if want, exists := wantActionPermissions[action.ID]; !exists || !reflect.DeepEqual(action.RequiredPermissions, want) {
			t.Errorf("Supplier action %s permissions = %#v, want %#v", action.ID, action.RequiredPermissions, want)
		}
		if want, exists := wantActionOperations[action.ID]; !exists || action.Operation != want {
			t.Errorf("Supplier action %s operation = %q, want %q", action.ID, action.Operation, want)
		}
	}

	second, err := Generate(module, Options{Root: root, Write: true})
	if err != nil {
		t.Fatalf("Generate(presentation second write) error = %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("second presentation generation changed %s: %s", change.Path, change.Action)
		}
	}
	if _, err := Generate(module, Options{Root: root, Check: true}); err != nil {
		t.Fatalf("Generate(presentation check) error = %v", err)
	}
}

func TestGenerateWithoutPresentationEmitsNoModuleCapabilityAndCleansOnlyOwnedPaths(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := generatorTestModule()
	module.SourcePath = ".mss/modules/supplier.yaml"
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(no presentation) error = %v", err)
	}

	obsolete := []string{
		"admin/modules/supplier/presentation_generated.go",
		"admin/modules/supplier/presentation_generated_test.go",
		"admin/modules/supplier/presentation_manifest.generated.json",
		"web/antd-v6/src/generated/modules/supplier/presentation.generated.ts",
		"web/antd-v6/src/generated/modules/supplier/presentation.generated.test.ts",
		"web/antd-v6/src/generated/modules/supplier/presentation.adapter.generated.tsx",
	}
	for _, relative := range obsolete {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", relative, err)
		}
		content := []byte("// Code generated by mss. DO NOT EDIT.\n")
		if filepath.Ext(relative) == ".json" {
			content = []byte("{\n  \"$generated\": \"Code generated by mss. DO NOT EDIT.\"\n}\n")
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", relative, err)
		}
	}
	custom := filepath.Join(root, "web", "antd-v6", "src", "generated", "modules", "supplier", "presentation.custom.ts")
	if err := os.WriteFile(custom, []byte("export const ownedByApplication = true;\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(custom): %v", err)
	}

	plan, err := Generate(module, Options{Root: root})
	if err != nil {
		t.Fatalf("Generate(no presentation cleanup dry-run) error = %v", err)
	}
	for _, relative := range obsolete {
		change, ok := findChange(plan.Changes, relative)
		if !ok || change.Action != ActionDelete {
			t.Fatalf("obsolete presentation output %s = %#v, found=%t", relative, change, ok)
		}
	}
	if _, err := Generate(module, Options{Root: root, Check: true}); err == nil {
		t.Fatal("Generate(no presentation cleanup check) unexpectedly passed")
	}
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(no presentation cleanup write) error = %v", err)
	}
	for _, relative := range obsolete {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obsolete presentation output %s remains: %v", relative, err)
		}
	}
	if data, err := os.ReadFile(custom); err != nil || !strings.Contains(string(data), "ownedByApplication") {
		t.Fatalf("application-owned presentation extension changed: %q, %v", data, err)
	}
	registry := string(readGeneratedTestFile(t, root, "web/antd-v6/src/generated/presentation-registry.generated.ts"))
	if strings.Contains(registry, "supplier.list") {
		t.Fatalf("presentation omission left supplier in the registry:\n%s", registry)
	}
}

func TestGeneratePresentationRejectsDuplicatePageKeyAcrossModules(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := loadPresentationTestModule(t, repositoryRoot)
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(first presentation module) error = %v", err)
	}

	duplicate := *module
	duplicate.Metadata = module.Metadata
	duplicate.Metadata.Name = "vendor"
	duplicate.Spec = module.Spec
	duplicate.Spec.Entity = module.Spec.Entity
	duplicate.Spec.Entity.Table = "biz_vendors"
	duplicate.Spec.Generation = module.Spec.Generation
	duplicate.Spec.Generation.MigrationID = "20260810160100"
	duplicate.Spec.Generation.AuthorizationMigrationID = "20260821093100"
	duplicate.SourcePath = "admin/modules/vendor/module.yaml"
	duplicate.Normalize()
	data, err := duplicate.YAML()
	if err != nil {
		t.Fatalf("duplicate YAML: %v", err)
	}
	path := filepath.Join(root, "admin", "modules", "vendor", "module.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(duplicate): %v", err)
	}
	if err := os.WriteFile(path, append([]byte("# Code generated by mss. DO NOT EDIT.\n"), data...), 0o644); err != nil {
		t.Fatalf("WriteFile(duplicate): %v", err)
	}

	_, err = Generate(module, Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "duplicate-page-key") || !strings.Contains(err.Error(), "page key") {
		t.Fatalf("Generate(duplicate page key) error = %v", err)
	}
}

func TestBuildPresentationQueryBindingsRejectsAmbiguousKeywordControls(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	module := loadPresentationTestModule(t, repositoryRoot)
	projection, _, err := normalizedPresentationProjection(module)
	if err != nil || projection == nil {
		t.Fatalf("normalizedPresentationProjection() = %#v, %v", projection, err)
	}
	projection.DefaultPresentation.Search.Fields = append(
		projection.DefaultPresentation.Search.Fields,
		presentationDefaultField{Field: "name", Component: "input", Order: 50},
	)
	_, err = buildPresentationQueryBindings(projection)
	if err == nil || !strings.Contains(err.Error(), `both bind query parameter "q"`) {
		t.Fatalf("buildPresentationQueryBindings(ambiguous keyword fields) error = %v", err)
	}
}

func TestGeneratePresentationRejectsUserOwnedOutputBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	path := filepath.Join(root, "admin", "modules", "supplier", "presentation_generated.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(conflict): %v", err)
	}
	if err := os.WriteFile(path, []byte("package supplier\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict): %v", err)
	}
	plan, err := Generate(loadPresentationTestModule(t, repositoryRoot), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != "admin/modules/supplier/presentation_generated.go" {
		t.Fatalf("Generate(presentation conflict) plan=%#v error=%v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(root, "admin", "modules", "supplier", "model_generated.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("presentation conflict allowed a partial write: %v", err)
	}
}

func TestGeneratePresentationRejectsMarkerOnlyInUserOwnedBody(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	relative := "admin/modules/supplier/presentation_generated.go"
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(marker-in-body conflict): %v", err)
	}
	content := []byte("package supplier\n\n// Code generated by mss. DO NOT EDIT.\nconst applicationOwned = true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(marker-in-body conflict): %v", err)
	}

	plan, err := Generate(loadPresentationTestModule(t, repositoryRoot), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != relative {
		t.Fatalf("Generate(marker-in-body conflict) plan=%#v error=%v", plan, err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(after, content) {
		t.Fatalf("user-owned marker-in-body file changed: %q, %v", after, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "admin", "modules", "supplier", "model_generated.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("marker-in-body conflict allowed a partial write: %v", statErr)
	}
}

func TestGeneratePresentationCleanupRejectsMarkerOnlyInUserOwnedBody(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	relative := "web/antd-v6/src/generated/modules/supplier/presentation.generated.ts"
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(cleanup marker-in-body conflict): %v", err)
	}
	content := []byte("export const applicationOwned = 'Code generated by mss';\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(cleanup marker-in-body conflict): %v", err)
	}

	plan, err := Generate(generatorTestModule(), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != relative {
		t.Fatalf("Generate(cleanup marker-in-body conflict) plan=%#v error=%v", plan, err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(after, content) {
		t.Fatalf("user-owned cleanup target changed: %q, %v", after, readErr)
	}
}

func TestGeneratePresentationRejectsUserOwnedContractTestBeforeWriting(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	path := filepath.Join(root, "web", "antd-v6", "src", "generated", "modules", "supplier", "presentation.generated.test.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(contract test conflict): %v", err)
	}
	if err := os.WriteFile(path, []byte("export const applicationOwnedTest = true;\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(contract test conflict): %v", err)
	}
	plan, err := Generate(loadPresentationTestModule(t, repositoryRoot), Options{Root: root, Write: true})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != "web/antd-v6/src/generated/modules/supplier/presentation.generated.test.ts" {
		t.Fatalf("Generate(presentation contract test conflict) plan=%#v error=%v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(root, "admin", "modules", "supplier", "model_generated.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("presentation contract test conflict allowed a partial write: %v", err)
	}
}

func TestGeneratePresentationRejectsPartialOutputGroupWithoutMutation(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := loadPresentationTestModule(t, repositoryRoot)
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation baseline write) error = %v", err)
	}

	missingRelative := "web/antd-v6/src/generated/modules/supplier/presentation.generated.ts"
	tamperedRelative := "admin/modules/supplier/presentation_generated.go"
	missingPath := filepath.Join(root, filepath.FromSlash(missingRelative))
	tamperedPath := filepath.Join(root, filepath.FromSlash(tamperedRelative))
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove paired TypeScript definition: %v", err)
	}
	tamperedBefore, err := os.ReadFile(tamperedPath)
	if err != nil {
		t.Fatalf("read paired Go definition: %v", err)
	}
	tamperedBefore = append(tamperedBefore, []byte("\n// managed paired-output drift\n")...)
	if err := os.WriteFile(tamperedPath, tamperedBefore, 0o644); err != nil {
		t.Fatalf("tamper paired Go definition: %v", err)
	}

	plan, err := Generate(module, Options{Root: root, Check: true})
	var groupConflict *OutputGroupConflictError
	if !errors.As(err, &groupConflict) {
		t.Fatalf("Generate(presentation partial-output check) plan=%#v error=%v", plan, err)
	}
	if !slicesContains(groupConflict.Missing, missingRelative) || !slicesContains(groupConflict.Present, tamperedRelative) {
		t.Fatalf("partial-output conflict = %#v", groupConflict)
	}
	if _, err := os.Stat(missingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only partial-output check recreated missing TypeScript definition: %v", err)
	}
	if tamperedAfter, err := os.ReadFile(tamperedPath); err != nil {
		t.Fatalf("read partial Go definition after check: %v", err)
	} else if !reflect.DeepEqual(tamperedAfter, tamperedBefore) {
		t.Fatalf("read-only partial-output check changed tampered Go definition")
	}

	plan, err = Generate(module, Options{Root: root, Write: true})
	if !errors.As(err, &groupConflict) {
		t.Fatalf("Generate(presentation partial-output write) plan=%#v error=%v", plan, err)
	}
	if _, err := os.Stat(missingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial-output write recreated missing TypeScript definition: %v", err)
	}
	if tamperedAfter, err := os.ReadFile(tamperedPath); err != nil {
		t.Fatalf("read partial Go definition after write rejection: %v", err)
	} else if !reflect.DeepEqual(tamperedAfter, tamperedBefore) {
		t.Fatalf("partial-output write rejection changed tampered Go definition")
	}

	layout, err := resolveTargetLayout(nil)
	if err != nil {
		t.Fatalf("resolve monorepo target layout: %v", err)
	}
	for _, relative := range presentationOutputGroupPaths(module.Metadata.Name, layout) {
		err := os.Remove(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove presentation output %s: %v", relative, err)
		}
	}
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation all-missing output group) error = %v", err)
	}
	if _, err := Generate(module, Options{Root: root, Check: true}); err != nil {
		t.Fatalf("Generate(presentation all-missing post-write check) error = %v", err)
	}
}

func TestGeneratePresentationRejectsPartialRegistryPairWithoutMutation(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(repositoryRoot, "templates", "module"), filepath.Join(root, "templates", "module"))
	module := loadPresentationTestModule(t, repositoryRoot)
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation baseline write) error = %v", err)
	}
	layout, err := resolveTargetLayout(nil)
	if err != nil {
		t.Fatalf("resolve monorepo target layout: %v", err)
	}
	pair := presentationRegistryOutputPairPaths(layout)
	backendRelative, frontendRelative := pair[0], pair[1]
	backendPath := filepath.Join(root, filepath.FromSlash(backendRelative))
	frontendPath := filepath.Join(root, filepath.FromSlash(frontendRelative))
	if err := os.Remove(frontendPath); err != nil {
		t.Fatalf("remove frontend presentation registry: %v", err)
	}
	backendBefore, err := os.ReadFile(backendPath)
	if err != nil {
		t.Fatalf("read backend presentation registry: %v", err)
	}
	backendBefore = append(backendBefore, []byte("\n// managed registry pair drift\n")...)
	if err := os.WriteFile(backendPath, backendBefore, 0o644); err != nil {
		t.Fatalf("tamper backend presentation registry: %v", err)
	}

	for _, options := range []Options{
		{Root: root, Check: true},
		{Root: root, Write: true},
	} {
		plan, err := Generate(module, options)
		var groupConflict *OutputGroupConflictError
		if !errors.As(err, &groupConflict) {
			t.Fatalf("Generate(presentation partial registry pair, check=%t/write=%t) plan=%#v error=%v", options.Check, options.Write, plan, err)
		}
		if groupConflict.Group != "presentation-registry" ||
			!slicesContains(groupConflict.Present, backendRelative) ||
			!slicesContains(groupConflict.Missing, frontendRelative) {
			t.Fatalf("partial registry pair conflict = %#v", groupConflict)
		}
		if _, err := os.Stat(frontendPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial registry pair recreated missing frontend registry: %v", err)
		}
		if backendAfter, err := os.ReadFile(backendPath); err != nil || !bytes.Equal(backendAfter, backendBefore) {
			t.Fatalf("partial registry pair changed backend registry: %q, %v", backendAfter, err)
		}
	}

	if err := os.Remove(backendPath); err != nil {
		t.Fatalf("remove backend presentation registry: %v", err)
	}
	if _, err := Generate(module, Options{Root: root, Write: true}); err != nil {
		t.Fatalf("Generate(presentation all-missing registry pair) error = %v", err)
	}
	if _, err := Generate(module, Options{Root: root, Check: true}); err != nil {
		t.Fatalf("Generate(presentation all-missing registry pair post-write check) error = %v", err)
	}
}

func TestGeneratePresentationUsesConfinedThinHostPaths(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	root := t.TempDir()
	module := loadPresentationTestModule(t, repositoryRoot)
	document := thinHostProjectDocument()
	document.Spec.Distribution.Frontend.Package = "@acme/admin-web"
	if _, err := Generate(module, Options{Root: root, Write: true, Project: &document}); err != nil {
		t.Fatalf("Generate(thin-host presentation) error = %v", err)
	}
	for _, required := range []string{
		"internal/modules/supplier/presentation_generated.go",
		"internal/modules/supplier/presentation_generated_test.go",
		"internal/modules/supplier/presentation_manifest.generated.json",
		"internal/modules/all/presentation_generated.go",
		"web/src/generated/modules/supplier/presentation.generated.ts",
		"web/src/generated/modules/supplier/presentation.generated.test.ts",
		"web/src/generated/modules/supplier/presentation.adapter.generated.tsx",
		"web/src/generated/presentation-registry.generated.ts",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(required))); err != nil {
			t.Errorf("thin-host presentation output %s: %v", required, err)
		}
	}
	contractTest := string(readGeneratedTestFile(
		t,
		root,
		"web/src/generated/modules/supplier/presentation.generated.test.ts",
	))
	for _, required := range []string{
		`from '@acme/admin-web/runtime/presentation'`,
		"canonicalizeCapabilityContract",
		"createHash('sha256')",
	} {
		if !strings.Contains(contractTest, required) {
			t.Errorf("thin-host generated presentation contract test omitted %q:\n%s", required, contractTest)
		}
	}
	if strings.Contains(contractTest, "../../../shared/") {
		t.Fatalf("thin-host generated presentation contract test imports Foundation-only source:\n%s", contractTest)
	}
	if entries, err := os.ReadDir(filepath.Dir(root)); err != nil {
		t.Fatalf("ReadDir(thin-host parent): %v", err)
	} else {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "mss-presentation-") {
				t.Fatalf("presentation generation escaped target root: %s", entry.Name())
			}
		}
	}
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func loadPresentationTestModule(t *testing.T, repositoryRoot string) *spec.Module {
	t.Helper()
	module, err := spec.LoadModule(filepath.Join(repositoryRoot, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("LoadModule(example supplier): %v", err)
	}
	if module.Spec.Presentation == nil {
		module.Spec.Presentation = supplierPresentationTestSource()
		module.Normalize()
		if issues := module.Validate(); len(issues) > 0 {
			t.Fatalf("synthetic Supplier presentation is invalid: %#v", issues)
		}
	}
	module.SourcePath = ".mss/modules/example-supplier.yaml"
	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(example supplier): %v", err)
	}
	if manifest == nil {
		t.Fatal("example supplier does not declare presentation")
	}
	return module
}

func supplierPresentationTestSource() *spec.PresentationSource {
	width180, width240, span12 := 180, 240, 12
	field := func(name, component string, order int) spec.PresentationFieldSource {
		return spec.PresentationFieldSource{Field: name, Component: component, Order: order}
	}
	formField := func(name, component string, order int) spec.PresentationFieldSource {
		value := field(name, component, order)
		value.Span = &span12
		return value
	}
	listCode := field("code", "text", 10)
	listCode.Width = &width180
	listName := field("name", "text", 20)
	listName.Width = &width240
	confirm := spec.PresentationLocalizedText{ZhCN: "确认删除该供应商？", EnUS: "Delete this supplier?"}
	return &spec.PresentationSource{
		PageKey: "supplier.list", DefinitionVersion: "2",
		Title:      spec.PresentationLocalizedText{ZhCN: "供应商管理", EnUS: "Suppliers"},
		DataSource: "list",
		List: spec.PresentationListSource{
			Density: "middle", PageSize: 20,
			DefaultSort: []spec.PresentationSortSource{{Field: "code", Direction: "asc"}},
			Fields: []spec.PresentationFieldSource{
				listCode, listName, field("country", "text", 30), field("contactName", "text", 40),
				field("contactEmail", "text", 50), field("creditLevel", "tag", 60), field("enabled", "boolean", 70),
			},
		},
		Search: spec.PresentationSearchSource{
			CollapsedByDefault: true,
			Fields: []spec.PresentationFieldSource{
				field("code", "input", 10), field("name", "input", 20), field("country", "input", 30),
				field("contactName", "input", 40), field("creditLevel", "select", 50), field("enabled", "boolean-filter", 60),
			},
		},
		Form: spec.PresentationFormSource{
			Columns: 2,
			Fields: []spec.PresentationFieldSource{
				formField("code", "input", 10), formField("name", "input", 20), formField("country", "input", 30),
				formField("contactName", "input", 40), formField("contactEmail", "email-input", 50),
				formField("creditLevel", "select", 60), formField("enabled", "switch", 70),
			},
		},
		Detail: spec.PresentationDetailSource{
			Columns: 2,
			Fields: []spec.PresentationFieldSource{
				formField("id", "copyable-code", 5), formField("code", "text", 10), formField("name", "text", 20),
				formField("country", "text", 30), formField("contactName", "text", 40), formField("contactEmail", "text", 50),
				formField("creditLevel", "tag", 60), formField("enabled", "boolean", 70),
				formField("createdAt", "date-time", 80), formField("updatedAt", "date-time", 90),
			},
		},
		Actions: []spec.PresentationActionSource{
			{Action: "create", Placement: "toolbar", Order: 10},
			{Action: "export", Placement: "toolbar", Order: 20},
			{Action: "read", Placement: "row", Order: 30},
			{Action: "update", Placement: "row", Order: 40},
			{Action: "delete", Placement: "row", Order: 50, Confirm: &confirm},
		},
	}
}

func readGeneratedTestFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read generated %s: %v", relative, err)
	}
	return data
}

func findChange(changes []Change, path string) (Change, bool) {
	for _, change := range changes {
		if change.Path == path {
			return change, true
		}
	}
	return Change{}, false
}
