package blueprint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

// Action describes one downstream file operation.
type Action string

const (
	ActionCreate    Action = "create"
	ActionUnchanged Action = "unchanged"
	ActionConflict  Action = "conflict"
)

// Options controls application generation.
type Options struct {
	FoundationRoot      string
	Blueprint           string
	Destination         string
	Application         Application
	FrontendRegistryURL string
	Write               bool
	InitializeGit       bool
}

// FileChange is one deterministic output file decision.
type FileChange struct {
	Path   string      `json:"path"`
	Action Action      `json:"action"`
	Mode   fs.FileMode `json:"mode"`
	Size   int64       `json:"size"`
	SHA256 string      `json:"sha256"`
	Detail string      `json:"detail,omitempty"`
}

// Plan is returned in dry-run and write modes.
type Plan struct {
	Blueprint        string                   `json:"blueprint"`
	BlueprintVersion string                   `json:"blueprintVersion"`
	FoundationCommit string                   `json:"foundationCommit"`
	Identities       IdentitySet              `json:"identities"`
	Distribution     project.DistributionSpec `json:"distribution,omitempty"`
	Application      Application              `json:"application"`
	Destination      string                   `json:"destination"`
	DryRun           bool                     `json:"dryRun"`
	Success          bool                     `json:"success"`
	TotalFiles       int                      `json:"totalFiles"`
	TotalBytes       int64                    `json:"totalBytes"`
	Changes          []FileChange             `json:"changes"`
}

// Manifest records the exact foundation file hashes used for future three-way upgrades.
type Manifest struct {
	APIVersion   string                   `json:"apiVersion"`
	Kind         string                   `json:"kind"`
	Metadata     ManifestMetadata         `json:"metadata"`
	Distribution project.DistributionSpec `json:"distribution,omitempty"`
	Identities   IdentitySet              `json:"identities,omitempty"`
	Records      ManifestRecords          `json:"records,omitempty"`
	Files        map[string]ManifestFile  `json:"files"`
}

// ManifestMetadata identifies the downstream application and foundation revision.
type ManifestMetadata struct {
	Project              string `json:"project"`
	Module               string `json:"module"`
	Repository           string `json:"repository"`
	Blueprint            string `json:"blueprint"`
	BlueprintVersion     string `json:"blueprintVersion"`
	FoundationRepository string `json:"foundationRepository"`
	FoundationCommit     string `json:"foundationCommit"`
	FoundationTimestamp  string `json:"foundationTimestamp"`
	GeneratorVersion     string `json:"generatorVersion"`
	GeneratorCommit      string `json:"generatorCommit,omitempty"`
}

// SnapshotRecordPaths identifies the two representations of one downstream
// snapshot. Path changes require an explicit migration recipe.
type SnapshotRecordPaths struct {
	LockPath     string `json:"lockPath" yaml:"lockPath"`
	ManifestPath string `json:"manifestPath" yaml:"manifestPath"`
}

// ManifestRecords binds the manifest to the exact lock bytes.
type ManifestRecords struct {
	SnapshotRecordPaths
	LockSHA256 string `json:"lockSha256"`
}

// ManifestFile records the deterministic content and permission baseline.
type ManifestFile struct {
	SHA256 string      `json:"sha256"`
	Mode   fs.FileMode `json:"mode"`
	Size   int64       `json:"size"`
}

type desiredFile struct {
	Data []byte
	Mode fs.FileMode
}

type selectedCommittedFile struct {
	Source committedFile
	Output string
}

type blueprintSourceFile struct {
	SourcePath string
	OutputPath string
	Data       []byte
	Mode       fs.FileMode
}

const applicationDisplayNameYAMLToken = "__MSS_APP_DISPLAY_NAME_YAML__"

// Generate plans or writes a complete downstream management-system repository.
func Generate(ctx context.Context, options Options) (Plan, error) {
	root, err := filepath.Abs(options.FoundationRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve foundation root: %w", err)
	}
	options.FoundationRoot = root
	options.Application = normalizeApplication(options.Application)
	if err := ValidateApplication(options.Application); err != nil {
		return Plan{}, err
	}
	blueprint, err := Load(root, options.Blueprint)
	if err != nil {
		return Plan{}, err
	}
	destination, err := resolveDestination(root, blueprint, options.Application.Name, options.Destination)
	if err != nil {
		return Plan{}, err
	}
	files, manifest, err := buildDesired(ctx, root, blueprint, options.Application, options.FrontendRegistryURL)
	if err != nil {
		return Plan{}, err
	}

	plan, err := planDestination(destination, blueprint, options.Application, manifest, files, !options.Write)
	if err != nil {
		return plan, err
	}
	if !options.Write {
		return plan, nil
	}
	if !plan.Success {
		return plan, errors.New("destination contains conflicting files; no files were written")
	}
	if err := writeGeneratedSnapshot(ctx, destination, blueprint, plan, files, options.InitializeGit); err != nil {
		return plan, err
	}
	plan.DryRun = false
	return plan, nil
}

// BuildDesired renders all tracked foundation files in memory without touching the destination.
func BuildDesired(ctx context.Context, root string, blueprint *Document, application Application) (map[string]desiredFile, Manifest, error) {
	return buildDesired(ctx, root, blueprint, application, "")
}

func buildDesired(ctx context.Context, root string, blueprint *Document, application Application, frontendRegistryURL string) (map[string]desiredFile, Manifest, error) {
	application = normalizeApplication(application)
	if err := ValidateApplication(application); err != nil {
		return nil, Manifest{}, err
	}
	source, err := loadCommittedFoundation(ctx, root, blueprint)
	if err != nil {
		return nil, Manifest{}, err
	}
	blueprint = source.Blueprint
	selected := make([]selectedCommittedFile, 0, len(source.Entries))
	templatePrefix := ""
	if blueprint.Spec.TemplateRoot != "" {
		templatePrefix = strings.TrimSuffix(normalizedPath(blueprint.Spec.TemplateRoot), "/") + "/"
	}
	for _, entry := range source.Entries {
		relative := entry.Path
		if templatePrefix != "" {
			if !strings.HasPrefix(relative, templatePrefix) {
				continue
			}
			relative = strings.TrimPrefix(relative, templatePrefix)
			relative = templateOutputPath(relative)
			if !safeRelativePath(relative) {
				return nil, Manifest{}, fmt.Errorf("application template produced unsafe output path %q", relative)
			}
		}
		if blueprint.Excluded(relative) ||
			normalizedPath(relative) == normalizedPath(blueprint.Spec.ManifestPath) ||
			normalizedPath(relative) == normalizedPath(blueprint.Spec.LockPath) {
			continue
		}
		if entry.GitMode == "120000" {
			return nil, Manifest{}, fmt.Errorf("tracked symlinks are not supported by application blueprints: %s", relative)
		}
		if entry.Type != "blob" {
			continue
		}
		selected = append(selected, selectedCommittedFile{Source: entry, Output: relative})
	}
	sourceEntries := make([]committedFile, 0, len(selected))
	for _, selectedEntry := range selected {
		sourceEntries = append(sourceEntries, selectedEntry.Source)
	}
	blobs, err := readCommittedBlobs(ctx, root, sourceEntries)
	if err != nil {
		return nil, Manifest{}, err
	}
	resolved := make([]blueprintSourceFile, 0, len(selected))
	for _, selectedEntry := range selected {
		entry := selectedEntry.Source
		resolved = append(resolved, blueprintSourceFile{
			SourcePath: entry.Path,
			OutputPath: selectedEntry.Output,
			Data:       blobs[entry.Path],
			Mode:       entry.Mode,
		})
	}
	frontendPackage, err := resolveFrontendPackageForSource(ctx, frontendRegistryURL, blueprint, resolved)
	if err != nil {
		return nil, Manifest{}, err
	}
	return buildDesiredFromSource(blueprint, source.BlueprintSHA, source.Identity, resolved, source.Presentation, application, frontendPackage)
}

func buildDesiredFromSource(
	blueprint *Document,
	blueprintSHA string,
	foundationIdentity FoundationIdentity,
	sourceFiles []blueprintSourceFile,
	presentation presentationSnapshot,
	application Application,
	frontendPackage frontendPackageResolution,
) (map[string]desiredFile, Manifest, error) {
	files := make(map[string]desiredFile, len(sourceFiles)+2)
	templatePrefix := ""
	if blueprint.Spec.TemplateRoot != "" {
		templatePrefix = strings.TrimSuffix(normalizedPath(blueprint.Spec.TemplateRoot), "/") + "/"
	}
	for _, sourceFile := range sourceFiles {
		relative := sourceFile.OutputPath
		data := sourceFile.Data
		if blueprint.Text(relative, data) {
			data = transformText(data, blueprint, application, frontendPackage)
			if templatePrefix != "" && bytes.Contains(data, []byte("__MSS_")) {
				return nil, Manifest{}, fmt.Errorf("application template %s contains an unresolved mss placeholder", sourceFile.SourcePath)
			}
		}
		mode := sourceFile.Mode.Perm()
		if mode == 0 {
			mode = 0o644
		}
		files[relative] = desiredFile{Data: data, Mode: mode}
	}
	for _, required := range blueprint.Spec.RequiredFiles {
		if _, exists := files[required]; !exists {
			return nil, Manifest{}, fmt.Errorf("required blueprint output %s was excluded or missing", required)
		}
	}
	if templatePrefix != "" {
		projectFile, exists := files[".mss/project.yaml"]
		if !exists {
			return nil, Manifest{}, errors.New("application template is missing .mss/project.yaml")
		}
		renderedProject, err := project.DecodeProjectDocument(projectFile.Data)
		if err != nil {
			return nil, Manifest{}, fmt.Errorf("self-validate rendered .mss/project.yaml: %w", err)
		}
		if renderedProject.Metadata.Name != application.Name ||
			renderedProject.Metadata.DisplayName != application.DisplayName ||
			renderedProject.Metadata.Repository != application.Repository ||
			renderedProject.Spec.Backend.Module != application.Module {
			return nil, Manifest{}, errors.New("self-validate rendered .mss/project.yaml: application identity changed during rendering")
		}
	}
	if presentation.APIVersion != "" {
		if _, exists := files[presentationSnapshotPath]; exists {
			return nil, Manifest{}, fmt.Errorf("application template collides with generated Admin presentation snapshot %s", presentationSnapshotPath)
		}
		data, err := renderPresentationSnapshot(presentation)
		if err != nil {
			return nil, Manifest{}, fmt.Errorf("render Admin presentation upgrade snapshot: %w", err)
		}
		files[presentationSnapshotPath] = desiredFile{Data: data, Mode: 0o644}
	}
	baseline := make(map[string]ManifestFile, len(files))
	for relative, file := range files {
		baseline[relative] = ManifestFile{
			SHA256: digest(file.Data),
			Mode:   file.Mode.Perm(),
			Size:   int64(len(file.Data)),
		}
	}
	identities := IdentitySet{
		Foundation: foundationIdentity,
		Blueprint: BlueprintIdentity{
			Name:    blueprint.Metadata.Name,
			Version: blueprint.Metadata.Version,
			SHA256:  blueprintSHA,
		},
		Generator: GeneratorIdentity{
			Tool:    "mss",
			Version: buildinfo.VersionString(),
			Commit:  strings.ToLower(buildinfo.CommitString()),
		},
		Snapshot: DownstreamSnapshotIdentity{
			Project:    application.Name,
			Module:     application.Module,
			Repository: application.Repository,
		},
	}
	snapshotSHA, err := computeSnapshotDigest(identities, baseline)
	if err != nil {
		return nil, Manifest{}, err
	}
	identities.Snapshot.SHA256 = snapshotSHA
	records := SnapshotRecordPaths{
		LockPath:     normalizedPath(blueprint.Spec.LockPath),
		ManifestPath: normalizedPath(blueprint.Spec.ManifestPath),
	}
	if err := validateIdentitySet(identities, baseline, records.LockPath, records.ManifestPath, true); err != nil {
		return nil, Manifest{}, fmt.Errorf("resolve downstream snapshot identities: %w", err)
	}
	lockData, err := renderFoundationLock(identities, records, blueprint.Spec.Distribution)
	if err != nil {
		return nil, Manifest{}, err
	}
	if _, err := decodeFoundationLock(lockData); err != nil {
		return nil, Manifest{}, fmt.Errorf("self-validate rendered foundation lock: %w", err)
	}
	manifest := Manifest{
		APIVersion: snapshotAPIVersion,
		Kind:       manifestKind,
		Metadata: ManifestMetadata{
			Project:              application.Name,
			Module:               application.Module,
			Repository:           application.Repository,
			Blueprint:            blueprint.Metadata.Name,
			BlueprintVersion:     blueprint.Metadata.Version,
			FoundationRepository: identities.Foundation.Repository,
			FoundationCommit:     identities.Foundation.Commit,
			FoundationTimestamp:  identities.Foundation.Timestamp,
			GeneratorVersion:     identities.Generator.Version,
			GeneratorCommit:      identities.Generator.Commit,
		},
		Distribution: blueprint.Spec.Distribution,
		Identities:   identities,
		Records: ManifestRecords{
			SnapshotRecordPaths: records,
			LockSHA256:          digest(lockData),
		},
		Files: baseline,
	}
	manifestData, err := renderManifest(manifest)
	if err != nil {
		return nil, Manifest{}, err
	}
	if _, _, err := decodeManifest(manifestData, false); err != nil {
		return nil, Manifest{}, fmt.Errorf("self-validate rendered manifest: %w", err)
	}
	files[records.LockPath] = desiredFile{Data: lockData, Mode: 0o644}
	files[records.ManifestPath] = desiredFile{Data: manifestData, Mode: 0o644}
	return files, manifest, nil
}

func planDestination(destination string, blueprint *Document, application Application, manifest Manifest, files map[string]desiredFile, dryRun bool) (Plan, error) {
	return buildDestinationPlan(
		destination,
		blueprint,
		application,
		manifest,
		files,
		dryRun,
		func(relative string) ([]byte, bool, error) { return readManagedFile(destination, relative) },
		func() ([]string, error) { return unknownDestinationFiles(destination, files) },
	)
}

func planDestinationManaged(root *managedRoot, blueprint *Document, application Application, manifest Manifest, files map[string]desiredFile, dryRun bool) (Plan, error) {
	return buildDestinationPlan(
		root.path,
		blueprint,
		application,
		manifest,
		files,
		dryRun,
		func(relative string) ([]byte, bool, error) {
			data, exists, _, err := root.readFile(relative)
			return data, exists, err
		},
		func() ([]string, error) { return root.unknownFiles(files) },
	)
}

func buildDestinationPlan(
	destination string,
	blueprint *Document,
	application Application,
	manifest Manifest,
	files map[string]desiredFile,
	dryRun bool,
	readFile func(string) ([]byte, bool, error),
	unknownFiles func() ([]string, error),
) (Plan, error) {
	plan := Plan{
		Blueprint:        blueprint.Metadata.Name,
		BlueprintVersion: blueprint.Metadata.Version,
		FoundationCommit: manifest.Metadata.FoundationCommit,
		Identities:       manifest.Identities,
		Distribution:     blueprint.Spec.Distribution,
		Application:      application,
		Destination:      destination,
		DryRun:           dryRun,
		Success:          true,
		TotalFiles:       len(files),
		Changes:          make([]FileChange, 0, len(files)),
	}
	paths := make([]string, 0, len(files))
	for relative := range files {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		file := files[relative]
		plan.TotalBytes += int64(len(file.Data))
		change := FileChange{
			Path:   relative,
			Action: ActionCreate,
			Mode:   file.Mode,
			Size:   int64(len(file.Data)),
			SHA256: digest(file.Data),
		}
		existing, exists, err := readFile(relative)
		switch {
		case err != nil:
			change.Action = ActionConflict
			change.Detail = err.Error()
			plan.Success = false
		case !exists:
		case bytes.Equal(existing, file.Data):
			change.Action = ActionUnchanged
		default:
			change.Action = ActionConflict
			change.Detail = "destination file differs from the blueprint"
			plan.Success = false
		}
		plan.Changes = append(plan.Changes, change)
	}
	unknown, err := unknownFiles()
	if err != nil {
		return plan, err
	}
	for _, relative := range unknown {
		plan.Success = false
		plan.Changes = append(plan.Changes, FileChange{
			Path:   relative,
			Action: ActionConflict,
			Detail: "destination contains a file not owned by this blueprint",
		})
	}
	sort.SliceStable(plan.Changes, func(i, j int) bool { return plan.Changes[i].Path < plan.Changes[j].Path })
	return plan, nil
}

func transformText(data []byte, blueprint *Document, application Application, frontendPackage frontendPackageResolution) []byte {
	text := string(data)
	const (
		moduleSentinel     = "__MSS_BLUEPRINT_TARGET_MODULE__"
		repositorySentinel = "__MSS_BLUEPRINT_TARGET_REPOSITORY__"
	)
	replacements := []struct{ token, value string }{
		{token: "__MSS_APP_NAME__", value: application.Name},
		// A Go-quoted string is also a portable YAML double-quoted scalar. Keep
		// this context-specific token separate from the human-readable token so
		// Markdown headings retain the exact display name rather than YAML
		// quoting or escape sequences.
		{token: applicationDisplayNameYAMLToken, value: strconv.Quote(application.DisplayName)},
		{token: "__MSS_APP_DISPLAY_NAME__", value: application.DisplayName},
		{token: "__MSS_APP_MODULE__", value: application.Module},
		{token: "__MSS_APP_REPOSITORY__", value: application.Repository},
		{token: "__MSS_GENERATOR_VERSION__", value: buildinfo.VersionString()},
		{token: "__MSS_DISTRIBUTION_NAME__", value: blueprint.Spec.Distribution.Name},
		{token: "__MSS_DISTRIBUTION_VERSION__", value: blueprint.Spec.Distribution.Version},
		{token: "__MSS_DISTRIBUTION_BACKEND_MODULE__", value: blueprint.Spec.Distribution.Backend.Module},
		{token: "__MSS_DISTRIBUTION_BACKEND_VERSION__", value: blueprint.Spec.Distribution.Backend.Version},
		{token: "__MSS_DISTRIBUTION_FRONTEND_PACKAGE__", value: blueprint.Spec.Distribution.Frontend.Package},
		{token: "__MSS_DISTRIBUTION_FRONTEND_VERSION__", value: blueprint.Spec.Distribution.Frontend.Version},
		{token: frontendIntegrityToken, value: frontendPackage.Integrity},
		{token: frontendTarballToken, value: frontendPackage.Tarball},
	}
	for _, replacement := range replacements {
		text = strings.ReplaceAll(text, replacement.token, replacement.value)
	}
	// Application templates use explicit placeholders for every downstream and
	// Distribution-owned identity. Do not run the legacy full-repository text
	// rewrite afterwards: the Admin module is intentionally rooted in the
	// Foundation repository and must not be rewritten to the downstream module.
	if blueprint.Spec.TemplateRoot != "" {
		return []byte(text)
	}
	sourceRepository := foundationRepository(blueprint)
	text = strings.ReplaceAll(text, blueprint.Spec.SourceModule, moduleSentinel)
	if sourceRepository != "" {
		text = strings.ReplaceAll(text, sourceRepository, repositorySentinel)
	}
	text = strings.ReplaceAll(text, blueprint.Spec.SourceProjectName, application.Name)
	text = strings.ReplaceAll(text, "mss-boot Agent-Native Management Foundation", application.DisplayName)
	text = strings.ReplaceAll(text, "mss-boot-admin Agent Contract", application.DisplayName+" Agent Contract")
	text = strings.ReplaceAll(text, moduleSentinel, application.Module)
	if sourceRepository != "" {
		text = strings.ReplaceAll(text, repositorySentinel, application.Repository)
	}
	return []byte(text)
}

func resolveDestination(root string, blueprint *Document, name, requested string) (string, error) {
	var destination string
	if strings.TrimSpace(requested) == "" {
		destination = filepath.Join(root, filepath.FromSlash(blueprint.Spec.DefaultOutputDirectory), name)
	} else if filepath.IsAbs(requested) {
		destination = filepath.Clean(requested)
	} else {
		destination = filepath.Join(root, filepath.FromSlash(requested))
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	if absolute == root {
		return "", errors.New("application destination cannot be the foundation root")
	}
	relativeRoot, err := filepath.Rel(absolute, root)
	if err == nil && relativeRoot != ".." && !strings.HasPrefix(relativeRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("application destination cannot be an ancestor of the foundation root")
	}
	return absolute, nil
}

func unknownDestinationFiles(destination string, desired map[string]desiredFile) ([]string, error) {
	info, err := os.Stat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("application destination exists and is not a directory")
	}
	var unknown []string
	err = filepath.WalkDir(destination, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(destination, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "." {
			return nil
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git/") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == strings.TrimSuffix(snapshotRuntimePrefix, "/") || strings.HasPrefix(relative, snapshotRuntimePrefix) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if _, exists := desired[relative]; !exists {
			unknown = append(unknown, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(unknown)
	return unknown, nil
}

func initializeGit(ctx context.Context, root *managedRoot) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("initialize downstream Git repository: %w", err)
	}
	if info, err := root.root.Lstat(".git"); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("downstream .git path must not be a symlink")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect downstream Git repository: %w", err)
	}

	// A minimal non-bare repository avoids passing the destination path to an
	// external process after it has been validated. Every entry is created via
	// the pinned os.Root, so a concurrent parent rename or symlink swap cannot
	// redirect `--git-init` outside the selected repository.
	for _, directory := range []string{
		".git",
		".git/objects",
		".git/refs",
		".git/refs/heads",
		".git/refs/tags",
	} {
		if err := root.ensureDirectory(directory, 0o755); err != nil {
			return fmt.Errorf("initialize downstream Git directory %s: %w", directory, err)
		}
	}
	fileMode := "true"
	if runtime.GOOS == "windows" {
		fileMode = "false"
	}
	config := "[core]\n" +
		"\trepositoryformatversion = 0\n" +
		"\tfilemode = " + fileMode + "\n" +
		"\tbare = false\n" +
		"\tlogallrefupdates = true\n"
	gitFiles := []struct {
		path string
		data []byte
	}{
		{path: ".git/HEAD", data: []byte("ref: refs/heads/main\n")},
		{path: ".git/config", data: []byte(config)},
		{path: ".git/description", data: []byte("Unnamed repository; edit this file 'description' to name the repository.\n")},
	}
	for _, file := range gitFiles {
		if err := root.writeAtomic(file.path, file.data, 0o644); err != nil {
			return fmt.Errorf("initialize downstream Git file %s: %w", file.path, err)
		}
	}
	return nil
}

func normalizeApplication(application Application) Application {
	application.Name = strings.TrimSpace(application.Name)
	if strings.TrimSpace(application.DisplayName) == "" {
		words := strings.Split(application.Name, "-")
		for index, word := range words {
			if word == "" {
				continue
			}
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
		application.DisplayName = strings.Join(words, " ")
	}
	application.DisplayName = strings.TrimSpace(application.DisplayName)
	application.Module = strings.TrimSpace(application.Module)
	application.Repository = strings.TrimSpace(application.Repository)
	if application.Repository == "" {
		application.Repository = inferRepository(application.Module)
	}
	if application.Repository == "" {
		application.Repository = "local/" + application.Name
	}
	return application
}

func inferRepository(module string) string {
	const prefix = "github.com/"
	if !strings.HasPrefix(module, prefix) {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(module, prefix), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func foundationRepository(blueprint *Document) string {
	const prefix = "github.com/"
	if strings.HasPrefix(blueprint.Spec.SourceModule, prefix) {
		parts := strings.Split(strings.TrimPrefix(blueprint.Spec.SourceModule, prefix), "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return blueprint.Spec.SourceProjectName
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
