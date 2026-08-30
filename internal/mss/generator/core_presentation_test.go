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

func TestCorePresentationTrackedOutputsMatchCanonicalInventory(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	sourceMatches, err := filepath.Glob(filepath.Join(repositoryRoot, ".mss", "core-pages", "*.yaml"))
	if err != nil {
		t.Fatalf("discover core sources: %v", err)
	}
	expectedSources := make([]string, 0, len(sourceMatches))
	for _, source := range sourceMatches {
		relative, relativeErr := filepath.Rel(repositoryRoot, source)
		if relativeErr != nil {
			t.Fatalf("relativize core source %s: %v", source, relativeErr)
		}
		expectedSources = append(expectedSources, filepath.ToSlash(relative))
	}
	if len(expectedSources) != 14 {
		t.Fatalf("core source count = %d, want 14", len(expectedSources))
	}
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
		if output.source != strings.Join(expectedSources, ",") {
			t.Fatalf("core output %s source = %q", output.path, output.source)
		}
	}

	var snapshot corePresentationSnapshot
	snapshotData := readGeneratedTestFile(t, repositoryRoot, "admin/presentation/core/manifest.generated.json")
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		t.Fatalf("parse tracked core snapshot: %v", err)
	}
	if !slices.Equal(snapshot.Sources, expectedSources) || len(snapshot.Manifests) != len(expectedSources) {
		t.Fatalf("core snapshot provenance = %#v", snapshot)
	}
	backend := string(readGeneratedTestFile(t, repositoryRoot, "admin/presentation/core/definitions_generated.go"))
	frontend := string(readGeneratedTestFile(t, repositoryRoot, "web/antd-v6/src/generated/core-presentation-registry.generated.ts"))
	wantPageKeys := []string{
		"department.list", "language.list", "log.audit", "log.login", "log.runtime", "menu.list", "notice.list",
		"online-session.list", "option.list", "post.list", "role.list", "system-config.list", "task.list", "user.list",
	}
	wantTitles := map[string][2]string{
		"department.list":     {"部门管理", "Departments"},
		"language.list":       {"语言管理", "Languages"},
		"log.audit":           {"审计日志", "Audit logs"},
		"log.login":           {"登录日志", "Login logs"},
		"log.runtime":         {"运行日志", "Runtime logs"},
		"menu.list":           {"菜单管理", "Menus"},
		"notice.list":         {"通知中心", "Notices"},
		"online-session.list": {"在线会话", "Online sessions"},
		"option.list":         {"选项字典管理", "Option dictionaries"},
		"post.list":           {"岗位管理", "Posts"},
		"role.list":           {"角色管理", "Roles"},
		"system-config.list":  {"系统配置", "System configuration"},
		"task.list":           {"任务调度", "Task scheduler"},
		"user.list":           {"用户管理", "Users"},
	}
	for index := range snapshot.Manifests {
		projection := &snapshot.Manifests[index]
		if projection.PageKey != wantPageKeys[index] {
			t.Fatalf("manifest page %d = %q, want %q", index, projection.PageKey, wantPageKeys[index])
		}
		wantTitle := wantTitles[projection.PageKey]
		if projection.DefaultPresentation.Title.ZhCN == nil || *projection.DefaultPresentation.Title.ZhCN != wantTitle[0] ||
			projection.DefaultPresentation.Title.EnUS == nil || *projection.DefaultPresentation.Title.EnUS != wantTitle[1] {
			t.Errorf("core projection %s title = %#v, want zh-CN=%q en-US=%q", projection.PageKey, projection.DefaultPresentation.Title, wantTitle[0], wantTitle[1])
		}
		canonical, canonicalErr := canonicalPresentationProjection(projection)
		if canonicalErr != nil {
			t.Fatalf("canonicalPresentationProjection(%s) error = %v", projection.PageKey, canonicalErr)
		}
		digest := sha256.Sum256(canonical)
		wantHash := "sha256:" + hex.EncodeToString(digest[:])
		if projection.DefinitionHash != wantHash {
			t.Fatalf("snapshot %s hash = %q, want %q", projection.PageKey, projection.DefinitionHash, wantHash)
		}
		for path, content := range map[string]string{"backend": backend, "frontend": frontend} {
			if !strings.Contains(content, wantHash) || !strings.Contains(content, projection.PageKey) {
				t.Errorf("%s core projection omitted %s or its hash", path, projection.PageKey)
			}
		}
		if len(projection.Actions) != 0 || len(projection.DefaultPresentation.Form.Fields) != 0 ||
			len(projection.DefaultPresentation.Detail.Fields) != 0 || len(projection.DefaultPresentation.Actions) != 0 {
			t.Errorf("core projection %s exposed non-list capability", projection.PageKey)
		}
	}
	for _, contract := range []string{
		"export const corePresentationInventory", "export const corePresentationRegistry", `"user.list"`, `"system-config.list"`,
		`"user-identity"`, `"user-role"`, `"user-organization"`, `"status-tag"`, `"status-filter"`,
		`"maxSortFields": 0`, `"defaultSort": []`, `"requiredPermissions": [`, `"/users"`,
	} {
		if !strings.Contains(frontend, contract) {
			t.Errorf("generated frontend core registry omitted %q", contract)
		}
	}
	lower := strings.ToLower(frontend)
	for _, forbidden := range []string{
		"password", "confirmpassword", "departmentid", "postid", `"root"`,
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

	coreSources, err := os.ReadDir(filepath.Join(root, ".mss", "core-pages"))
	if err != nil {
		t.Fatalf("read temporary core sources: %v", err)
	}
	for _, source := range coreSources {
		if source.IsDir() || filepath.Ext(source.Name()) != ".yaml" {
			continue
		}
		if removeErr := os.Remove(filepath.Join(root, ".mss", "core-pages", source.Name())); removeErr != nil {
			t.Fatalf("remove temporary core source %s: %v", source.Name(), removeErr)
		}
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
