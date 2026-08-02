package blueprint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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
	FoundationRoot string
	Blueprint      string
	Destination    string
	Application    Application
	Write          bool
	InitializeGit  bool
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
	Blueprint        string       `json:"blueprint"`
	BlueprintVersion string       `json:"blueprintVersion"`
	FoundationCommit string       `json:"foundationCommit"`
	Application      Application  `json:"application"`
	Destination      string       `json:"destination"`
	DryRun           bool         `json:"dryRun"`
	Success          bool         `json:"success"`
	TotalFiles       int          `json:"totalFiles"`
	TotalBytes       int64        `json:"totalBytes"`
	Changes          []FileChange `json:"changes"`
}

// Manifest records the exact foundation file hashes used for future three-way upgrades.
type Manifest struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   ManifestMetadata        `json:"metadata"`
	Files      map[string]ManifestFile `json:"files"`
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
	files, manifest, err := BuildDesired(ctx, root, blueprint, options.Application)
	if err != nil {
		return Plan{}, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Plan{}, err
	}
	manifestData = append(manifestData, '\n')
	files[blueprint.Spec.ManifestPath] = desiredFile{Data: manifestData, Mode: 0o644}

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
	if err := writeFiles(destination, files); err != nil {
		return plan, err
	}
	if options.InitializeGit {
		if err := initializeGit(ctx, destination); err != nil {
			return plan, err
		}
	}
	plan.DryRun = false
	return plan, nil
}

// BuildDesired renders all tracked foundation files in memory without touching the destination.
func BuildDesired(ctx context.Context, root string, blueprint *Document, application Application) (map[string]desiredFile, Manifest, error) {
	tracked, err := trackedFiles(ctx, root)
	if err != nil {
		return nil, Manifest{}, err
	}
	commit, timestamp, err := foundationRevision(ctx, root)
	if err != nil {
		return nil, Manifest{}, err
	}
	files := make(map[string]desiredFile, len(tracked))
	for _, relative := range tracked {
		if blueprint.Excluded(relative) || relative == blueprint.Spec.ManifestPath {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, Manifest{}, fmt.Errorf("stat tracked file %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, Manifest{}, fmt.Errorf("tracked symlinks are not supported by application blueprints: %s", relative)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, Manifest{}, fmt.Errorf("read tracked file %s: %w", relative, err)
		}
		if blueprint.Text(relative, data) {
			data = transformText(data, blueprint, application)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		files[relative] = desiredFile{Data: data, Mode: mode}
	}

	lockData, err := renderFoundationLock(blueprint, application, commit)
	if err != nil {
		return nil, Manifest{}, err
	}
	files[blueprint.Spec.LockPath] = desiredFile{Data: lockData, Mode: 0o644}

	for _, required := range blueprint.Spec.RequiredFiles {
		if _, exists := files[required]; !exists {
			return nil, Manifest{}, fmt.Errorf("required blueprint output %s was excluded or missing", required)
		}
	}
	manifest := Manifest{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "BlueprintManifest",
		Metadata: ManifestMetadata{
			Project:              application.Name,
			Module:               application.Module,
			Repository:           application.Repository,
			Blueprint:            blueprint.Metadata.Name,
			BlueprintVersion:     blueprint.Metadata.Version,
			FoundationRepository: foundationRepository(blueprint),
			FoundationCommit:     commit,
			FoundationTimestamp:  timestamp,
			GeneratorVersion:     "0.1.0",
		},
		Files: make(map[string]ManifestFile, len(files)),
	}
	for relative, file := range files {
		manifest.Files[relative] = ManifestFile{
			SHA256: digest(file.Data),
			Mode:   file.Mode,
			Size:   int64(len(file.Data)),
		}
	}
	return files, manifest, nil
}

// ReadManifest loads a downstream blueprint baseline.
func ReadManifest(root, relative string) (Manifest, error) {
	if relative == "" {
		relative = ".mss/blueprint-manifest.json"
	}
	if !safeRelativePath(relative) {
		return Manifest{}, errors.New("manifest path must be repository-relative")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse blueprint manifest: %w", err)
	}
	if manifest.APIVersion != "mss.io/v1alpha1" || manifest.Kind != "BlueprintManifest" {
		return Manifest{}, errors.New("unsupported blueprint manifest identity")
	}
	if manifest.Metadata.Project == "" || manifest.Metadata.Module == "" || manifest.Metadata.Blueprint == "" {
		return Manifest{}, errors.New("blueprint manifest metadata is incomplete")
	}
	return manifest, nil
}

func planDestination(destination string, blueprint *Document, application Application, manifest Manifest, files map[string]desiredFile, dryRun bool) (Plan, error) {
	plan := Plan{
		Blueprint:        blueprint.Metadata.Name,
		BlueprintVersion: blueprint.Metadata.Version,
		FoundationCommit: manifest.Metadata.FoundationCommit,
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
		existing, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			change.Action = ActionConflict
			change.Detail = err.Error()
			plan.Success = false
		case bytes.Equal(existing, file.Data):
			change.Action = ActionUnchanged
		case err == nil:
			change.Action = ActionConflict
			change.Detail = "destination file differs from the blueprint"
			plan.Success = false
		}
		plan.Changes = append(plan.Changes, change)
	}
	unknown, err := unknownDestinationFiles(destination, files)
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

func writeFiles(destination string, files map[string]desiredFile) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create application destination: %w", err)
	}
	paths := make([]string, 0, len(files))
	for relative := range files {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		file := files[relative]
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent for %s: %w", relative, err)
		}
		if err := writeAtomic(target, file.Data, file.Mode); err != nil {
			return fmt.Errorf("write %s: %w", relative, err)
		}
	}
	return nil
}

func transformText(data []byte, blueprint *Document, application Application) []byte {
	text := string(data)
	const (
		moduleSentinel     = "__MSS_BLUEPRINT_TARGET_MODULE__"
		repositorySentinel = "__MSS_BLUEPRINT_TARGET_REPOSITORY__"
	)
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

func renderFoundationLock(blueprint *Document, application Application, commit string) ([]byte, error) {
	lock := map[string]any{
		"apiVersion": "mss.io/v1alpha1",
		"kind":       "FoundationLock",
		"metadata": map[string]any{
			"project": application.Name,
		},
		"spec": map[string]any{
			"foundation": map[string]any{
				"repository": foundationRepository(blueprint),
				"version":    blueprint.Metadata.Version,
				"commit":     commit,
				"channel":    "stable",
			},
			"blueprint": map[string]any{
				"name":    blueprint.Metadata.Name,
				"version": blueprint.Metadata.Version,
			},
			"contracts": map[string]any{
				"project":           "v1alpha1",
				"capabilityCatalog": "v1alpha1",
				"commandCatalog":    "v1alpha1",
				"adminModule":       "v1alpha1",
				"feature":           "v1alpha1",
				"evaluation":        "v1alpha1",
			},
			"generatedBy": map[string]any{
				"tool":    "mss",
				"version": "0.1.0",
			},
			"modules":  map[string]any{},
			"upgrades": []any{},
		},
	}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func trackedFiles(ctx context.Context, root string) ([]string, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked foundation files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		relative := filepath.ToSlash(filepath.Clean(filepath.FromSlash(string(part))))
		if !safeRelativePath(relative) {
			return nil, fmt.Errorf("git returned unsafe tracked path %q", relative)
		}
		files = append(files, relative)
	}
	sort.Strings(files)
	return files, nil
}

func foundationRevision(ctx context.Context, root string) (string, string, error) {
	commitCommand := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	commitOutput, err := commitCommand.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve foundation commit: %w", err)
	}
	commit := strings.TrimSpace(string(commitOutput))
	timeCommand := exec.CommandContext(ctx, "git", "-C", root, "show", "-s", "--format=%cI", "HEAD")
	timeOutput, err := timeCommand.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve foundation timestamp: %w", err)
	}
	timestamp := strings.TrimSpace(string(timeOutput))
	if timestamp == "" {
		return "", "", errors.New("foundation timestamp is empty")
	}
	// The value is emitted by Git itself and is audit metadata only. Do not
	// reject valid ISO-8601 offset variants due to Go runtime parser changes.
	return commit, timestamp, nil
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

func initializeGit(ctx context.Context, destination string) error {
	if _, err := os.Stat(filepath.Join(destination, ".git")); err == nil {
		return nil
	}
	command := exec.CommandContext(ctx, "git", "init", "--initial-branch=main", destination)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initialize downstream Git repository: %w: %s", err, strings.TrimSpace(string(output)))
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

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	temporary := path + ".mss-tmp"
	if err := os.WriteFile(temporary, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(temporary, mode); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
