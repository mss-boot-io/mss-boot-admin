package blueprint

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// SnapshotRole distinguishes a generated downstream snapshot from the
// development-only lock sentinel kept by the Foundation source repository.
type SnapshotRole string

const (
	SnapshotRoleGenerated        SnapshotRole = "generated-downstream"
	SnapshotRoleFoundationSource SnapshotRole = "foundation-source"
)

// SnapshotStatus preserves the historical flat manifest metadata fields for
// JSON consumers while exposing the four independent identities and the two
// cross-digested snapshot records.
type SnapshotStatus struct {
	ManifestMetadata
	Identities IdentitySet     `json:"identities"`
	Records    ManifestRecords `json:"records"`
}

// FoundationSourceStatus is the deliberately limited identity carried by the
// Foundation repository's legacy development lock sentinel. It is not a
// generated downstream snapshot and cannot be used as an upgrade baseline.
type FoundationSourceStatus struct {
	Project              string `json:"project"`
	FoundationRepository string `json:"foundationRepository"`
	FoundationVersion    string `json:"foundationVersion"`
	Blueprint            string `json:"blueprint"`
	BlueprintVersion     string `json:"blueprintVersion"`
	GeneratorVersion     string `json:"generatorVersion"`
}

// SnapshotInspection is used by repository readiness checks that must accept
// the Foundation source sentinel but require a strict current snapshot in a
// generated downstream repository.
type SnapshotInspection struct {
	Role   SnapshotRole            `json:"role"`
	Status *SnapshotStatus         `json:"status,omitempty"`
	Source *FoundationSourceStatus `json:"source,omitempty"`
}

// ReadSnapshotStatus returns a status projection of one strictly verified
// current snapshot. Legacy manifests remain upgrade inputs only.
func ReadSnapshotStatus(root, manifestRelative string) (SnapshotStatus, error) {
	snapshot, err := ReadSnapshot(root, manifestRelative)
	if err != nil {
		return SnapshotStatus{}, err
	}
	return statusFromSnapshot(snapshot), nil
}

func statusFromSnapshot(snapshot Snapshot) SnapshotStatus {
	return SnapshotStatus{
		ManifestMetadata: snapshot.Manifest.Metadata,
		Identities:       snapshot.Manifest.Identities,
		Records:          snapshot.Manifest.Records,
	}
}

// ValidateProjectIdentity cross-checks only the downstream project fields
// owned by project metadata. spec.foundationVersion is a historical generation
// baseline and spec.backend.module may identify a nested deployable module, so
// neither is part of this comparison.
func (status SnapshotStatus) ValidateProjectIdentity(name, repository string) error {
	name = strings.TrimSpace(name)
	repository = strings.TrimSpace(repository)
	var problems []string
	if status.Identities.Snapshot.Project != name {
		problems = append(problems, fmt.Sprintf(
			"project metadata.name %q contradicts snapshot project %q",
			name,
			status.Identities.Snapshot.Project,
		))
	}
	if status.Identities.Snapshot.Repository != repository {
		problems = append(problems, fmt.Sprintf(
			"project metadata.repository %q contradicts snapshot repository %q",
			repository,
			status.Identities.Snapshot.Repository,
		))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// InspectSnapshot classifies repository snapshot state for doctor. A manifest
// always selects the strict current reader; it can never fall back to the
// legacy source sentinel. With no manifest, only a complete and strictly
// decoded v1alpha1 development lock is accepted as Foundation source state.
func InspectSnapshot(root, manifestRelative, projectName, projectRepository string) (SnapshotInspection, error) {
	if strings.TrimSpace(manifestRelative) == "" {
		manifestRelative = ".mss/blueprint-manifest.json"
	}
	manifestRelative = normalizedPath(manifestRelative)
	if !safeRelativePath(manifestRelative) {
		return SnapshotInspection{}, errors.New("manifest path must be repository-relative")
	}

	managed, err := openManagedRoot(root, false)
	if err != nil {
		return SnapshotInspection{}, err
	}
	defer managed.Close()
	release, err := acquireSnapshotReader(managed)
	if err != nil {
		return SnapshotInspection{}, err
	}
	defer release()
	if err := managed.checkSnapshotTransaction(); err != nil {
		return SnapshotInspection{}, err
	}

	_, manifestExists, _, manifestErr := managed.readFile(manifestRelative)
	if manifestErr != nil {
		return SnapshotInspection{}, fmt.Errorf("inspect blueprint manifest: %w", manifestErr)
	}
	if manifestExists {
		snapshot, err := readSnapshotUnlocked(managed, manifestRelative)
		if err != nil {
			return SnapshotInspection{}, err
		}
		status := statusFromSnapshot(snapshot)
		if err := status.ValidateProjectIdentity(projectName, projectRepository); err != nil {
			return SnapshotInspection{}, err
		}
		return SnapshotInspection{Role: SnapshotRoleGenerated, Status: &status}, nil
	}

	lockData, lockExists, _, lockErr := managed.readFile(".mss/lock.yaml")
	if lockErr != nil {
		return SnapshotInspection{}, fmt.Errorf("inspect foundation lock: %w", lockErr)
	}
	if !lockExists {
		return SnapshotInspection{}, errors.New("snapshot records are missing: neither a current manifest nor a Foundation source lock exists")
	}
	source, err := decodeFoundationSourceSentinel(lockData, projectName, projectRepository)
	if err != nil {
		return SnapshotInspection{}, fmt.Errorf("orphan or malformed snapshot lock: %w", err)
	}
	return SnapshotInspection{Role: SnapshotRoleFoundationSource, Source: &source}, nil
}

type foundationSourceLock struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Project string `yaml:"project"`
	} `yaml:"metadata"`
	Spec struct {
		Foundation struct {
			Repository string `yaml:"repository"`
			Version    string `yaml:"version"`
			Channel    string `yaml:"channel"`
		} `yaml:"foundation"`
		Blueprint struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"blueprint"`
		Contracts   map[string]string `yaml:"contracts"`
		GeneratedBy struct {
			Tool    string `yaml:"tool"`
			Version string `yaml:"version"`
		} `yaml:"generatedBy"`
		Modules  map[string]any `yaml:"modules"`
		Upgrades []any          `yaml:"upgrades"`
	} `yaml:"spec"`
}

func decodeFoundationSourceSentinel(data []byte, projectName, projectRepository string) (FoundationSourceStatus, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return FoundationSourceStatus{}, fmt.Errorf("parse Foundation source lock: %w", err)
	}
	lock := foundationSourceLock{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&lock); err != nil {
		return FoundationSourceStatus{}, fmt.Errorf("parse Foundation source lock: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return FoundationSourceStatus{}, errors.New("parse Foundation source lock: multiple YAML documents are not supported")
		}
		return FoundationSourceStatus{}, fmt.Errorf("parse Foundation source lock: %w", err)
	}

	projectName = strings.TrimSpace(projectName)
	projectRepository = strings.TrimSpace(projectRepository)
	var problems []string
	if lock.APIVersion != legacyAPIVersion || lock.Kind != lockKind {
		problems = append(problems, "orphan snapshot lock is not the mss.io/v1alpha1 Foundation source sentinel")
	}
	if lock.Metadata.Project == "" || lock.Metadata.Project != projectName {
		problems = append(problems, "Foundation source lock project contradicts project metadata.name")
	}
	if !repositoryPattern.MatchString(lock.Spec.Foundation.Repository) || lock.Spec.Foundation.Repository != projectRepository {
		problems = append(problems, "Foundation source lock repository contradicts project metadata.repository")
	}
	if !validSemanticVersion(lock.Spec.Foundation.Version) {
		problems = append(problems, "Foundation source lock version must be semantic")
	}
	if lock.Spec.Foundation.Channel != "development" {
		problems = append(problems, "legacy Foundation source lock channel must equal development")
	}
	if !blueprintNamePattern.MatchString(lock.Spec.Blueprint.Name) || !validSemanticVersion(lock.Spec.Blueprint.Version) {
		problems = append(problems, "Foundation source Blueprint identity is invalid")
	}
	if lock.Spec.GeneratedBy.Tool != "mss" || strings.TrimSpace(lock.Spec.GeneratedBy.Version) == "" {
		problems = append(problems, "Foundation source generator identity is invalid")
	}
	if len(lock.Spec.Contracts) == 0 {
		problems = append(problems, "Foundation source lock contracts must not be empty")
	}
	if lock.Spec.Modules == nil || lock.Spec.Upgrades == nil {
		problems = append(problems, "Foundation source lock modules and upgrades sentinels are required")
	}
	if len(problems) > 0 {
		return FoundationSourceStatus{}, errors.New(strings.Join(problems, "; "))
	}
	return FoundationSourceStatus{
		Project:              lock.Metadata.Project,
		FoundationRepository: lock.Spec.Foundation.Repository,
		FoundationVersion:    lock.Spec.Foundation.Version,
		Blueprint:            lock.Spec.Blueprint.Name,
		BlueprintVersion:     lock.Spec.Blueprint.Version,
		GeneratorVersion:     lock.Spec.GeneratedBy.Version,
	}, nil
}
