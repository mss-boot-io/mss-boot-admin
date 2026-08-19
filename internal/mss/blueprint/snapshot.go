package blueprint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"gopkg.in/yaml.v3"
)

const (
	manifestKind = "BlueprintManifest"
	lockKind     = "FoundationLock"
)

// FoundationLock is the YAML representation of one verified downstream
// snapshot. It carries the same identities and digest as Manifest.
type FoundationLock struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   FoundationLockMetadata `yaml:"metadata" json:"metadata"`
	Spec       FoundationLockSpec     `yaml:"spec" json:"spec"`
}

type FoundationLockMetadata struct {
	Project string `yaml:"project" json:"project"`
}

type FoundationLockSpec struct {
	Distribution project.DistributionSpec `yaml:"distribution,omitempty" json:"distribution,omitempty"`
	Identities   IdentitySet              `yaml:"identities" json:"identities"`
	Records      SnapshotRecordPaths      `yaml:"records" json:"records"`
	Contracts    map[string]string        `yaml:"contracts" json:"contracts"`
	Modules      map[string]any           `yaml:"modules" json:"modules"`
	Upgrades     []any                    `yaml:"upgrades" json:"upgrades"`
}

// Snapshot is a lock and manifest pair that has passed strict structural,
// digest, and cross-record validation.
type Snapshot struct {
	Manifest Manifest       `json:"manifest"`
	Lock     FoundationLock `json:"lock"`
}

func renderFoundationLock(identities IdentitySet, records SnapshotRecordPaths, distribution project.DistributionSpec) ([]byte, error) {
	lock := FoundationLock{
		APIVersion: snapshotAPIVersion,
		Kind:       lockKind,
		Metadata: FoundationLockMetadata{
			Project: identities.Snapshot.Project,
		},
		Spec: FoundationLockSpec{
			Distribution: distribution,
			Identities:   identities,
			Records:      records,
			Contracts: map[string]string{
				"project":           "v1alpha1",
				"capabilityCatalog": "v1alpha1",
				"commandCatalog":    "v1alpha1",
				"adminModule":       "v1alpha1",
				"feature":           "v1alpha1",
				"evaluation":        "v1alpha1",
				"adminDistribution": "v1alpha1",
			},
			Modules:  map[string]any{},
			Upgrades: []any{},
		},
	}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return nil, fmt.Errorf("render foundation lock: %w", err)
	}
	return data, nil
}

func renderManifest(manifest Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render blueprint manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// ReadSnapshot reads and cross-validates both representations of a current
// downstream snapshot. Legacy v1alpha1 manifests are intentionally rejected;
// they are accepted only by the upgrade input path.
func ReadSnapshot(root, manifestRelative string) (Snapshot, error) {
	managed, err := openManagedRoot(root, false)
	if err != nil {
		return Snapshot{}, err
	}
	defer managed.Close()
	release, err := acquireSnapshotReader(managed)
	if err != nil {
		return Snapshot{}, err
	}
	defer release()
	if err := managed.checkSnapshotTransaction(); err != nil {
		return Snapshot{}, err
	}
	return readSnapshotUnlocked(managed, manifestRelative)
}

func readSnapshotUnlocked(root *managedRoot, manifestRelative string) (Snapshot, error) {
	if manifestRelative == "" {
		manifestRelative = ".mss/blueprint-manifest.json"
	}
	manifestRelative = normalizedPath(manifestRelative)
	if !safeRelativePath(manifestRelative) {
		return Snapshot{}, errors.New("manifest path must be repository-relative")
	}
	manifestData, err := readRegularRecord(root, manifestRelative)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read blueprint manifest: %w", err)
	}
	manifest, legacy, err := decodeManifest(manifestData, false)
	if err != nil {
		return Snapshot{}, err
	}
	if legacy {
		return Snapshot{}, errors.New("legacy v1alpha1 blueprint manifests are upgrade inputs only")
	}
	if normalizedPath(manifest.Records.ManifestPath) != manifestRelative {
		return Snapshot{}, fmt.Errorf("manifest record path %q contradicts requested path %q", manifest.Records.ManifestPath, manifestRelative)
	}
	lockData, err := readRegularRecord(root, manifest.Records.LockPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read foundation lock: %w", err)
	}
	lock, err := decodeFoundationLock(lockData)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateSnapshotPair(manifest, lock, lockData); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Manifest: manifest, Lock: lock}, nil
}

// ReadManifest returns only a verified current manifest while retaining the
// original API as a source-compatibility bridge.
//
// Deprecated: new consumers must use ReadSnapshot or ReadSnapshotStatus so
// both cross-digested representations and all four identities remain visible.
func ReadManifest(root, relative string) (Manifest, error) {
	snapshot, err := ReadSnapshot(root, relative)
	if err != nil {
		return Manifest{}, err
	}
	return snapshot.Manifest, nil
}

func readManifestForUpgrade(root, relative string) (Manifest, bool, error) {
	managed, err := openManagedRoot(root, false)
	if err != nil {
		return Manifest{}, false, err
	}
	defer managed.Close()
	release, err := acquireSnapshotReader(managed)
	if err != nil {
		return Manifest{}, false, err
	}
	defer release()
	if err := managed.checkSnapshotTransaction(); err != nil {
		return Manifest{}, false, err
	}
	if relative == "" {
		relative = ".mss/blueprint-manifest.json"
	}
	relative = normalizedPath(relative)
	if !safeRelativePath(relative) {
		return Manifest{}, false, errors.New("manifest path must be repository-relative")
	}
	data, err := readRegularRecord(managed, relative)
	if err != nil {
		return Manifest{}, false, err
	}
	manifest, legacy, err := decodeManifest(data, true)
	if err != nil {
		return Manifest{}, false, err
	}
	if legacy {
		if err := validateLegacyManifest(manifest); err != nil {
			return Manifest{}, false, err
		}
		return manifest, true, nil
	}
	snapshot, err := readSnapshotUnlocked(managed, relative)
	if err != nil {
		return Manifest{}, false, err
	}
	return snapshot.Manifest, false, nil
}

func decodeManifest(data []byte, allowLegacy bool) (Manifest, bool, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return Manifest{}, false, fmt.Errorf("parse blueprint manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	manifest := Manifest{}
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, false, fmt.Errorf("parse blueprint manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Manifest{}, false, fmt.Errorf("parse blueprint manifest: %w", err)
	}
	if manifest.Kind != manifestKind {
		return Manifest{}, false, errors.New("unsupported blueprint manifest identity")
	}
	switch manifest.APIVersion {
	case snapshotAPIVersion:
		if err := validateCurrentManifest(manifest); err != nil {
			return Manifest{}, false, err
		}
		return manifest, false, nil
	case legacyAPIVersion:
		if !allowLegacy {
			return Manifest{}, true, errors.New("legacy v1alpha1 blueprint manifests are upgrade inputs only")
		}
		return manifest, true, nil
	default:
		return Manifest{}, false, errors.New("unsupported blueprint manifest identity")
	}
}

func validateCurrentManifest(manifest Manifest) error {
	if err := validateIdentitySet(
		manifest.Identities,
		manifest.Files,
		manifest.Records.LockPath,
		manifest.Records.ManifestPath,
		true,
	); err != nil {
		return fmt.Errorf("invalid blueprint manifest identities: %w", err)
	}
	if !sha256Pattern.MatchString(manifest.Records.LockSHA256) {
		return errors.New("blueprint manifest lock digest must be a SHA-256 digest")
	}
	want := ManifestMetadata{
		Project:              manifest.Identities.Snapshot.Project,
		Module:               manifest.Identities.Snapshot.Module,
		Repository:           manifest.Identities.Snapshot.Repository,
		Blueprint:            manifest.Identities.Blueprint.Name,
		BlueprintVersion:     manifest.Identities.Blueprint.Version,
		FoundationRepository: manifest.Identities.Foundation.Repository,
		FoundationCommit:     manifest.Identities.Foundation.Commit,
		FoundationTimestamp:  manifest.Identities.Foundation.Timestamp,
		GeneratorVersion:     manifest.Identities.Generator.Version,
		GeneratorCommit:      manifest.Identities.Generator.Commit,
	}
	if !reflect.DeepEqual(manifest.Metadata, want) {
		return errors.New("blueprint manifest compatibility metadata contradicts its independent identities")
	}
	return nil
}

func validateLegacyManifest(manifest Manifest) error {
	var problems []string
	if manifest.APIVersion != legacyAPIVersion || manifest.Kind != manifestKind {
		problems = append(problems, "unsupported legacy blueprint manifest identity")
	}
	if manifest.Metadata.Project == "" || manifest.Metadata.Module == "" || manifest.Metadata.Blueprint == "" {
		problems = append(problems, "legacy blueprint manifest metadata is incomplete")
	}
	if !validSemanticVersion(manifest.Metadata.BlueprintVersion) {
		problems = append(problems, "legacy blueprint version must be semantic")
	}
	if !fullCommitPattern.MatchString(manifest.Metadata.FoundationCommit) {
		problems = append(problems, "legacy foundation commit must be a full 40-character hexadecimal commit")
	}
	if !reflect.DeepEqual(manifest.Identities, IdentitySet{}) || !reflect.DeepEqual(manifest.Records, ManifestRecords{}) {
		problems = append(problems, "legacy blueprint manifest must not claim current snapshot identity fields")
	}
	if len(manifest.Files) == 0 {
		problems = append(problems, "legacy blueprint manifest files must not be empty")
	}
	for relative, file := range manifest.Files {
		if !safeRelativePath(relative) {
			problems = append(problems, "legacy managed path is unsafe: "+relative)
		}
		if relative != normalizedPath(relative) {
			problems = append(problems, "legacy managed path is not canonical: "+relative)
		}
		if !sha256Pattern.MatchString(file.SHA256) {
			problems = append(problems, "legacy managed digest is invalid: "+relative)
		}
		if file.Size < 0 || file.Mode.Perm() == 0 {
			problems = append(problems, "legacy managed metadata is invalid: "+relative)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func decodeFoundationLock(data []byte) (FoundationLock, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return FoundationLock{}, fmt.Errorf("parse foundation lock: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	lock := FoundationLock{}
	if err := decoder.Decode(&lock); err != nil {
		return FoundationLock{}, fmt.Errorf("parse foundation lock: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return FoundationLock{}, errors.New("parse foundation lock: multiple YAML documents are not supported")
		}
		return FoundationLock{}, fmt.Errorf("parse foundation lock: %w", err)
	}
	if lock.APIVersion != snapshotAPIVersion || lock.Kind != lockKind {
		return FoundationLock{}, errors.New("unsupported foundation lock identity")
	}
	if lock.Metadata.Project == "" {
		return FoundationLock{}, errors.New("foundation lock metadata.project is required")
	}
	if err := validateIdentitySet(lock.Spec.Identities, nil, lock.Spec.Records.LockPath, lock.Spec.Records.ManifestPath, false); err != nil {
		return FoundationLock{}, fmt.Errorf("invalid foundation lock identities: %w", err)
	}
	if !lock.Spec.Distribution.Empty() {
		if problems := lock.Spec.Distribution.Validate(); len(problems) > 0 {
			return FoundationLock{}, fmt.Errorf("invalid foundation lock distribution: %s", strings.Join(problems, "; "))
		}
	}
	return lock, nil
}

func validateSnapshotPair(manifest Manifest, lock FoundationLock, lockData []byte) error {
	if manifest.Records.LockSHA256 != digest(lockData) {
		return errors.New("foundation lock digest contradicts the blueprint manifest")
	}
	if !equalIdentitySets(manifest.Identities, lock.Spec.Identities) {
		return errors.New("foundation lock and blueprint manifest identities contradict each other")
	}
	if !reflect.DeepEqual(manifest.Distribution, lock.Spec.Distribution) {
		return errors.New("foundation lock and blueprint manifest distributions contradict each other")
	}
	if lock.Metadata.Project != manifest.Identities.Snapshot.Project {
		return errors.New("foundation lock project contradicts the downstream snapshot identity")
	}
	if normalizedPath(lock.Spec.Records.LockPath) != normalizedPath(manifest.Records.LockPath) ||
		normalizedPath(lock.Spec.Records.ManifestPath) != normalizedPath(manifest.Records.ManifestPath) {
		return errors.New("foundation lock and blueprint manifest record paths contradict each other")
	}
	if err := validateIdentitySet(
		lock.Spec.Identities,
		manifest.Files,
		lock.Spec.Records.LockPath,
		lock.Spec.Records.ManifestPath,
		true,
	); err != nil {
		return fmt.Errorf("invalid foundation lock snapshot: %w", err)
	}
	return nil
}

func readRegularRecord(root *managedRoot, relative string) ([]byte, error) {
	if !safeRelativePath(relative) {
		return nil, errors.New("snapshot record path must be repository-relative")
	}
	data, exists, _, err := root.readFile(relative)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder, "$", nil); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func consumeJSONValue(decoder *json.Decoder, path string, first json.Token) error {
	token := first
	var err error
	if token == nil {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key at %s is not a string", path)
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON key %q at %s", key, path)
			}
			seen[key] = true
			if err := consumeJSONValue(decoder, path+"."+key, nil); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("unterminated JSON object at %s", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index), nil); err != nil {
				return err
			}
			index++
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("unterminated JSON array at %s", path)
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q at %s", delimiter, path)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values are not supported")
	}
	return err
}
