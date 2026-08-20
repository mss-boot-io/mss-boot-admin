package blueprint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
)

func TestBuildDesiredRecordsIndependentFoundationBlueprintGeneratorAndSnapshotIdentities(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	files, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	identities := manifest.Identities
	if identities.Foundation.Version != "1.1.0" || identities.Foundation.Channel != "candidate" {
		t.Fatalf("foundation identity = %#v, want 1.1.0 candidate", identities.Foundation)
	}
	if identities.Blueprint.Version != "0.1.0" {
		t.Fatalf("blueprint version = %q, want 0.1.0", identities.Blueprint.Version)
	}
	if identities.Generator.Version != buildinfo.VersionString() {
		t.Fatalf("generator version = %q, want %q", identities.Generator.Version, buildinfo.VersionString())
	}
	if !sha256Pattern.MatchString(identities.Snapshot.SHA256) {
		t.Fatalf("snapshot digest = %q, want SHA-256", identities.Snapshot.SHA256)
	}
	if identities.Foundation.Version == identities.Blueprint.Version ||
		identities.Blueprint.Version == identities.Generator.Version {
		t.Fatalf("independent versions were conflated: %#v", identities)
	}
	if _, exists := manifest.Files[blueprint.Spec.LockPath]; exists {
		t.Fatal("lock record was included in the ordinary managed baseline")
	}
	if _, exists := manifest.Files[blueprint.Spec.ManifestPath]; exists {
		t.Fatal("manifest record was included in the ordinary managed baseline")
	}
	lockFile := files[blueprint.Spec.LockPath]
	manifestFile := files[blueprint.Spec.ManifestPath]
	lock, err := decodeFoundationLock(lockFile.Data)
	if err != nil {
		t.Fatalf("decode generated lock: %v", err)
	}
	decodedManifest, legacy, err := decodeManifest(manifestFile.Data, false)
	if err != nil || legacy {
		t.Fatalf("decode generated manifest: legacy=%t err=%v", legacy, err)
	}
	if err := validateSnapshotPair(decodedManifest, lock, lockFile.Data); err != nil {
		t.Fatalf("generated pair does not cross-validate: %v", err)
	}
}

func TestBuildDesiredUsesReleasePolicyNotProjectGenerationBaseline(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	_, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	if got, want := manifest.Identities.Foundation.Version, "1.1.0"; got != want {
		t.Fatalf("foundation version = %q, want %q", got, want)
	}
	if got, want := manifest.Identities.Blueprint.Version, "0.1.0"; got != want {
		t.Fatalf("blueprint version = %q, want %q", got, want)
	}
	if got, want := manifest.Identities.Foundation.Source, ".mss/release-policy.yaml"; got != want {
		t.Fatalf("foundation version source = %q, want %q", got, want)
	}
}

func TestBuildDesiredMarksExactPublicTagAsStableFoundationRelease(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	runGit(t, root, "tag", "v1.1.0")
	_, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	if got := manifest.Identities.Foundation; got.Version != "1.1.0" || got.Channel != "stable" {
		t.Fatalf("foundation identity = %#v, want exact tagged 1.1.0 stable", got)
	}
}

func TestFoundationReleaseVersionKeepsExactPrereleaseTagOnCandidateChannel(t *testing.T) {
	root, _, _ := identityFixture(t)
	commitOutput, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("resolve fixture commit: %v", err)
	}
	commit := strings.TrimSpace(string(commitOutput))
	runGit(t, root, "tag", "v1.1.0-rc.1")
	policy, err := decodeFoundationReleasePolicy([]byte(`apiVersion: mss.io/v1alpha1
kind: ReleasePolicy
metadata:
  name: fixture
spec:
  mode: development-first
  releaseBranch: main
  requireMergedPullRequestSource: true
  currentStableVersion: v1.0.0
  currentStableCommit: 0000000000000000000000000000000000000000
  nextPublicVersion: v1.1.0-rc.1
  distributionVersion: v1.1.0-rc.1
  distributionComponents: "root,framework,admin,frontend"
  publicationWorkflowsReady: true
  publicPrereleases: true
  rootTagTemplate: "{version}"
  frameworkTagTemplate: "mss-boot/{version}"
  adminTagTemplate: "admin/{version}"
  frontendTagTemplate: "web/antd-v6/{version}"
  docsTagTemplate: "docs/{version}"
`))
	if err != nil {
		t.Fatalf("decode prerelease policy: %v", err)
	}
	version, channel, err := foundationReleaseVersion(context.Background(), root, commit, policy)
	if err != nil {
		t.Fatalf("foundationReleaseVersion(prerelease): %v", err)
	}
	if version != "1.1.0-rc.1" || channel != "candidate" {
		t.Fatalf("prerelease identity = %s %s, want 1.1.0-rc.1 candidate", version, channel)
	}
}

func TestDecodeFoundationReleasePolicyRejectsUnknownFieldsAndYAMLGraphs(t *testing.T) {
	valid := `apiVersion: mss.io/v1alpha1
kind: ReleasePolicy
metadata:
  name: fixture
spec:
  mode: development-first
  releaseBranch: main
  requireMergedPullRequestSource: true
  currentStableVersion: v1.0.0
  currentStableCommit: 0000000000000000000000000000000000000000
  nextPublicVersion: v1.1.0
  distributionVersion: v1.1.0
  distributionComponents: "root,framework,admin,frontend"
  publicationWorkflowsReady: false
  publicPrereleases: false
  rootTagTemplate: "{version}"
  frameworkTagTemplate: "mss-boot/{version}"
  adminTagTemplate: "admin/{version}"
  frontendTagTemplate: "web/antd-v6/{version}"
  docsTagTemplate: "docs/{version}"
`
	policy, err := decodeFoundationReleasePolicy([]byte(valid))
	if err != nil {
		t.Fatalf("decodeFoundationReleasePolicy(valid) error = %v", err)
	}
	if policy.Spec.ReleaseBranch != "main" || policy.Spec.RequireMergedPRSource == nil || !*policy.Spec.RequireMergedPRSource {
		t.Fatalf("release source governance = branch %q required %#v, want main/true", policy.Spec.ReleaseBranch, policy.Spec.RequireMergedPRSource)
	}
	if got, want := policy.Spec.DocsTagTemplate, "docs/{version}"; got != want {
		t.Fatalf("docs tag template = %q, want %q", got, want)
	}
	if policy.Spec.DistributionVersion != "v1.1.0" || policy.Spec.DistributionComponents != "root,framework,admin,frontend" || policy.Spec.AdminTagTemplate != "admin/{version}" {
		t.Fatalf("Admin Distribution release contract = %#v", policy.Spec)
	}
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "unknown", data: strings.Replace(valid, "  mode:", "  unsupported: true\n  mode:", 1), want: "field unsupported"},
		{name: "anchor", data: strings.Replace(valid, "metadata:", "metadata: &metadata", 1), want: "anchors and aliases"},
		{name: "invalid docs tag", data: strings.Replace(valid, "docs/{version}", "docs/v1.2.3", 1), want: "docsTagTemplate must contain exactly one {version} placeholder"},
		{name: "distribution version mismatch", data: strings.Replace(valid, "distributionVersion: v1.1.0", "distributionVersion: v1.2.0", 1), want: "distributionVersion must equal nextPublicVersion"},
		{name: "disabled prerelease target", data: strings.Replace(strings.Replace(valid, "nextPublicVersion: v1.1.0", "nextPublicVersion: v1.1.0-rc.1", 1), "distributionVersion: v1.1.0", "distributionVersion: v1.1.0-rc.1", 1), want: "publicPrereleases must be true"},
		{name: "prerelease stable", data: strings.Replace(valid, "currentStableVersion: v1.0.0", "currentStableVersion: v1.0.0-rc.1", 1), want: "currentStableVersion must not be a prerelease"},
		{name: "missing Admin component", data: strings.Replace(valid, "root,framework,admin,frontend", "root,framework,frontend", 1), want: "distributionComponents"},
		{name: "invalid Admin tag", data: strings.Replace(valid, "admin/{version}", "admin/v1.1.0", 1), want: "adminTagTemplate must contain exactly one {version} placeholder"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeFoundationReleasePolicy([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("decodeFoundationReleasePolicy() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBuildDesiredUsesActualBuildInfoGeneratorVersion(t *testing.T) {
	originalVersion, originalCommit := buildinfo.Version, buildinfo.Commit
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit = originalVersion, originalCommit
	})
	buildinfo.Version = "9.8.7-test"
	buildinfo.Commit = strings.Repeat("a", 40)
	root, blueprint, application := identityFixture(t)
	_, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	if manifest.Identities.Generator.Version != "9.8.7-test" || manifest.Identities.Generator.Commit != strings.Repeat("a", 40) {
		t.Fatalf("unexpected generator identity: %#v", manifest.Identities.Generator)
	}
}

func TestBuildDesiredRejectsAbbreviatedGeneratorCommit(t *testing.T) {
	originalVersion, originalCommit := buildinfo.Version, buildinfo.Commit
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit = originalVersion, originalCommit
	})
	buildinfo.Version = "9.8.7-test"
	buildinfo.Commit = "deadbeef"
	root, blueprint, application := identityFixture(t)
	_, _, err := BuildDesired(context.Background(), root, blueprint, application)
	if err == nil || !strings.Contains(err.Error(), "full 40-character") {
		t.Fatalf("BuildDesired() error = %v, want abbreviated generator commit rejection", err)
	}
}

func TestBuildDesiredRecordsCommittedBlueprintDigest(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	_, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	command := exec.Command("git", "-C", root, "show", "HEAD:.mss/blueprints/management-system.yaml")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("read committed blueprint: %v", err)
	}
	if got, want := manifest.Identities.Blueprint.SHA256, digest(data); got != want {
		t.Fatalf("blueprint digest = %q, want %q", got, want)
	}
}

func TestBuildDesiredRejectsDirtyTrackedFoundation(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("dirty tracked foundation\n"), 0o644); err != nil {
		t.Fatalf("dirty fixture: %v", err)
	}
	_, _, err := BuildDesired(context.Background(), root, blueprint, application)
	if err == nil || !strings.Contains(err.Error(), "dirty tracked files") {
		t.Fatalf("BuildDesired() error = %v, want dirty tracked foundation error", err)
	}
}

func TestBuildDesiredRejectsStagedTrackedFoundation(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("staged tracked foundation\n"), 0o644); err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	runGit(t, root, "add", "AGENTS.md")
	_, _, err := BuildDesired(context.Background(), root, blueprint, application)
	if err == nil || !strings.Contains(err.Error(), "dirty tracked files") {
		t.Fatalf("BuildDesired() error = %v, want staged tracked foundation rejection", err)
	}
}

func TestBuildDesiredUsesCommittedTreeAndIgnoresUntrackedFiles(t *testing.T) {
	root, blueprint, application := identityFixture(t)
	untracked := filepath.Join(root, "untracked-secret.txt")
	if err := os.WriteFile(untracked, []byte("not part of the committed foundation\n"), 0o644); err != nil {
		t.Fatalf("write untracked fixture: %v", err)
	}
	files, manifest, err := BuildDesired(context.Background(), root, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	if _, exists := files["untracked-secret.txt"]; exists {
		t.Fatal("untracked worktree file entered the desired output")
	}
	if _, exists := manifest.Files["untracked-secret.txt"]; exists {
		t.Fatal("untracked worktree file entered the snapshot baseline")
	}
}

func TestLoadRejectsCoincidentLockAndManifestPaths(t *testing.T) {
	root := writeBlueprintFixture(t)
	path := filepath.Join(root, ".mss", "blueprints", "management-system.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blueprint: %v", err)
	}
	data = []byte(strings.Replace(string(data), "lockPath: .mss/lock.yaml", "lockPath: .mss/blueprint-manifest.json", 1))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if _, err := Load(root, "management-system"); err == nil || !strings.Contains(err.Error(), "must be distinct") {
		t.Fatalf("Load() error = %v, want coincident record path error", err)
	}
}

func TestSafeRelativePathUsesPortableRepositoryGrammar(t *testing.T) {
	for _, unsafe := range []string{
		"../outside",
		`..\outside`,
		"C:/outside",
		`C:\outside`,
		"//server/share",
		"nested/file:stream",
	} {
		if safeRelativePath(unsafe) {
			t.Errorf("safeRelativePath(%q) = true, want portable-path rejection", unsafe)
		}
	}
	for _, safe := range []string{"go.mod", ".mss/lock.yaml", "admin/modules/example/file.go"} {
		if !safeRelativePath(safe) {
			t.Errorf("safeRelativePath(%q) = false, want safe", safe)
		}
	}
}

func identityFixture(t *testing.T) (string, *Document, Application) {
	t.Helper()
	root := writeBlueprintFixture(t)
	document, err := Load(root, "management-system")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return root, document, Application{
		Name:        "identity-admin",
		DisplayName: "Identity Administration",
		Module:      "github.com/acme/identity-admin",
		Repository:  "acme/identity-admin",
	}
}
