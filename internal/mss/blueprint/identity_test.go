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
  releaseTargetState: active
  immutableStoppedTrains:
    - version: v1.3.5
      commit: 396f60615cdfa589353b16ef9d3531e249e65432
      refs:
        root: v1.3.5
        framework: mss-boot/v1.3.5
        admin: admin/v1.3.5
        frontend: web/antd-v6/v1.3.5
        docs: docs/v1.3.5
        npm: "@mss-boot-io/admin-web@1.3.5"
    - version: v1.3.6
      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277
      refs:
        root: v1.3.6
        framework: mss-boot/v1.3.6
        admin: admin/v1.3.6
        frontend: web/antd-v6/v1.3.6
        docs: docs/v1.3.6
        npm: "@mss-boot-io/admin-web@1.3.6"
  publicationWorkflowsReady: true
  docsRevisionPublicationReady: false
  publicPrereleases: true
  rootTagTemplate: "{version}"
  frameworkTagTemplate: "mss-boot/{version}"
  adminTagTemplate: "admin/{version}"
  frontendTagTemplate: "web/antd-v6/{version}"
  docsTagTemplate: "docs/{version}"
  npmPackageTemplate: "@mss-boot-io/admin-web@{npmVersion}"
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
  releaseTargetState: active
  immutableStoppedTrains:
    - version: v1.3.5
      commit: 396f60615cdfa589353b16ef9d3531e249e65432
      refs:
        root: v1.3.5
        framework: mss-boot/v1.3.5
        admin: admin/v1.3.5
        frontend: web/antd-v6/v1.3.5
        docs: docs/v1.3.5
        npm: "@mss-boot-io/admin-web@1.3.5"
    - version: v1.3.6
      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277
      refs:
        root: v1.3.6
        framework: mss-boot/v1.3.6
        admin: admin/v1.3.6
        frontend: web/antd-v6/v1.3.6
        docs: docs/v1.3.6
        npm: "@mss-boot-io/admin-web@1.3.6"
  publicationWorkflowsReady: false
  docsTagMutable: true
  stablePromotionReady: false
  stablePromotionVersion: v1.1.0
  stablePromotionCommit: disabled
  publicPrereleases: false
  rootTagTemplate: "{version}"
  frameworkTagTemplate: "mss-boot/{version}"
  adminTagTemplate: "admin/{version}"
  frontendTagTemplate: "web/antd-v6/{version}"
  docsTagTemplate: "docs/{version}"
  npmPackageTemplate: "@mss-boot-io/admin-web@{npmVersion}"
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
	if policy.Spec.ReleaseTargetState != "active" || policy.Spec.DocsTagMutable == nil || !*policy.Spec.DocsTagMutable || policy.Spec.StablePromotionReady == nil || *policy.Spec.StablePromotionReady || policy.Spec.StablePromotionVersion == nil || *policy.Spec.StablePromotionVersion != "v1.1.0" || policy.Spec.NpmPackageTemplate != "@mss-boot-io/admin-web@{npmVersion}" {
		t.Fatalf("extended release contract = %#v", policy.Spec)
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
		{name: "invalid target state", data: strings.Replace(valid, "releaseTargetState: active", "releaseTargetState: pending", 1), want: "releaseTargetState must equal active or stopped"},
		{name: "missing docs publication boolean", data: strings.Replace(valid, "  docsTagMutable: true\n", "", 1), want: "boolean controls are required"},
		{name: "missing stable promotion boolean", data: strings.Replace(valid, "  stablePromotionReady: false\n", "", 1), want: "boolean controls are required"},
		{name: "missing lifecycle string", data: strings.Replace(valid, "  stablePromotionCommit: disabled\n", "", 1), want: "boolean controls are required"},
		{name: "empty lifecycle string", data: strings.Replace(valid, "stablePromotionCommit: disabled", "stablePromotionCommit: \"\"", 1), want: "lifecycle authorization fields are required"},
		{name: "promotion version mismatch", data: strings.Replace(valid, "stablePromotionVersion: v1.1.0", "stablePromotionVersion: v1.2.0", 1), want: "stablePromotionVersion must equal nextPublicVersion"},
		{name: "promotion commit enabled early", data: strings.Replace(valid, "stablePromotionCommit: disabled", "stablePromotionCommit: 1111111111111111111111111111111111111111", 1), want: "stablePromotionCommit must be disabled"},
		{name: "mutable docs disabled", data: strings.Replace(valid, "docsTagMutable: true", "docsTagMutable: false", 1), want: "docsTagMutable must remain true"},
		{name: "stopped target version mismatch", data: strings.Replace(valid, "releaseTargetState: active", "releaseTargetState: stopped", 1), want: "stopped target must belong to immutableStoppedTrains"},
		{name: "duplicate stopped version", data: strings.Replace(valid, "- version: v1.3.6", "- version: v1.3.5", 1), want: "duplicates version v1.3.5"},
		{name: "abbreviated stopped commit", data: strings.Replace(valid, "b1fe47a3a83209574e09d53526b122dd2cbc5277", "b1fe47a3", 1), want: "commit must be a full commit"},
		{name: "missing stopped ref", data: strings.Replace(valid, "        docs: docs/v1.3.6\n", "", 1), want: "missing docs ref"},
		{name: "stopped ref template mismatch", data: strings.Replace(valid, "framework: mss-boot/v1.3.6", "framework: mss-boot/v1.3.5", 1), want: "framework ref must remain"},
		{name: "unknown stopped train field", data: strings.Replace(valid, "      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277\n", "      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277\n      unsupported: true\n", 1), want: "field unsupported"},
		{name: "invalid npm package template", data: strings.Replace(valid, "@mss-boot-io/admin-web@{npmVersion}", "@mss-boot-io/admin-web@1.1.0", 1), want: "npmPackageTemplate must contain exactly one {npmVersion} placeholder"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeFoundationReleasePolicy([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("decodeFoundationReleasePolicy() error = %v, want %q", err, test.want)
			}
		})
	}

	t.Run("legacy policy without lifecycle authorization remains compatible", func(t *testing.T) {
		legacy := strings.Replace(
			valid,
			"  docsTagMutable: true\n",
			"  docsRevisionPublicationReady: false\n",
			1,
		)
		for _, line := range []string{
			"  stablePromotionReady: false\n",
			"  stablePromotionVersion: v1.1.0\n",
			"  stablePromotionCommit: disabled\n",
		} {
			legacy = strings.Replace(legacy, line, "", 1)
		}
		if _, err := decodeFoundationReleasePolicy([]byte(legacy)); err != nil {
			t.Fatalf("decode legacy policy: %v", err)
		}
	})

	t.Run("stable promotion exact binding", func(t *testing.T) {
		ready := strings.NewReplacer(
			"publicationWorkflowsReady: false", "publicationWorkflowsReady: true",
			"stablePromotionReady: false", "stablePromotionReady: true",
			"stablePromotionCommit: disabled", "stablePromotionCommit: 1111111111111111111111111111111111111111",
		).Replace(valid)
		if _, err := decodeFoundationReleasePolicy([]byte(ready)); err != nil {
			t.Fatalf("decode promotion-ready policy: %v", err)
		}

		for _, test := range []struct {
			name string
			data string
			want string
		}{
			{name: "workflows disabled", data: strings.Replace(ready, "publicationWorkflowsReady: true", "publicationWorkflowsReady: false", 1), want: "workflows must remain ready"},
			{name: "short commit", data: strings.Replace(ready, strings.Repeat("1", 40), strings.Repeat("1", 39), 1), want: "full commit"},
			{name: "uppercase commit", data: strings.Replace(ready, strings.Repeat("1", 40), strings.Repeat("A", 40), 1), want: "full commit"},
			{name: "consumed promotion", data: strings.Replace(ready, "currentStableVersion: v1.0.0", "currentStableVersion: v1.1.0", 1), want: "already consumed"},
		} {
			t.Run(test.name, func(t *testing.T) {
				if _, err := decodeFoundationReleasePolicy([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("decodeFoundationReleasePolicy() error = %v, want %q", err, test.want)
				}
			})
		}
	})

	t.Run("docs revision exact binding", func(t *testing.T) {
		legacy := strings.Replace(
			valid,
			"  docsTagMutable: true\n",
			"  docsRevisionPublicationReady: false\n  docsRevisionVersion: disabled\n  docsRevisionCommit: disabled\n",
			1,
		)
		ready := strings.NewReplacer(
			"docsRevisionPublicationReady: false", "docsRevisionPublicationReady: true",
			"docsRevisionVersion: disabled", "docsRevisionVersion: v1.0.0+docs.1",
			"docsRevisionCommit: disabled", "docsRevisionCommit: 2222222222222222222222222222222222222222",
		).Replace(legacy)
		for _, revision := range []string{"1", "999"} {
			candidate := strings.Replace(ready, "+docs.1", "+docs."+revision, 1)
			if _, err := decodeFoundationReleasePolicy([]byte(candidate)); err != nil {
				t.Fatalf("decode docs revision %s: %v", revision, err)
			}
		}

		for _, test := range []struct {
			name string
			data string
			want string
		}{
			{name: "wrong base", data: strings.Replace(ready, "v1.0.0+docs.1", "v1.1.0+docs.1", 1), want: "current stable"},
			{name: "zero", data: strings.Replace(ready, "+docs.1", "+docs.0", 1), want: "current stable"},
			{name: "leading zero", data: strings.Replace(ready, "+docs.1", "+docs.01", 1), want: "current stable"},
			{name: "too large", data: strings.Replace(ready, "+docs.1", "+docs.1000", 1), want: "current stable"},
			{name: "short commit", data: strings.Replace(ready, strings.Repeat("2", 40), strings.Repeat("2", 39), 1), want: "full commit"},
			{name: "uppercase commit", data: strings.Replace(ready, strings.Repeat("2", 40), strings.Repeat("A", 40), 1), want: "full commit"},
		} {
			t.Run(test.name, func(t *testing.T) {
				if _, err := decodeFoundationReleasePolicy([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("decodeFoundationReleasePolicy() error = %v, want %q", err, test.want)
				}
			})
		}
	})
}

func TestDecodeCanonicalFoundationReleasePolicy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".mss", "release-policy.yaml"))
	if err != nil {
		t.Fatalf("read canonical release policy: %v", err)
	}
	policy, err := decodeFoundationReleasePolicy(data)
	if err != nil {
		t.Fatalf("decode canonical release policy: %v", err)
	}
	if policy.Spec.DocsTagMutable == nil || !*policy.Spec.DocsTagMutable || policy.Spec.DocsRevisionPublicationReady != nil || policy.Spec.StablePromotionReady == nil || *policy.Spec.StablePromotionReady || policy.Spec.StablePromotionVersion == nil || *policy.Spec.StablePromotionVersion != "v1.3.7" || policy.Spec.StablePromotionCommit == nil || *policy.Spec.StablePromotionCommit != "disabled" || len(policy.Spec.ImmutableStoppedTrains) != 2 {
		t.Fatalf("canonical extended release controls = %#v", policy.Spec)
	}
	if policy.Spec.NextPublicVersion != "v1.3.7" || policy.Spec.PublicationWorkflowsReady == nil || !*policy.Spec.PublicationWorkflowsReady {
		t.Fatalf("canonical recovery target = %#v, want v1.3.7 with publication enabled", policy.Spec)
	}
	if policy.Spec.CurrentStableVersion != "v1.3.7" || policy.Spec.CurrentStableCommit != "77b53d41092741eac62fa6418c0bdbf87413c7cd" {
		t.Fatalf("canonical current stable = %#v, want exact v1.3.7 release", policy.Spec)
	}
	stopped, err := validateFoundationStoppedTrains(policy)
	if err != nil {
		t.Fatalf("validate canonical immutable stopped trains: %v", err)
	}
	if stopped["v1.3.5"].Commit != "396f60615cdfa589353b16ef9d3531e249e65432" || stopped["v1.3.6"].Commit != "b1fe47a3a83209574e09d53526b122dd2cbc5277" {
		t.Fatalf("canonical immutable stopped trains = %#v", stopped)
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
