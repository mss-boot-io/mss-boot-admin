package blueprint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
