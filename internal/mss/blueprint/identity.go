package blueprint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	pathpkg "path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	snapshotAPIVersion = "mss.io/v1alpha2"
	legacyAPIVersion   = "mss.io/v1alpha1"
)

var (
	semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	fullCommitPattern      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Pattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// FoundationIdentity identifies the exact Foundation source release target.
// Version and channel come from the committed release policy (and an exact
// tag, when present), never from the Project's historical generation baseline.
type FoundationIdentity struct {
	Repository string `json:"repository" yaml:"repository"`
	Version    string `json:"version" yaml:"version"`
	Commit     string `json:"commit" yaml:"commit"`
	Timestamp  string `json:"timestamp" yaml:"timestamp"`
	Channel    string `json:"channel" yaml:"channel"`
	Source     string `json:"source" yaml:"source"`
}

// BlueprintIdentity identifies one committed Blueprint revision and its exact
// source document bytes.
type BlueprintIdentity struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	SHA256  string `json:"sha256" yaml:"sha256"`
}

// GeneratorIdentity identifies the binary that resolved and rendered a
// downstream snapshot.
type GeneratorIdentity struct {
	Tool    string `json:"tool" yaml:"tool"`
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit,omitempty" yaml:"commit,omitempty"`
}

// DownstreamSnapshotIdentity identifies the application and the deterministic
// digest of its resolved inputs and ordinary managed baseline. It deliberately
// has no semantic version of its own.
type DownstreamSnapshotIdentity struct {
	Project    string `json:"project" yaml:"project"`
	Module     string `json:"module" yaml:"module"`
	Repository string `json:"repository" yaml:"repository"`
	SHA256     string `json:"sha256" yaml:"sha256"`
}

// IdentitySet keeps the four independently sourced identities together in
// both snapshot records.
type IdentitySet struct {
	Foundation FoundationIdentity         `json:"foundation" yaml:"foundation"`
	Blueprint  BlueprintIdentity          `json:"blueprint" yaml:"blueprint"`
	Generator  GeneratorIdentity          `json:"generator" yaml:"generator"`
	Snapshot   DownstreamSnapshotIdentity `json:"snapshot" yaml:"snapshot"`
}

type snapshotDigestInput struct {
	Application snapshotApplication  `json:"application"`
	Foundation  FoundationIdentity   `json:"foundation"`
	Blueprint   BlueprintIdentity    `json:"blueprint"`
	Generator   GeneratorIdentity    `json:"generator"`
	Files       []snapshotDigestFile `json:"files"`
}

type snapshotApplication struct {
	Project    string `json:"project"`
	Module     string `json:"module"`
	Repository string `json:"repository"`
}

type snapshotDigestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
}

func computeSnapshotDigest(identities IdentitySet, files map[string]ManifestFile) (string, error) {
	paths := make([]string, 0, len(files))
	for relative := range files {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	input := snapshotDigestInput{
		Application: snapshotApplication{
			Project:    identities.Snapshot.Project,
			Module:     identities.Snapshot.Module,
			Repository: identities.Snapshot.Repository,
		},
		Foundation: identities.Foundation,
		Blueprint:  identities.Blueprint,
		Generator:  identities.Generator,
		Files:      make([]snapshotDigestFile, 0, len(paths)),
	}
	for _, relative := range paths {
		file := files[relative]
		input.Files = append(input.Files, snapshotDigestFile{
			Path:   relative,
			SHA256: file.SHA256,
			Mode:   uint32(file.Mode.Perm()),
			Size:   file.Size,
		})
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal downstream snapshot identity: %w", err)
	}
	return digest(data), nil
}

func validateIdentitySet(identities IdentitySet, files map[string]ManifestFile, lockPath, manifestPath string, requireBaseline bool) error {
	var problems []string
	if !repositoryPattern.MatchString(identities.Foundation.Repository) {
		problems = append(problems, "foundation repository must use owner/name form")
	}
	if !validSemanticVersion(identities.Foundation.Version) {
		problems = append(problems, "foundation version must be semantic")
	}
	if !fullCommitPattern.MatchString(identities.Foundation.Commit) {
		problems = append(problems, "foundation commit must be a full 40-character hexadecimal commit")
	}
	if _, err := time.Parse(time.RFC3339, identities.Foundation.Timestamp); err != nil {
		problems = append(problems, "foundation timestamp must be RFC3339")
	}
	if identities.Foundation.Channel != "candidate" && identities.Foundation.Channel != "stable" {
		problems = append(problems, "foundation channel must equal candidate or stable")
	}
	if identities.Foundation.Source != ".mss/release-policy.yaml" {
		problems = append(problems, "foundation identity source must equal .mss/release-policy.yaml")
	}
	if !blueprintNamePattern.MatchString(identities.Blueprint.Name) {
		problems = append(problems, "blueprint name must be lower-case kebab-case")
	}
	if !validSemanticVersion(identities.Blueprint.Version) {
		problems = append(problems, "blueprint version must be semantic")
	}
	if !sha256Pattern.MatchString(identities.Blueprint.SHA256) {
		problems = append(problems, "blueprint digest must be a SHA-256 digest")
	}
	if identities.Generator.Tool != "mss" {
		problems = append(problems, "generator tool must equal mss")
	}
	if strings.TrimSpace(identities.Generator.Version) == "" {
		problems = append(problems, "generator version is required")
	}
	if identities.Generator.Commit != "" && !fullCommitPattern.MatchString(identities.Generator.Commit) {
		problems = append(problems, "generator commit must be empty or a full 40-character hexadecimal commit")
	}
	application := Application{
		Name:       identities.Snapshot.Project,
		Module:     identities.Snapshot.Module,
		Repository: identities.Snapshot.Repository,
		// ValidateApplication requires a display name, but display text is not
		// part of the stable downstream identity.
		DisplayName: identities.Snapshot.Project,
	}
	if err := ValidateApplication(application); err != nil {
		problems = append(problems, "snapshot application identity is invalid: "+err.Error())
	}
	if !sha256Pattern.MatchString(identities.Snapshot.SHA256) {
		problems = append(problems, "snapshot digest must be a SHA-256 digest")
	}
	if !safeRelativePath(lockPath) || !safeRelativePath(manifestPath) {
		problems = append(problems, "snapshot record paths must be repository-relative")
	} else if normalizedPath(lockPath) == normalizedPath(manifestPath) {
		problems = append(problems, "snapshot lock and manifest paths must be distinct")
	}
	if lockPath != normalizedPath(lockPath) || manifestPath != normalizedPath(manifestPath) {
		problems = append(problems, "snapshot record paths must use canonical slash-separated form")
	}
	if requireBaseline && len(files) == 0 {
		problems = append(problems, "managed snapshot baseline must not be empty")
	}
	for relative, file := range files {
		if !safeRelativePath(relative) {
			problems = append(problems, "managed snapshot path is unsafe: "+relative)
		}
		if relative != normalizedPath(relative) {
			problems = append(problems, "managed snapshot path is not canonical: "+relative)
		}
		if normalizedPath(relative) == normalizedPath(lockPath) || normalizedPath(relative) == normalizedPath(manifestPath) {
			problems = append(problems, "snapshot records must not be included in the ordinary managed baseline: "+relative)
		}
		if !sha256Pattern.MatchString(file.SHA256) {
			problems = append(problems, "managed snapshot digest is invalid: "+relative)
		}
		if file.Size < 0 {
			problems = append(problems, "managed snapshot size is negative: "+relative)
		}
		if file.Mode.Perm() == 0 || file.Mode&^fs.ModePerm != 0 {
			problems = append(problems, "managed snapshot mode is invalid: "+relative)
		}
	}
	if requireBaseline {
		computed, err := computeSnapshotDigest(identities, files)
		if err != nil {
			problems = append(problems, err.Error())
		} else if identities.Snapshot.SHA256 != computed {
			problems = append(problems, "snapshot digest does not match the resolved identities and managed baseline")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func equalIdentitySets(left, right IdentitySet) bool {
	return reflect.DeepEqual(left, right)
}

func validSemanticVersion(value string) bool {
	return semanticVersionPattern.MatchString(strings.TrimSpace(value))
}

func normalizedPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return pathpkg.Clean(value)
}
