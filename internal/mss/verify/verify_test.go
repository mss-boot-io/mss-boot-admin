package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestReleaseEvidenceOptionsFailClosed(t *testing.T) {
	commit := strings.Repeat("a", 40)
	tests := []struct {
		name    string
		options Options
		wantErr string
	}{
		{name: "valid", options: Options{Mode: ModeAll, ReleaseEvidence: true, ExpectedCommit: commit}},
		{name: "requires all", options: Options{Mode: ModeChanged, ReleaseEvidence: true, ExpectedCommit: commit}, wantErr: "requires --all"},
		{name: "rejects plan", options: Options{Mode: ModeAll, PlanOnly: true, ReleaseEvidence: true, ExpectedCommit: commit}, wantErr: "cannot be combined with --plan"},
		{name: "requires expected commit", options: Options{Mode: ModeAll, ReleaseEvidence: true}, wantErr: "full 40-character lowercase"},
		{name: "rejects abbreviated commit", options: Options{Mode: ModeAll, ReleaseEvidence: true, ExpectedCommit: "abcdef1"}, wantErr: "full 40-character lowercase"},
		{name: "rejects uppercase commit", options: Options{Mode: ModeAll, ReleaseEvidence: true, ExpectedCommit: strings.Repeat("A", 40)}, wantErr: "full 40-character lowercase"},
		{name: "expected commit requires evidence", options: Options{Mode: ModeAll, ExpectedCommit: commit}, wantErr: "requires --release-evidence"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateReleaseEvidenceOptions(test.options)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateReleaseEvidenceOptions() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateReleaseEvidenceOptions() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateReleaseEvidenceSnapshotFailClosed(t *testing.T) {
	commit := strings.Repeat("a", 40)
	tests := []struct {
		name     string
		snapshot releaseEvidenceSnapshot
		wantErr  string
	}{
		{name: "exact and clean", snapshot: releaseEvidenceSnapshot{Commit: commit, TrackedClean: true}},
		{name: "head mismatch", snapshot: releaseEvidenceSnapshot{Commit: strings.Repeat("b", 40), TrackedClean: true}, wantErr: "expected " + commit},
		{name: "tracked dirty", snapshot: releaseEvidenceSnapshot{Commit: commit, TrackedClean: false}, wantErr: "tracked worktree is dirty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateReleaseEvidenceSnapshot("after verification", test.snapshot, commit)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateReleaseEvidenceSnapshot() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateReleaseEvidenceSnapshot() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestInspectReleaseEvidenceChecksEveryTrackedPathAndIgnoresUntrackedOutputs(t *testing.T) {
	root, commit := initializeEvidenceRepository(t)

	snapshot, err := inspectReleaseEvidence(root)
	if err != nil {
		t.Fatalf("inspectReleaseEvidence(clean) error = %v", err)
	}
	if snapshot.Commit != commit || !snapshot.TrackedClean {
		t.Fatalf("clean snapshot = %#v, want commit %s and clean", snapshot, commit)
	}

	if err := os.WriteFile(filepath.Join(root, ".mss", "reports", "verify.json"), []byte("generated report\n"), 0o644); err != nil {
		t.Fatalf("write untracked report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	snapshot, err = inspectReleaseEvidence(root)
	if err != nil {
		t.Fatalf("inspectReleaseEvidence(untracked outputs) error = %v", err)
	}
	if !snapshot.TrackedClean {
		t.Fatalf("untracked outputs made tracked snapshot dirty: %#v", snapshot)
	}

	if err := os.WriteFile(filepath.Join(root, ".mss", "reports", "tracked.txt"), []byte("updated tracked report\n"), 0o644); err != nil {
		t.Fatalf("update tracked report: %v", err)
	}
	snapshot, err = inspectReleaseEvidence(root)
	if err != nil {
		t.Fatalf("inspectReleaseEvidence(tracked report dirty) error = %v", err)
	}
	if snapshot.TrackedClean {
		t.Fatalf("tracked report change was accepted as clean: %#v", snapshot)
	}
	if err := os.WriteFile(filepath.Join(root, ".mss", "reports", "tracked.txt"), []byte("initial report\n"), 0o644); err != nil {
		t.Fatalf("restore tracked report: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("update tracked file: %v", err)
	}
	snapshot, err = inspectReleaseEvidence(root)
	if err != nil {
		t.Fatalf("inspectReleaseEvidence(dirty) error = %v", err)
	}
	if snapshot.TrackedClean {
		t.Fatalf("tracked change was accepted as clean: %#v", snapshot)
	}
	if _, err := runGit(root, "add", "tracked.txt"); err != nil {
		t.Fatalf("stage tracked file: %v", err)
	}
	snapshot, err = inspectReleaseEvidence(root)
	if err != nil {
		t.Fatalf("inspectReleaseEvidence(staged) error = %v", err)
	}
	if snapshot.TrackedClean {
		t.Fatalf("staged tracked change was accepted as clean: %#v", snapshot)
	}
}

func TestInspectReleaseEvidenceRejectsHiddenIndexFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		flag string
	}{
		{name: "assume unchanged", flag: "--assume-unchanged"},
		{name: "skip worktree", flag: "--skip-worktree"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, _ := initializeEvidenceRepository(t)
			if _, err := runGit(root, "update-index", test.flag, "tracked.txt"); err != nil {
				t.Fatalf("git update-index %s: %v", test.flag, err)
			}
			if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("hidden change\n"), 0o644); err != nil {
				t.Fatalf("update hidden tracked file: %v", err)
			}
			_, err := inspectReleaseEvidence(root)
			if err == nil || !strings.Contains(err.Error(), "assume-unchanged or skip-worktree") {
				t.Fatalf("inspectReleaseEvidence() error = %v, want hidden-index rejection", err)
			}
			if !strings.Contains(err.Error(), "tracked.txt") {
				t.Fatalf("inspectReleaseEvidence() error = %v, want affected path", err)
			}
		})
	}
}

func TestRunReleaseEvidenceRejectsDirtyStartAndWritesBoundReport(t *testing.T) {
	root, commit := initializeEvidenceRepository(t)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("update tracked file: %v", err)
	}
	ctx := &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{Name: "release-evidence-test"},
			Spec:     project.ProjectSpec{RepositoryLayout: map[string]string{"kind": "foundation"}},
		},
	}
	report, err := Run(t.Context(), ctx, Options{
		Mode:            ModeAll,
		ReleaseEvidence: true,
		ExpectedCommit:  commit,
	})
	if err == nil || !strings.Contains(err.Error(), "tracked worktree is dirty") {
		t.Fatalf("Run() error = %v, want dirty tracked worktree rejection", err)
	}
	if report.Success || !report.EvidenceMode || report.Commit != commit {
		t.Fatalf("failed release evidence report = %#v", report)
	}
	if report.TrackedCleanBefore == nil || *report.TrackedCleanBefore {
		t.Fatalf("trackedCleanBefore = %v, want explicit false", report.TrackedCleanBefore)
	}
	if report.TrackedCleanAfter == nil || *report.TrackedCleanAfter {
		t.Fatalf("trackedCleanAfter = %v, want explicit false", report.TrackedCleanAfter)
	}

	data, readErr := os.ReadFile(filepath.Join(root, ".mss", "reports", "verify.json"))
	if readErr != nil {
		t.Fatalf("read verify.json: %v", readErr)
	}
	var decoded map[string]any
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("decode verify.json: %v", unmarshalErr)
	}
	for key, want := range map[string]any{
		"evidenceMode":       true,
		"commit":             commit,
		"trackedCleanBefore": false,
		"trackedCleanAfter":  false,
	} {
		if got := decoded[key]; got != want {
			t.Errorf("verify.json %s = %#v, want %#v", key, got, want)
		}
	}
	markdown := report.Markdown()
	for _, want := range []string{
		"- Evidence mode: `true`",
		"- Commit: `" + commit + "`",
		"- Tracked clean before: `false`",
		"- Tracked clean after: `false`",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("Markdown() missing %q:\n%s", want, markdown)
		}
	}
}

func initializeEvidenceRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if _, err := runGit(root, "init", "--quiet"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "MSS Evidence Test"},
		{"config", "user.email", "evidence@example.invalid"},
		{"config", "commit.gpgsign", "false"},
	} {
		if _, err := runGit(root, args...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".mss", "reports"), 0o755); err != nil {
		t.Fatalf("create reports directory: %v", err)
	}
	for path, content := range map[string]string{
		"tracked.txt":              "tracked\n",
		".mss/reports/tracked.txt": "initial report\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if _, err := runGit(root, "add", "--all"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGit(root, "commit", "--quiet", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	output, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return root, strings.TrimSpace(string(output))
}

func TestFoundationModeAllOwnsLocalReleaseQualification(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{
		Root: root,
		Project: project.ProjectDocument{Spec: project.ProjectSpec{
			RepositoryLayout: map[string]string{
				"kind":    "foundation",
				"backend": "admin",
				"modules": "admin/modules",
			},
			Frontend: project.FrontendSpec{Applications: []project.FrontendApplicationSpec{
				{ID: "antd-v6", Path: "web/antd-v6"},
			}},
		}},
	}

	plan, err := PlanChecks(ctx, Options{Mode: ModeAll})
	if err != nil {
		t.Fatalf("PlanChecks(Foundation all): %v", err)
	}
	wantIDs := []string{
		"admin-distribution-external-consumer",
		"agent-build",
		"agent-doctor-strict",
		"agent-release-test",
		"agent-skills-validation",
		"backend-doctor-strict",
		"backend-release-qualification",
		"docs-build",
		"foundation-compatibility",
		"framework-release-qualification",
		"frontend-qualification",
		"git-diff-check",
		"git-worktree-check",
		"presentation-thin-host-contract",
		"release-contract-test",
	}
	gotIDs := make([]string, 0, len(plan.Checks))
	for _, check := range plan.Checks {
		gotIDs = append(gotIDs, check.ID)
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("Foundation all check IDs = %q, want %q", gotIDs, wantIDs)
	}
}

func TestAdminDistributionExternalConsumerUsesRepositoryExternalQualification(t *testing.T) {
	root := t.TempDir()
	spec := adminDistributionExternalConsumer(root, false)
	if got, want := spec.Args, []string{
		"bash",
		"tools/compatibility/test-thin-host-external-consumer.sh",
		"--foundation-root",
		root,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("external consumer arguments = %q, want %q", got, want)
	}
	if spec.Directory != root || spec.Environment["CI"] != "true" {
		t.Fatalf("external consumer contract = %#v", spec)
	}
	if got, want := spec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("external consumer GOWORK = %q, want %q", got, want)
	}
	if spec.Timeout != 90*time.Minute {
		t.Fatalf("external consumer timeout = %s, want 90m", spec.Timeout)
	}
	if _, ok := spec.Environment["MSS_PERSIST_EVIDENCE"]; ok {
		t.Fatalf("ordinary full verification must not retain external evidence: %#v", spec.Environment)
	}
	persistent := adminDistributionExternalConsumer(root, true)
	if persistent.Environment["MSS_PERSIST_EVIDENCE"] != "1" {
		t.Fatalf("release evidence must retain external reports: %#v", persistent.Environment)
	}
}

func TestTruncatePreservesCommandOutputHeadAndTail(t *testing.T) {
	value := "HEAD-" + strings.Repeat("x", 200) + "-TAIL"
	got := truncate(value, 80)
	if len(got) != 80 {
		t.Fatalf("truncated output length = %d, want 80", len(got))
	}
	if !strings.HasPrefix(got, "HEAD-") || !strings.HasSuffix(got, "-TAIL") {
		t.Fatalf("truncated output did not preserve both boundaries: %q", got)
	}
	if !strings.Contains(got, "preserving head and tail") {
		t.Fatalf("truncated output did not explain truncation: %q", got)
	}
}

func TestReleaseWorkflowContractSensitivity(t *testing.T) {
	for _, path := range []string{
		".github/workflows/release.yml",
		".github/workflows/container.yml",
		".mss/release-policy.yaml",
		".mss/release-qualification.json",
		".mss/commands.yaml",
		".agents/skills/mss-release/SKILL.md",
		"Makefile",
		"tools/release/test_root_release_workflow.py",
		"tools/verification/run-frontend-e2e.sh",
	} {
		if !releaseWorkflowContractSensitive(path) {
			t.Errorf("release workflow contract did not select %q", path)
		}
	}
	for _, path := range []string{
		"admin/presentation/validation.go",
		"docs/docs/admin/release-verification-checklist.md",
		"web/antd-v6/src/app.tsx",
	} {
		if releaseWorkflowContractSensitive(path) {
			t.Errorf("release workflow contract unexpectedly selected %q", path)
		}
	}
}

func TestLocalQualificationScriptSensitivity(t *testing.T) {
	if !foundationCompatibilitySensitive("tools/compatibility/test-standalone-mss-consumer.sh") {
		t.Fatal("standalone Foundation gate did not select next-Foundation qualification")
	}
	if foundationCompatibilitySensitive("tools/compatibility/test-admin-external-consumer.sh") {
		t.Fatal("Admin consumer unexpectedly selected next-Foundation qualification")
	}
	for _, path := range []string{
		".mss/core-pages/user-list.yaml",
		"admin/cmd/migrate/migration/system/20260824120000_presentation_profiles.go",
		"admin/cmd/migrate/migration/system/future_presentation_contract.go",
		"admin/router/security_contract.go",
		"tools/verification/run-frontend-e2e.sh",
		"web/antd-v6/package.json",
		"web/antd-v6/playwright.config.ts",
		"web/antd-v6/pnpm-lock.yaml",
		"web/antd-v6/scripts/start-e2e-backend.sh",
		"web/antd-v6/src/generated/core-presentation-registry.generated.ts",
		"web/antd-v6/e2e/presentation.spec.ts",
		"web/antd-v6/e2e/support/session.ts",
		"web/antd-v6/src/shared/presentation/runtime.ts",
		"web/antd-v6/src/shared/presentation/table.test.ts",
		"admin/config/application-e2e.yml",
		"admin/config/presentation.go",
		"admin/apis/presentation_profile.go",
		"admin/dto/presentation_profile.go",
		"admin/models/presentation_profile.go",
		"admin/presentation/validation.go",
		"admin/service/presentation_profile.go",
	} {
		if !frontendQualificationSensitive(path) {
			t.Errorf("frontend browser qualification did not select %q", path)
		}
	}
	for _, path := range []string{
		"admin/cmd/migrate/migration/system/20260801120000_users.go",
		"tools/verification/other.sh",
		"web/antd-v6/src/shared/api/client.ts",
		"admin/service/user.go",
		"docs/docs/admin/presentation.md",
	} {
		if frontendQualificationSensitive(path) {
			t.Errorf("unrelated path unexpectedly selected frontend qualification: %q", path)
		}
	}
}

func TestFoundationCompatibilityUsesCanonicalLocalNextFoundationGate(t *testing.T) {
	root := t.TempDir()
	spec := foundationCompatibility(root)
	if spec.Directory != root {
		t.Fatalf("Foundation compatibility directory = %q, want %q", spec.Directory, root)
	}
	if want := []string{"make", "compatibility-foundation-next"}; !reflect.DeepEqual(spec.Args, want) {
		t.Fatalf("Foundation compatibility arguments = %q, want %q", spec.Args, want)
	}
	if spec.Environment["CI"] != "true" || spec.Timeout != 90*time.Minute {
		t.Fatalf("Foundation compatibility contract = %#v", spec)
	}
}

func TestWriteReportsUsesPrivateFilePermissions(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{Root: root}
	report := Report{Project: "private-report", Root: root, Success: true}
	if err := WriteReports(ctx, report); err != nil {
		t.Fatalf("WriteReports() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for _, name := range []string{"verify.json", "verify.md"} {
		info, err := os.Stat(filepath.Join(root, ".mss", "reports", name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, got)
		}
	}
}

func TestToolingTestUsesConsolidatedAdminRuntimePathWhenOptionalRuntimeIsAbsent(t *testing.T) {
	root := t.TempDir()
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
	}

	spec := toolingTest(root)
	if got := spec.Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tooling test arguments = %q, want %q", got, want)
	}
	if got, want := spec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("tooling test GOWORK = %q, want %q", got, want)
	}
	if got := spec.Environment["GOFLAGS"]; got != "-mod=readonly" {
		t.Fatalf("tooling test GOFLAGS = %q, want -mod=readonly", got)
	}
}

func TestToolingTestIncludesExistingOptionalModuleRuntime(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "modules", "runtime"), 0o755); err != nil {
		t.Fatalf("create optional module runtime: %v", err)
	}
	want := []string{
		"go",
		"test",
		"./internal/mss/...",
		"./cmd/mss/...",
		"./admin/modules/runtime/...",
		"./modules/runtime/...",
	}

	if got := toolingTest(root).Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tooling test arguments = %q, want %q", got, want)
	}
}

func TestBackendChecksUseTheFoundationWorkspaceBeforePublicDependencyQualification(t *testing.T) {
	root := t.TempDir()
	adminDir := filepath.Join(root, "admin")

	testSpec := backendTest(root)
	if testSpec.Directory != adminDir {
		t.Fatalf("backend test directory = %q, want %q", testSpec.Directory, adminDir)
	}
	if want := []string{
		"go", "test",
		"-coverprofile=" + filepath.Join(root, ".mss", "reports", "admin-coverage.out"),
		"./...",
	}; !reflect.DeepEqual(testSpec.Args, want) {
		t.Fatalf("backend test arguments = %q, want %q", testSpec.Args, want)
	}
	if got, want := testSpec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("backend test GOWORK = %q, want %q", got, want)
	}
	if testSpec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("backend test environment = %#v", testSpec.Environment)
	}

	buildSpec := backendBuild(root)
	if buildSpec.Directory != adminDir {
		t.Fatalf("backend build directory = %q, want %q", buildSpec.Directory, adminDir)
	}
	if want := []string{"go", "build", "./..."}; !reflect.DeepEqual(buildSpec.Args, want) {
		t.Fatalf("backend build arguments = %q, want %q", buildSpec.Args, want)
	}
	if buildSpec.Environment["GOWORK"] != filepath.Join(root, "go.work") ||
		buildSpec.Environment["GOFLAGS"] != "-mod=readonly" || buildSpec.Environment["CGO_ENABLED"] != "0" {
		t.Fatalf("backend build environment = %#v", buildSpec.Environment)
	}

	qualification := backendReleaseQualification(root)
	if got, want := qualification.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("backend release qualification GOWORK = %q, want %q", got, want)
	}
	if qualification.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("backend release qualification environment = %#v", qualification.Environment)
	}

	frontend := frontendQualification(root)
	if got, want := frontend.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("frontend qualification GOWORK = %q, want %q", got, want)
	}
	if frontend.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("frontend qualification environment = %#v", frontend.Environment)
	}
}

func TestFocusedFoundationModulePinsFoundationWorkspace(t *testing.T) {
	root := t.TempDir()
	ctx := &project.Context{
		Root: root,
		Project: project.ProjectDocument{Spec: project.ProjectSpec{RepositoryLayout: map[string]string{
			"kind":    "foundation",
			"backend": "admin",
			"modules": "admin/modules",
		}}},
	}

	spec, _, err := focusedModuleTest(ctx, "supplier")
	if err != nil {
		t.Fatalf("focusedModuleTest() error = %v", err)
	}
	if got, want := spec.Environment["GOWORK"], filepath.Join(root, "go.work"); got != want {
		t.Fatalf("focused Foundation GOWORK = %q, want %q", got, want)
	}
	if spec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("focused Foundation environment = %#v", spec.Environment)
	}
}

func TestFrameworkCheckIsIndependentAndReadOnly(t *testing.T) {
	spec := frameworkTest(t.TempDir())
	if spec.Environment["GOWORK"] != "off" || spec.Environment["GOFLAGS"] != "-mod=readonly" {
		t.Fatalf("framework test environment = %#v", spec.Environment)
	}
}

func TestPresentationThinHostContractUsesRepositoryExternalConsumers(t *testing.T) {
	root := t.TempDir()
	spec := presentationThinHostContract(root)
	if got, want := spec.Directory, root; got != want {
		t.Fatalf("presentation Thin Host directory = %q, want %q", got, want)
	}
	if got, want := spec.Args, []string{"bash", "tools/compatibility/test-presentation-thin-host-contract.sh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation Thin Host arguments = %q, want %q", got, want)
	}
	if spec.Environment["CI"] != "true" {
		t.Fatalf("presentation Thin Host environment = %#v", spec.Environment)
	}
}

func TestPresentationThinHostContractSensitivity(t *testing.T) {
	for _, path := range []string{
		".mss/core-pages/user-list.yaml",
		".mss/modules/example-supplier.yaml",
		"admin/presentation/core/definitions_generated.go",
		"admin/modules/supplier/presentation_manifest.generated.json",
		"cmd/mss/main.go",
		"internal/mss/generator/core_presentation.go",
		"internal/mss/spec/admin_presentation_inventory.go",
		"templates/application/.mss/project.yaml",
		"templates/module/frontend/page.tsx.tmpl",
		"tools/compatibility/test-presentation-thin-host-contract.sh",
		"web/antd-v6/package.json",
		"web/antd-v6/src/generated/core-presentation-registry.generated.ts",
		"web/antd-v6/src/modules/operations/tablePresentation.ts",
		"web/antd-v6/src/shared/presentation/runtime.ts",
	} {
		if !presentationThinHostContractSensitive(path) {
			t.Errorf("presentation Thin Host contract did not select %q", path)
		}
	}
	for _, path := range []string{
		"docs/docs/release.md",
		"mss-boot/cache/cache.go",
		"web/antd-v6/README.md",
	} {
		if presentationThinHostContractSensitive(path) {
			t.Errorf("presentation Thin Host contract unexpectedly selected %q", path)
		}
	}
}

func TestFrontendBuildUsesPortableReleaseProfile(t *testing.T) {
	root := t.TempDir()
	spec := frontendBuild(root)
	if want := []string{"corepack", "pnpm@10.34.5", "build:release"}; !reflect.DeepEqual(spec.Args, want) {
		t.Fatalf("frontend build arguments = %q, want %q", spec.Args, want)
	}
	if spec.Directory != filepath.Join(root, "web", "antd-v6") {
		t.Fatalf("frontend build directory = %q", spec.Directory)
	}
}

func TestFrontendChecksUseOnlyV6ApplicationDirectory(t *testing.T) {
	root := t.TempDir()
	wantDirectory := filepath.Join(root, "web", "antd-v6")
	for _, spec := range []struct {
		name     string
		got      commandSpecView
		wantArgs []string
	}{
		{name: "lint", got: commandSpecView{frontendLint(root).Directory, frontendLint(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "lint"}},
		{name: "test", got: commandSpecView{frontendTest(root).Directory, frontendTest(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "test:ci"}},
		{name: "build", got: commandSpecView{frontendBuild(root).Directory, frontendBuild(root).Args}, wantArgs: []string{"corepack", "pnpm@10.34.5", "build:release"}},
	} {
		if spec.got.directory != wantDirectory {
			t.Fatalf("%s directory = %q, want %q", spec.name, spec.got.directory, wantDirectory)
		}
		if !reflect.DeepEqual(spec.got.args, spec.wantArgs) {
			t.Fatalf("%s arguments = %q, want %q", spec.name, spec.got.args, spec.wantArgs)
		}
	}
}

func TestFrontendV6FullChecksRequireConfiguredApplication(t *testing.T) {
	ctx := &project.Context{}
	if hasFrontendApplication(ctx, "web/antd-v6") {
		t.Fatal("empty project context unexpectedly enables the V6 frontend")
	}
	ctx.Project.Spec.Frontend.Applications = []project.FrontendApplicationSpec{
		{ID: "antd-v6", Path: "web/antd-v6"},
	}
	if !hasFrontendApplication(ctx, "web/antd-v6") {
		t.Fatal("configured v6 frontend was not detected")
	}
}

type commandSpecView struct {
	directory string
	args      []string
}

func TestValidateContractsRejectsInvalidFeature(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, ".mss", "features")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("create feature directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(featureDir, "invalid.yaml"),
		[]byte("apiVersion: mss.io/v1alpha1\nkind: Feature\nmetadata: {}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write invalid feature: %v", err)
	}

	result := validateContracts(root)
	if result.ExitCode == 0 {
		t.Fatalf("invalid FeatureSpec was ignored: %#v", result)
	}
}
