package blueprint

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"gopkg.in/yaml.v3"
)

type committedFoundation struct {
	Blueprint    *Document
	BlueprintSHA string
	Identity     FoundationIdentity
	Entries      []committedFile
	Presentation presentationSnapshot
}

type committedFile struct {
	Path    string
	Object  string
	Mode    fs.FileMode
	Type    string
	GitMode string
}

type foundationStoppedRefs struct {
	Root      string `yaml:"root"`
	Framework string `yaml:"framework"`
	Admin     string `yaml:"admin"`
	Frontend  string `yaml:"frontend"`
	Docs      string `yaml:"docs"`
	NPM       string `yaml:"npm"`
}

type foundationStoppedTrain struct {
	Version string                `yaml:"version"`
	Commit  string                `yaml:"commit"`
	Refs    foundationStoppedRefs `yaml:"refs"`
}

type foundationReleasePolicy struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Mode                      string                   `yaml:"mode"`
		ReleaseBranch             string                   `yaml:"releaseBranch"`
		RequireMergedPRSource     *bool                    `yaml:"requireMergedPullRequestSource"`
		CurrentStableVersion      string                   `yaml:"currentStableVersion"`
		CurrentStableCommit       string                   `yaml:"currentStableCommit"`
		NextPublicVersion         string                   `yaml:"nextPublicVersion"`
		DistributionVersion       string                   `yaml:"distributionVersion"`
		DistributionComponents    string                   `yaml:"distributionComponents"`
		ReleaseTargetState        string                   `yaml:"releaseTargetState"`
		ImmutableStoppedTrains    []foundationStoppedTrain `yaml:"immutableStoppedTrains"`
		PublicationWorkflowsReady *bool                    `yaml:"publicationWorkflowsReady"`
		DocsTagMutable            *bool                    `yaml:"docsTagMutable"`
		// Deprecated Docs revision fields remain readable for historical
		// Foundation policies. Current policies use DocsTagMutable instead.
		DocsRevisionPublicationReady *bool   `yaml:"docsRevisionPublicationReady"`
		DocsRevisionVersion          *string `yaml:"docsRevisionVersion"`
		DocsRevisionCommit           *string `yaml:"docsRevisionCommit"`
		StablePromotionReady         *bool   `yaml:"stablePromotionReady"`
		StablePromotionVersion       *string `yaml:"stablePromotionVersion"`
		StablePromotionCommit        *string `yaml:"stablePromotionCommit"`
		PublicPrereleases            *bool   `yaml:"publicPrereleases"`
		RootTagTemplate              string  `yaml:"rootTagTemplate"`
		FrameworkTagTemplate         string  `yaml:"frameworkTagTemplate"`
		AdminTagTemplate             string  `yaml:"adminTagTemplate"`
		FrontendTagTemplate          string  `yaml:"frontendTagTemplate"`
		FrontendV6TagTemplate        string  `yaml:"frontendV6TagTemplate"`
		DocsTagTemplate              string  `yaml:"docsTagTemplate"`
		NpmPackageTemplate           string  `yaml:"npmPackageTemplate"`
	} `yaml:"spec"`
}

func loadCommittedFoundation(ctx context.Context, root string, requested *Document) (committedFoundation, error) {
	if err := rejectDirtyTrackedFoundation(ctx, root); err != nil {
		return committedFoundation{}, err
	}
	commit, err := foundationCommit(ctx, root)
	if err != nil {
		return committedFoundation{}, err
	}
	timestamp, err := foundationTimestamp(ctx, root, commit)
	if err != nil {
		return committedFoundation{}, err
	}
	entries, err := committedFiles(ctx, root, commit)
	if err != nil {
		return committedFoundation{}, err
	}
	byPath := make(map[string]committedFile, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	blueprintPath := normalizedPath(requested.SourcePath)
	canonicalBlueprintPath := filepath.ToSlash(filepath.Join(blueprintDirectory, requested.Metadata.Name+".yaml"))
	if blueprintPath == "" {
		blueprintPath = canonicalBlueprintPath
	}
	if blueprintPath != canonicalBlueprintPath {
		return committedFoundation{}, fmt.Errorf("blueprint source path %q does not match canonical path %q", blueprintPath, canonicalBlueprintPath)
	}
	blueprintEntry, ok := byPath[blueprintPath]
	if !ok || blueprintEntry.Type != "blob" {
		return committedFoundation{}, fmt.Errorf("committed blueprint source is missing: %s", blueprintPath)
	}
	blueprintData, err := readCommittedBlob(ctx, root, blueprintEntry)
	if err != nil {
		return committedFoundation{}, err
	}
	committedBlueprint, err := decodeDocument(root, blueprintPath, blueprintData)
	if err != nil {
		return committedFoundation{}, fmt.Errorf("validate committed blueprint: %w", err)
	}
	if committedBlueprint.Metadata.Name != requested.Metadata.Name ||
		committedBlueprint.Metadata.Version != requested.Metadata.Version {
		return committedFoundation{}, errors.New("loaded blueprint identity does not match the committed blueprint object")
	}
	projectEntry, ok := byPath[".mss/project.yaml"]
	if !ok || projectEntry.Type != "blob" {
		return committedFoundation{}, errors.New("committed foundation is missing .mss/project.yaml")
	}
	projectData, err := readCommittedBlob(ctx, root, projectEntry)
	if err != nil {
		return committedFoundation{}, err
	}
	projectDocument, err := project.DecodeProjectDocument(projectData)
	if err != nil {
		return committedFoundation{}, fmt.Errorf("read committed foundation identity: %w", err)
	}
	repository := strings.TrimSpace(projectDocument.Metadata.Repository)
	if expected := foundationRepository(committedBlueprint); expected != "" && repository != expected {
		return committedFoundation{}, fmt.Errorf("project foundation repository %q contradicts blueprint source repository %q", repository, expected)
	}
	releaseEntry, ok := byPath[".mss/release-policy.yaml"]
	if !ok || releaseEntry.Type != "blob" {
		return committedFoundation{}, errors.New("committed foundation is missing .mss/release-policy.yaml")
	}
	releaseData, err := readCommittedBlob(ctx, root, releaseEntry)
	if err != nil {
		return committedFoundation{}, err
	}
	releasePolicy, err := decodeFoundationReleasePolicy(releaseData)
	if err != nil {
		return committedFoundation{}, err
	}
	version, channel, err := foundationReleaseVersion(ctx, root, commit, releasePolicy)
	if err != nil {
		return committedFoundation{}, err
	}
	identity := FoundationIdentity{
		Repository: repository,
		Version:    version,
		Commit:     commit,
		Timestamp:  timestamp,
		Channel:    channel,
		Source:     ".mss/release-policy.yaml",
	}
	presentation, err := loadPresentationSourceSnapshot(func(relative string) ([]byte, bool, error) {
		entry, exists := byPath[relative]
		if !exists {
			return nil, false, nil
		}
		if entry.Type != "blob" || entry.GitMode == "120000" {
			return nil, false, fmt.Errorf("committed Admin presentation source is not a regular blob: %s", relative)
		}
		data, err := readCommittedBlob(ctx, root, entry)
		return data, err == nil, err
	})
	if err != nil {
		return committedFoundation{}, err
	}
	if err := verifyFoundationHead(ctx, root, commit); err != nil {
		return committedFoundation{}, err
	}
	return committedFoundation{
		Blueprint:    committedBlueprint,
		BlueprintSHA: digest(blueprintData),
		Identity:     identity,
		Entries:      entries,
		Presentation: presentation,
	}, nil
}

func rejectDirtyTrackedFoundation(ctx context.Context, root string) error {
	command := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=no", "--ignore-submodules=none")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("inspect tracked foundation state: %w", err)
	}
	if len(output) != 0 {
		return errors.New("foundation checkout contains dirty tracked files; commit or restore them before resolving a downstream snapshot")
	}
	return nil
}

func committedFiles(ctx context.Context, root, commit string) ([]committedFile, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "ls-tree", "-rz", "--full-tree", commit)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list committed foundation files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	entries := make([]committedFile, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		tab := bytes.IndexByte(part, '\t')
		if tab < 0 {
			return nil, errors.New("git returned malformed committed file metadata")
		}
		fields := strings.Fields(string(part[:tab]))
		if len(fields) != 3 {
			return nil, errors.New("git returned malformed committed file identity")
		}
		relative := normalizedPath(string(part[tab+1:]))
		if !safeRelativePath(relative) {
			return nil, fmt.Errorf("git returned unsafe committed path %q", relative)
		}
		modeValue, err := strconv.ParseUint(fields[0], 8, 32)
		if err != nil {
			return nil, fmt.Errorf("parse committed mode for %s: %w", relative, err)
		}
		mode := fs.FileMode(modeValue & 0o777)
		if mode == 0 && fields[1] == "blob" {
			mode = 0o644
		}
		entries = append(entries, committedFile{
			Path:    relative,
			Object:  fields[2],
			Mode:    mode,
			Type:    fields[1],
			GitMode: fields[0],
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func readCommittedBlob(ctx context.Context, root string, entry committedFile) ([]byte, error) {
	if entry.Type != "blob" {
		return nil, fmt.Errorf("committed path is not a blob: %s", entry.Path)
	}
	command := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "blob", entry.Object)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read committed file %s: %w", entry.Path, err)
	}
	return output, nil
}

func readCommittedBlobs(ctx context.Context, root string, entries []committedFile) (map[string][]byte, error) {
	if len(entries) == 0 {
		return map[string][]byte{}, nil
	}
	var request strings.Builder
	for _, entry := range entries {
		if entry.Type != "blob" || entry.GitMode == "120000" {
			return nil, fmt.Errorf("committed path is not a regular blob: %s", entry.Path)
		}
		request.WriteString(entry.Object)
		request.WriteByte('\n')
	}
	command := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(request.String())
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open committed blob stream: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start committed blob stream: %w", err)
	}
	reader := bufio.NewReader(stdout)
	result := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		header, err := reader.ReadString('\n')
		if err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("read committed blob header for %s: %w", entry.Path, err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[0] != entry.Object || fields[1] != "blob" {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("git returned contradictory committed blob metadata for %s", entry.Path)
		}
		size, ok := parseCommittedBlobSize(fields[2])
		if !ok {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("git returned invalid committed blob size for %s", entry.Path)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(reader, data); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("read committed blob %s: %w", entry.Path, err)
		}
		terminator, err := reader.ReadByte()
		if err != nil || terminator != '\n' {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("git returned malformed committed blob terminator for %s", entry.Path)
		}
		result[entry.Path] = data
	}
	if err := command.Wait(); err != nil {
		return nil, fmt.Errorf("read committed foundation blobs: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return result, nil
}

func parseCommittedBlobSize(value string) (int, bool) {
	size, err := strconv.ParseInt(value, 10, strconv.IntSize)
	if err != nil || size < 0 {
		return 0, false
	}
	return int(size), true
}

func foundationCommit(ctx context.Context, root string) (string, error) {
	commitCommand := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD^{commit}")
	commitOutput, err := commitCommand.Output()
	if err != nil {
		return "", fmt.Errorf("resolve foundation commit: %w", err)
	}
	commit := strings.ToLower(strings.TrimSpace(string(commitOutput)))
	if !fullCommitPattern.MatchString(commit) {
		return "", errors.New("foundation commit is not a full 40-character hexadecimal commit")
	}
	return commit, nil
}

func foundationTimestamp(ctx context.Context, root, commit string) (string, error) {
	timeCommand := exec.CommandContext(ctx, "git", "-C", root, "show", "-s", "--format=%cI", commit)
	timeOutput, err := timeCommand.Output()
	if err != nil {
		return "", fmt.Errorf("resolve foundation timestamp: %w", err)
	}
	timestamp := strings.TrimSpace(string(timeOutput))
	if timestamp == "" {
		return "", errors.New("foundation timestamp is empty")
	}
	return timestamp, nil
}

func verifyFoundationHead(ctx context.Context, root, commit string) error {
	current, err := foundationCommit(ctx, root)
	if err != nil {
		return err
	}
	if current != commit {
		return errors.New("foundation HEAD changed while the committed snapshot source was being resolved")
	}
	return rejectDirtyTrackedFoundation(ctx, root)
}

func decodeFoundationReleasePolicy(data []byte) (foundationReleasePolicy, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return foundationReleasePolicy{}, fmt.Errorf("parse committed foundation release policy: %w", err)
	}
	policy := foundationReleasePolicy{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&policy); err != nil {
		return foundationReleasePolicy{}, fmt.Errorf("parse committed foundation release policy: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return foundationReleasePolicy{}, errors.New("parse committed foundation release policy: multiple YAML documents are not supported")
		}
		return foundationReleasePolicy{}, fmt.Errorf("parse committed foundation release policy: %w", err)
	}
	if policy.APIVersion != "mss.io/v1alpha1" || policy.Kind != "ReleasePolicy" {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy must be mss.io/v1alpha1 ReleasePolicy")
	}
	if strings.TrimSpace(policy.Metadata.Name) == "" {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy metadata.name is required")
	}
	if policy.Spec.Mode != "development-first" {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy mode must equal development-first")
	}
	extendedReleaseContract := strings.TrimSpace(policy.Spec.ReleaseTargetState) != "" ||
		len(policy.Spec.ImmutableStoppedTrains) != 0 ||
		policy.Spec.DocsTagMutable != nil ||
		policy.Spec.DocsRevisionPublicationReady != nil ||
		strings.TrimSpace(policy.Spec.NpmPackageTemplate) != ""
	lifecycleAuthorizationContract := policy.Spec.StablePromotionReady != nil ||
		policy.Spec.StablePromotionVersion != nil ||
		policy.Spec.StablePromotionCommit != nil
	legacyDocsRevisionContract := policy.Spec.DocsRevisionPublicationReady != nil ||
		policy.Spec.DocsRevisionVersion != nil ||
		policy.Spec.DocsRevisionCommit != nil
	if policy.Spec.PublicationWorkflowsReady == nil || policy.Spec.PublicPrereleases == nil ||
		(extendedReleaseContract && policy.Spec.DocsTagMutable == nil && policy.Spec.DocsRevisionPublicationReady == nil) ||
		(lifecycleAuthorizationContract && (policy.Spec.StablePromotionReady == nil ||
			policy.Spec.StablePromotionVersion == nil ||
			policy.Spec.StablePromotionCommit == nil)) ||
		(legacyDocsRevisionContract && policy.Spec.DocsRevisionPublicationReady == nil) {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy boolean controls are required")
	}
	if policy.Spec.DocsTagMutable != nil {
		if !*policy.Spec.DocsTagMutable {
			return foundationReleasePolicy{}, errors.New("committed foundation docsTagMutable must remain true")
		}
		if legacyDocsRevisionContract {
			return foundationReleasePolicy{}, errors.New("committed foundation policy cannot mix mutable Docs tags with legacy Docs revisions")
		}
	}
	currentRaw := strings.TrimSpace(policy.Spec.CurrentStableVersion)
	nextRaw := strings.TrimSpace(policy.Spec.NextPublicVersion)
	distributionRaw := strings.TrimSpace(policy.Spec.DistributionVersion)
	current := strings.TrimPrefix(currentRaw, "v")
	next := strings.TrimPrefix(nextRaw, "v")
	distribution := strings.TrimPrefix(distributionRaw, "v")
	if !strings.HasPrefix(currentRaw, "v") || !strings.HasPrefix(nextRaw, "v") || !strings.HasPrefix(distributionRaw, "v") {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy versions must use a v prefix")
	}
	if !validSemanticVersion(current) || !validSemanticVersion(next) || !validSemanticVersion(distribution) {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy versions must be semantic")
	}
	if strings.Contains(current, "-") {
		return foundationReleasePolicy{}, errors.New("committed foundation currentStableVersion must not be a prerelease")
	}
	if strings.Contains(next, "-") && !*policy.Spec.PublicPrereleases {
		return foundationReleasePolicy{}, errors.New("committed foundation publicPrereleases must be true for a prerelease target")
	}
	if distributionRaw != nextRaw {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy distributionVersion must equal nextPublicVersion")
	}
	if lifecycleAuthorizationContract {
		promotionRaw := strings.TrimSpace(*policy.Spec.StablePromotionVersion)
		promotionCommit := strings.TrimSpace(*policy.Spec.StablePromotionCommit)
		if promotionRaw == "" || promotionCommit == "" {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy lifecycle authorization fields are required")
		}
		if !strings.HasPrefix(promotionRaw, "v") || !validSemanticVersion(strings.TrimPrefix(promotionRaw, "v")) {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy stablePromotionVersion must be semantic")
		}
		if promotionRaw != nextRaw {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy stablePromotionVersion must equal nextPublicVersion")
		}
		if *policy.Spec.StablePromotionReady {
			if promotionRaw == currentRaw {
				return foundationReleasePolicy{}, errors.New("committed foundation stable promotion authorization is already consumed because the target is current stable")
			}
			if !*policy.Spec.PublicationWorkflowsReady {
				return foundationReleasePolicy{}, errors.New("committed foundation publication workflows must remain ready during stable promotion")
			}
			if !fullCommitPattern.MatchString(promotionCommit) {
				return foundationReleasePolicy{}, errors.New("committed foundation stablePromotionCommit must be a full commit when promotion is ready")
			}
		} else if promotionCommit != "disabled" {
			return foundationReleasePolicy{}, errors.New("committed foundation stablePromotionCommit must be disabled until promotion is ready")
		}
		*policy.Spec.StablePromotionVersion = promotionRaw
		*policy.Spec.StablePromotionCommit = promotionCommit
	}

	if legacyDocsRevisionContract {
		if policy.Spec.DocsRevisionVersion == nil || policy.Spec.DocsRevisionCommit == nil {
			if *policy.Spec.DocsRevisionPublicationReady {
				return foundationReleasePolicy{}, errors.New("committed foundation legacy Docs revision fields are required when publication is ready")
			}
		} else {
			docsRevisionVersion := strings.TrimSpace(*policy.Spec.DocsRevisionVersion)
			docsRevisionCommit := strings.TrimSpace(*policy.Spec.DocsRevisionCommit)
			if *policy.Spec.DocsRevisionPublicationReady {
				base, revision, ok := strings.Cut(docsRevisionVersion, "+docs.")
				revisionNumber, revisionErr := strconv.Atoi(revision)
				if !ok || base != currentRaw || revisionErr != nil || revisionNumber < 1 || revisionNumber > 999 || strconv.Itoa(revisionNumber) != revision {
					return foundationReleasePolicy{}, errors.New("committed foundation docsRevisionVersion must be the current stable vX.Y.Z+docs.N revision from 1 through 999")
				}
				if !fullCommitPattern.MatchString(docsRevisionCommit) {
					return foundationReleasePolicy{}, errors.New("committed foundation docsRevisionCommit must be a full commit when publication is ready")
				}
			} else if docsRevisionVersion != "disabled" || docsRevisionCommit != "disabled" {
				return foundationReleasePolicy{}, errors.New("committed foundation docs revision authorization must be disabled until publication is ready")
			}
			*policy.Spec.DocsRevisionVersion = docsRevisionVersion
			*policy.Spec.DocsRevisionCommit = docsRevisionCommit
		}
	}
	targetState := strings.TrimSpace(policy.Spec.ReleaseTargetState)
	if extendedReleaseContract {
		if targetState != "active" && targetState != "stopped" {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy releaseTargetState must equal active or stopped")
		}
		if strings.Count(policy.Spec.NpmPackageTemplate, "{npmVersion}") != 1 {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy npmPackageTemplate must contain exactly one {npmVersion} placeholder")
		}
	}
	if strings.TrimSpace(policy.Spec.DistributionComponents) != "root,framework,admin,frontend" {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy distributionComponents must equal root,framework,admin,frontend")
	}
	policy.Spec.CurrentStableCommit = strings.ToLower(strings.TrimSpace(policy.Spec.CurrentStableCommit))
	if !fullCommitPattern.MatchString(policy.Spec.CurrentStableCommit) {
		return foundationReleasePolicy{}, errors.New("committed foundation release policy currentStableCommit must be a full commit")
	}
	templates := []struct {
		name  string
		value string
	}{
		{name: "rootTagTemplate", value: policy.Spec.RootTagTemplate},
		{name: "frameworkTagTemplate", value: policy.Spec.FrameworkTagTemplate},
		{name: "adminTagTemplate", value: policy.Spec.AdminTagTemplate},
		{name: "frontendTagTemplate", value: policy.Spec.FrontendTagTemplate},
		{name: "docsTagTemplate", value: policy.Spec.DocsTagTemplate},
	}
	if policy.Spec.FrontendV6TagTemplate != "" {
		templates = append(templates, struct {
			name  string
			value string
		}{name: "frontendV6TagTemplate", value: policy.Spec.FrontendV6TagTemplate})
	}
	for _, template := range templates {
		if strings.Count(template.value, "{version}") != 1 {
			return foundationReleasePolicy{}, fmt.Errorf("committed foundation release policy %s must contain exactly one {version} placeholder", template.name)
		}
	}
	if extendedReleaseContract {
		stoppedVersions, err := validateFoundationStoppedTrains(policy)
		if err != nil {
			return foundationReleasePolicy{}, err
		}
		_, targetIsStopped := stoppedVersions[nextRaw]
		if targetState == "stopped" && !targetIsStopped {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy stopped target must belong to immutableStoppedTrains")
		}
		if targetState == "active" && targetIsStopped {
			return foundationReleasePolicy{}, errors.New("committed foundation release policy active target must not select an immutable stopped train")
		}
	}
	return policy, nil
}

func foundationStoppedRefMap(refs foundationStoppedRefs) map[string]string {
	return map[string]string{
		"root":      refs.Root,
		"framework": refs.Framework,
		"admin":     refs.Admin,
		"frontend":  refs.Frontend,
		"docs":      refs.Docs,
		"npm":       refs.NPM,
	}
}

func foundationPolicyReleaseRef(policy foundationReleasePolicy, component, version string) string {
	var template string
	var placeholder string
	switch component {
	case "root":
		template, placeholder = policy.Spec.RootTagTemplate, "{version}"
	case "framework":
		template, placeholder = policy.Spec.FrameworkTagTemplate, "{version}"
	case "admin":
		template, placeholder = policy.Spec.AdminTagTemplate, "{version}"
	case "frontend":
		template, placeholder = policy.Spec.FrontendTagTemplate, "{version}"
	case "docs":
		template, placeholder = policy.Spec.DocsTagTemplate, "{version}"
	case "npm":
		template, placeholder = policy.Spec.NpmPackageTemplate, "{npmVersion}"
		version = strings.TrimPrefix(version, "v")
	default:
		return ""
	}
	return strings.Replace(template, placeholder, version, 1)
}

func validateFoundationStoppedTrains(policy foundationReleasePolicy) (map[string]foundationStoppedTrain, error) {
	if len(policy.Spec.ImmutableStoppedTrains) == 0 {
		return nil, errors.New("committed foundation release policy immutableStoppedTrains must be a non-empty YAML list")
	}

	stopped := make(map[string]foundationStoppedTrain, len(policy.Spec.ImmutableStoppedTrains))
	seenRefs := make(map[string]string, len(policy.Spec.ImmutableStoppedTrains)*6)
	for index, train := range policy.Spec.ImmutableStoppedTrains {
		version := strings.TrimSpace(train.Version)
		semanticVersion := strings.TrimPrefix(version, "v")
		if !strings.HasPrefix(version, "v") || !validSemanticVersion(semanticVersion) {
			return nil, fmt.Errorf("committed foundation immutable stopped train %d version must be a v-prefixed semantic version", index)
		}
		if _, exists := stopped[version]; exists {
			return nil, fmt.Errorf("committed foundation release policy immutableStoppedTrains duplicates version %s", version)
		}
		commit := strings.TrimSpace(train.Commit)
		if !fullCommitPattern.MatchString(commit) {
			return nil, fmt.Errorf("committed foundation immutable stopped train %s commit must be a full commit", version)
		}

		refs := foundationStoppedRefMap(train.Refs)
		for _, component := range []string{"root", "framework", "admin", "frontend", "docs", "npm"} {
			publicRef := strings.TrimSpace(refs[component])
			if publicRef == "" {
				return nil, fmt.Errorf("committed foundation immutable stopped train %s is missing %s ref", version, component)
			}
			expected := foundationPolicyReleaseRef(policy, component, version)
			if publicRef != expected {
				return nil, fmt.Errorf("committed foundation immutable stopped train %s %s ref must remain %q", version, component, expected)
			}
			if owner, exists := seenRefs[publicRef]; exists {
				return nil, fmt.Errorf("committed foundation immutable stopped ref %q is duplicated by %s and %s", publicRef, version, owner)
			}
			seenRefs[publicRef] = version
		}
		train.Version = version
		train.Commit = commit
		stopped[version] = train
	}

	required := map[string]foundationStoppedTrain{
		"v1.3.5": {
			Version: "v1.3.5",
			Commit:  "396f60615cdfa589353b16ef9d3531e249e65432",
			Refs: foundationStoppedRefs{
				Root: "v1.3.5", Framework: "mss-boot/v1.3.5", Admin: "admin/v1.3.5",
				Frontend: "web/antd-v6/v1.3.5", Docs: "docs/v1.3.5", NPM: "@mss-boot-io/admin-web@1.3.5",
			},
		},
		"v1.3.6": {
			Version: "v1.3.6",
			Commit:  "b1fe47a3a83209574e09d53526b122dd2cbc5277",
			Refs: foundationStoppedRefs{
				Root: "v1.3.6", Framework: "mss-boot/v1.3.6", Admin: "admin/v1.3.6",
				Frontend: "web/antd-v6/v1.3.6", Docs: "docs/v1.3.6", NPM: "@mss-boot-io/admin-web@1.3.6",
			},
		},
	}
	for version, expected := range required {
		actual, exists := stopped[version]
		if !exists || actual.Commit != expected.Commit {
			return nil, fmt.Errorf("committed foundation release policy must preserve the exact %s stopped train permanently", version)
		}
		actualRefs, expectedRefs := foundationStoppedRefMap(actual.Refs), foundationStoppedRefMap(expected.Refs)
		for component, expectedRef := range expectedRefs {
			if actualRefs[component] != expectedRef {
				return nil, fmt.Errorf("committed foundation release policy must preserve the exact %s stopped train permanently", version)
			}
		}
	}
	return stopped, nil
}

func foundationReleaseVersion(ctx context.Context, root, commit string, policy foundationReleasePolicy) (string, string, error) {
	current := strings.TrimPrefix(strings.TrimSpace(policy.Spec.CurrentStableVersion), "v")
	next := strings.TrimPrefix(strings.TrimSpace(policy.Spec.NextPublicVersion), "v")
	if commit == policy.Spec.CurrentStableCommit {
		return current, "stable", nil
	}
	tag := strings.Replace(policy.Spec.RootTagTemplate, "{version}", strings.TrimSpace(policy.Spec.NextPublicVersion), 1)
	ref := "refs/tags/" + tag
	if err := exec.CommandContext(ctx, "git", "-C", root, "check-ref-format", ref).Run(); err != nil {
		return "", "", fmt.Errorf("validate foundation release tag %q: %w", tag, err)
	}
	show := exec.CommandContext(ctx, "git", "-C", root, "show-ref", "--verify", "--quiet", ref)
	if err := show.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return next, "candidate", nil
		}
		return "", "", fmt.Errorf("inspect foundation release tag %q: %w", tag, err)
	}
	resolve := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", ref+"^{commit}")
	output, err := resolve.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve foundation release tag %q: %w", tag, err)
	}
	tagCommit := strings.ToLower(strings.TrimSpace(string(output)))
	if !fullCommitPattern.MatchString(tagCommit) {
		return "", "", fmt.Errorf("foundation release tag %q did not resolve to a full commit", tag)
	}
	if tagCommit == commit {
		if strings.Contains(next, "-") {
			// A published prerelease is an immutable release candidate, but it
			// must not be promoted to the stable Foundation channel merely
			// because its exact tag exists.
			return next, "candidate", nil
		}
		return next, "stable", nil
	}
	return next, "candidate", nil
}
