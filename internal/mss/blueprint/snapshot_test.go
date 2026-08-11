package blueprint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestReadSnapshotRejectsMissingIdentityFields(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	path := filepath.Join(root, filepath.FromSlash(manifest.Records.ManifestPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode manifest map: %v", err)
	}
	identities := document["identities"].(map[string]any)
	delete(identities, "generator")
	writeJSONFile(t, path, document)
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "generator") {
		t.Fatalf("ReadSnapshot() error = %v, want missing generator identity error", err)
	}
}

func TestReadSnapshotRejectsInvalidFullCommit(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	manifest.Identities.Foundation.Commit = "deadbeef"
	manifest.Metadata.FoundationCommit = "deadbeef"
	writeManifestFile(t, root, manifest)
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "full 40-character") {
		t.Fatalf("ReadSnapshot() error = %v, want full commit error", err)
	}
}

func TestReadSnapshotRejectsDuplicateJSONIdentityKeys(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	path := filepath.Join(root, filepath.FromSlash(manifest.Records.ManifestPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	duplicate := strings.Replace(string(data), `"identities": {`, `"identities": {},
  "identities": {`, 1)
	if duplicate == string(data) {
		t.Fatal("manifest did not contain identities object")
	}
	if err := os.WriteFile(path, []byte(duplicate), 0o644); err != nil {
		t.Fatalf("write duplicate manifest: %v", err)
	}
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
		t.Fatalf("ReadSnapshot() error = %v, want duplicate key error", err)
	}
}

func TestReadSnapshotRejectsYAMLGraphFeaturesInFoundationLock(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	path := filepath.Join(root, filepath.FromSlash(manifest.Records.LockPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	data = []byte(strings.Replace(string(data), "metadata:\n", "metadata: &metadata\n", 1))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write anchored lock: %v", err)
	}
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "anchors and aliases") {
		t.Fatalf("ReadSnapshot() error = %v, want YAML graph rejection", err)
	}
}

func TestReadSnapshotRejectsStaleLockDigest(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	path := filepath.Join(root, filepath.FromSlash(manifest.Records.LockPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if err := os.WriteFile(path, append(data, []byte("# stale lock bytes\n")...), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "lock digest contradicts") {
		t.Fatalf("ReadSnapshot() error = %v, want stale lock digest error", err)
	}
}

func TestReadSnapshotRejectsContradictoryLockAndManifest(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	lockPath := filepath.Join(root, filepath.FromSlash(manifest.Records.LockPath))
	lockData, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	lock, err := decodeFoundationLock(lockData)
	if err != nil {
		t.Fatalf("decode lock: %v", err)
	}
	lock.Spec.Identities.Generator.Version = "8.8.8-contradiction"
	lockData, err = yaml.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(lockPath, lockData, 0o644); err != nil {
		t.Fatalf("write contradictory lock: %v", err)
	}
	manifest.Records.LockSHA256 = digest(lockData)
	writeManifestFile(t, root, manifest)
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "identities contradict") {
		t.Fatalf("ReadSnapshot() error = %v, want contradictory identity error", err)
	}
}

func TestReadSnapshotRejectsRecomputedSingleRecordForgery(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	manifest.Identities.Foundation.Version = "7.7.7-forged"
	manifest.Metadata.FoundationCommit = manifest.Identities.Foundation.Commit
	manifest.Metadata.FoundationRepository = manifest.Identities.Foundation.Repository
	manifest.Metadata.FoundationTimestamp = manifest.Identities.Foundation.Timestamp
	manifest.Identities.Snapshot.SHA256 = ""
	forgedDigest, err := computeSnapshotDigest(manifest.Identities, manifest.Files)
	if err != nil {
		t.Fatalf("compute forged snapshot digest: %v", err)
	}
	manifest.Identities.Snapshot.SHA256 = forgedDigest
	writeManifestFile(t, root, manifest)
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "identities contradict") {
		t.Fatalf("ReadSnapshot() error = %v, want single-record forgery error", err)
	}
}

func TestReadSnapshotAcceptsLegacyManifestForUpgradeOnly(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	manifest.APIVersion = legacyAPIVersion
	manifest.Identities = IdentitySet{}
	manifest.Records = ManifestRecords{}
	writeManifestAtPath(t, root, ".mss/blueprint-manifest.json", manifest)
	if _, err := ReadSnapshot(root, ""); err == nil || !strings.Contains(err.Error(), "upgrade inputs only") {
		t.Fatalf("ReadSnapshot() error = %v, want legacy status rejection", err)
	}
	legacy, accepted, err := readManifestForUpgrade(root, "")
	if err != nil {
		t.Fatalf("readManifestForUpgrade() error = %v", err)
	}
	if !accepted || legacy.APIVersion != legacyAPIVersion {
		t.Fatalf("legacy upgrade input was not accepted: accepted=%t manifest=%#v", accepted, legacy)
	}
}

func TestReadSnapshotStatusPreservesFlatMetadataAndAddsIdentityRecords(t *testing.T) {
	root, manifest := generatedSnapshotFixture(t)
	status, err := ReadSnapshotStatus(root, "")
	if err != nil {
		t.Fatalf("ReadSnapshotStatus() error = %v", err)
	}
	if status.Project != manifest.Metadata.Project || status.BlueprintVersion != manifest.Metadata.BlueprintVersion {
		t.Fatalf("flat compatibility metadata = %#v, want %#v", status.ManifestMetadata, manifest.Metadata)
	}
	if !equalIdentitySets(status.Identities, manifest.Identities) || status.Records != manifest.Records {
		t.Fatalf("status omitted strict snapshot data: %#v", status)
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode status JSON: %v", err)
	}
	for _, flat := range []string{"project", "module", "repository", "blueprint", "blueprintVersion", "foundationRepository", "foundationCommit", "generatorVersion"} {
		if _, ok := document[flat]; !ok {
			t.Errorf("flat compatibility field %q is missing: %s", flat, data)
		}
	}
	for _, strict := range []string{"identities", "records"} {
		if _, ok := document[strict]; !ok {
			t.Errorf("strict status field %q is missing: %s", strict, data)
		}
	}
	if err := status.ValidateProjectIdentity("snapshot-admin", "acme/snapshot-admin"); err != nil {
		t.Fatalf("ValidateProjectIdentity() error = %v", err)
	}
	if err := status.ValidateProjectIdentity("other-admin", "acme/snapshot-admin"); err == nil || !strings.Contains(err.Error(), "metadata.name") {
		t.Fatalf("name contradiction error = %v", err)
	}
	if err := status.ValidateProjectIdentity("snapshot-admin", "acme/other-admin"); err == nil || !strings.Contains(err.Error(), "metadata.repository") {
		t.Fatalf("repository contradiction error = %v", err)
	}
}

func TestInspectSnapshotDistinguishesStrictFoundationSourceSentinel(t *testing.T) {
	root := t.TempDir()
	lock := `apiVersion: mss.io/v1alpha1
kind: FoundationLock
metadata:
  project: mss-boot-admin
spec:
  foundation:
    repository: mss-boot-io/mss-boot-admin
    version: 0.1.0
    channel: development
  blueprint:
    name: management-system
    version: 0.1.0
  contracts:
    project: v1alpha1
  generatedBy:
    tool: mss
    version: 0.1.0-dev
  modules: {}
  upgrades: []
`
	path := filepath.Join(root, ".mss", "lock.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create source lock parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(lock), 0o644); err != nil {
		t.Fatalf("write source lock: %v", err)
	}

	inspection, err := InspectSnapshot(root, "", "mss-boot-admin", "mss-boot-io/mss-boot-admin")
	if err != nil {
		t.Fatalf("InspectSnapshot() error = %v", err)
	}
	if inspection.Role != SnapshotRoleFoundationSource || inspection.Source == nil || inspection.Status != nil {
		t.Fatalf("source inspection = %#v", inspection)
	}
	if inspection.Source.FoundationVersion != "0.1.0" || inspection.Source.GeneratorVersion != "0.1.0-dev" {
		t.Fatalf("source identity = %#v", inspection.Source)
	}
}

func TestInspectSnapshotRequiresValidGeneratedPairAndNeverFallsBack(t *testing.T) {
	root, _ := generatedSnapshotFixture(t)
	inspection, err := InspectSnapshot(root, "", "snapshot-admin", "acme/snapshot-admin")
	if err != nil {
		t.Fatalf("InspectSnapshot() valid generated error = %v", err)
	}
	if inspection.Role != SnapshotRoleGenerated || inspection.Status == nil || inspection.Source != nil {
		t.Fatalf("generated inspection = %#v", inspection)
	}

	manifestPath := filepath.Join(root, ".mss", "blueprint-manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("corrupt current manifest: %v", err)
	}
	if _, err := InspectSnapshot(root, "", "snapshot-admin", "acme/snapshot-admin"); err == nil || !strings.Contains(err.Error(), "unsupported blueprint manifest identity") {
		t.Fatalf("malformed current inspection error = %v", err)
	}

	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove current manifest: %v", err)
	}
	if _, err := InspectSnapshot(root, "", "snapshot-admin", "acme/snapshot-admin"); err == nil || !strings.Contains(err.Error(), "orphan or malformed snapshot lock") {
		t.Fatalf("orphan current lock inspection error = %v", err)
	}
}

func TestInspectSnapshotWaitsForSourceToGeneratedTransition(t *testing.T) {
	generatedRoot, manifest := generatedSnapshotFixture(t)
	currentLock, err := os.ReadFile(filepath.Join(generatedRoot, filepath.FromSlash(manifest.Records.LockPath)))
	if err != nil {
		t.Fatalf("read current lock: %v", err)
	}
	currentManifest, err := os.ReadFile(filepath.Join(generatedRoot, filepath.FromSlash(manifest.Records.ManifestPath)))
	if err != nil {
		t.Fatalf("read current manifest: %v", err)
	}

	root := t.TempDir()
	lockPath := filepath.Join(root, ".mss", "lock.yaml")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("create transition root: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte(`apiVersion: mss.io/v1alpha1
kind: FoundationLock
metadata:
  project: snapshot-admin
spec:
  foundation:
    repository: acme/snapshot-admin
    version: 0.1.0
    channel: development
  blueprint:
    name: management-system
    version: 0.1.0
  contracts:
    project: v1alpha1
  generatedBy:
    tool: mss
    version: 0.1.0-dev
  modules: {}
  upgrades: []
`), 0o644); err != nil {
		t.Fatalf("write source sentinel: %v", err)
	}

	writerRoot, err := openManagedRoot(root, false)
	if err != nil {
		t.Fatalf("open transition root: %v", err)
	}
	defer writerRoot.Close()
	releaseWriter, err := acquireSnapshotWriter(context.Background(), writerRoot)
	if err != nil {
		t.Fatalf("acquire transition writer: %v", err)
	}
	writerReleased := false
	defer func() {
		if !writerReleased {
			releaseWriter()
		}
	}()
	if err := writerRoot.writeAtomic(manifest.Records.LockPath, currentLock, 0o644); err != nil {
		t.Fatalf("commit transition lock: %v", err)
	}

	type inspectionResult struct {
		inspection SnapshotInspection
		err        error
	}
	started := make(chan struct{})
	result := make(chan inspectionResult, 1)
	go func() {
		close(started)
		inspection, err := InspectSnapshot(root, "", "snapshot-admin", "acme/snapshot-admin")
		result <- inspectionResult{inspection: inspection, err: err}
	}()
	<-started
	if err := writerRoot.writeAtomic(manifest.Records.ManifestPath, currentManifest, 0o644); err != nil {
		t.Fatalf("commit transition manifest: %v", err)
	}
	releaseWriter()
	writerReleased = true

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("InspectSnapshot() transition error = %v", got.err)
		}
		if got.inspection.Role != SnapshotRoleGenerated || got.inspection.Status == nil {
			t.Fatalf("transition inspection = %#v", got.inspection)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("InspectSnapshot() did not resume after the atomic pair commit")
	}
}

func generatedSnapshotFixture(t *testing.T) (string, Manifest) {
	t.Helper()
	foundation := writeBlueprintFixture(t)
	root := filepath.Join(t.TempDir(), "snapshot-admin")
	_, err := Generate(context.Background(), Options{
		FoundationRoot: foundation,
		Destination:    root,
		Application: Application{
			Name:        "snapshot-admin",
			DisplayName: "Snapshot Administration",
			Module:      "github.com/acme/snapshot-admin",
			Repository:  "acme/snapshot-admin",
		},
		Write: true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	manifest, err := ReadManifest(root, "")
	if err != nil {
		t.Fatalf("ReadManifest() error = %v", err)
	}
	return root, manifest
}

func writeManifestFile(t *testing.T, root string, manifest Manifest) {
	t.Helper()
	writeManifestAtPath(t, root, manifest.Records.ManifestPath, manifest)
}

func writeManifestAtPath(t *testing.T, root, relative string, manifest Manifest) {
	t.Helper()
	data, err := renderManifest(manifest)
	if err != nil {
		t.Fatalf("render manifest: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}
