package blueprint

import (
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	blueprintAPIVersion = "mss.io/v1alpha1"
	blueprintKind       = "ApplicationBlueprint"
	blueprintDirectory  = ".mss/blueprints"
)

var (
	blueprintNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	goModulePattern      = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)
	repositoryPattern    = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

// Document is the versioned machine-readable application blueprint.
type Document struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
	SourcePath string   `yaml:"-" json:"sourcePath,omitempty"`
}

// Metadata identifies one blueprint release.
type Metadata struct {
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"displayName" json:"displayName"`
	Version     string `yaml:"version" json:"version"`
}

// Spec controls tracked-file selection, transformations, and generated metadata.
type Spec struct {
	SourceMode             string   `yaml:"sourceMode" json:"sourceMode"`
	SourceModule           string   `yaml:"sourceModule" json:"sourceModule"`
	SourceProjectName      string   `yaml:"sourceProjectName" json:"sourceProjectName"`
	DefaultOutputDirectory string   `yaml:"defaultOutputDirectory" json:"defaultOutputDirectory"`
	ManifestPath           string   `yaml:"manifestPath" json:"manifestPath"`
	LockPath               string   `yaml:"lockPath" json:"lockPath"`
	RequiredFiles          []string `yaml:"requiredFiles" json:"requiredFiles"`
	ExcludePaths           []string `yaml:"excludePaths,omitempty" json:"excludePaths,omitempty"`
	ExcludePrefixes        []string `yaml:"excludePrefixes,omitempty" json:"excludePrefixes,omitempty"`
	TextExtensions         []string `yaml:"textExtensions,omitempty" json:"textExtensions,omitempty"`
	TextNames              []string `yaml:"textNames,omitempty" json:"textNames,omitempty"`
}

// Application identifies the downstream project rendered from a blueprint.
type Application struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Module      string `json:"module"`
	Repository  string `json:"repository"`
}

// Load reads a named blueprint from one foundation checkout.
func Load(root, name string) (*Document, error) {
	if strings.TrimSpace(name) == "" {
		name = "management-system"
	}
	if !blueprintNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid blueprint name %q", name)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve foundation root: %w", err)
	}
	relative := filepath.ToSlash(filepath.Join(blueprintDirectory, name+".yaml"))
	path := filepath.Join(absoluteRoot, filepath.FromSlash(relative))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read blueprint %s: %w", relative, err)
	}
	return decodeDocument(absoluteRoot, relative, data)
}

func decodeDocument(root, relative string, data []byte) (*Document, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return nil, fmt.Errorf("parse blueprint %s: %w", relative, err)
	}
	document := &Document{SourcePath: normalizedPath(relative)}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(document); err != nil {
		return nil, fmt.Errorf("parse blueprint %s: %w", relative, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("parse blueprint %s: multiple YAML documents are not supported", relative)
		}
		return nil, fmt.Errorf("parse blueprint %s: %w", relative, err)
	}
	document.Normalize()
	if err := document.Validate(root); err != nil {
		return nil, err
	}
	return document, nil
}

// Normalize makes all matching and output deterministic.
func (d *Document) Normalize() {
	d.Metadata.Name = strings.TrimSpace(d.Metadata.Name)
	d.Metadata.DisplayName = strings.TrimSpace(d.Metadata.DisplayName)
	d.Metadata.Version = strings.TrimSpace(d.Metadata.Version)
	d.Spec.SourceMode = strings.TrimSpace(d.Spec.SourceMode)
	d.Spec.SourceModule = strings.TrimSpace(d.Spec.SourceModule)
	d.Spec.SourceProjectName = strings.TrimSpace(d.Spec.SourceProjectName)
	if d.Spec.DefaultOutputDirectory == "" {
		d.Spec.DefaultOutputDirectory = ".mss/output"
	}
	if d.Spec.ManifestPath == "" {
		d.Spec.ManifestPath = ".mss/blueprint-manifest.json"
	}
	if d.Spec.LockPath == "" {
		d.Spec.LockPath = ".mss/lock.yaml"
	}
	d.Spec.DefaultOutputDirectory = normalizedPath(d.Spec.DefaultOutputDirectory)
	d.Spec.ManifestPath = normalizedPath(d.Spec.ManifestPath)
	d.Spec.LockPath = normalizedPath(d.Spec.LockPath)

	d.Spec.RequiredFiles = normalizePaths(d.Spec.RequiredFiles, false)
	d.Spec.ExcludePaths = normalizePaths(d.Spec.ExcludePaths, false)
	d.Spec.ExcludePrefixes = normalizePaths(d.Spec.ExcludePrefixes, true)
	for index, extension := range d.Spec.TextExtensions {
		extension = strings.ToLower(strings.TrimSpace(extension))
		if extension != "" && !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		d.Spec.TextExtensions[index] = extension
	}
	d.Spec.TextExtensions = uniqueSorted(d.Spec.TextExtensions)
	d.Spec.TextNames = uniqueSorted(d.Spec.TextNames)
}

// Validate checks blueprint identity, required source files, and path confinement.
func (d *Document) Validate(root string) error {
	var problems []string
	if d.APIVersion != blueprintAPIVersion {
		problems = append(problems, "apiVersion must equal "+blueprintAPIVersion)
	}
	if d.Kind != blueprintKind {
		problems = append(problems, "kind must equal "+blueprintKind)
	}
	if !blueprintNamePattern.MatchString(d.Metadata.Name) {
		problems = append(problems, "metadata.name must be lower-case kebab-case")
	}
	if d.Metadata.DisplayName == "" {
		problems = append(problems, "metadata.displayName is required")
	}
	if !validSemanticVersion(d.Metadata.Version) {
		problems = append(problems, "metadata.version must be a semantic revision")
	}
	if d.Spec.SourceMode != "git-tracked" {
		problems = append(problems, "spec.sourceMode must equal git-tracked")
	}
	if !validModule(d.Spec.SourceModule) {
		problems = append(problems, "spec.sourceModule is invalid")
	}
	if !blueprintNamePattern.MatchString(d.Spec.SourceProjectName) {
		problems = append(problems, "spec.sourceProjectName must be lower-case kebab-case")
	}
	for label, path := range map[string]string{
		"spec.defaultOutputDirectory": d.Spec.DefaultOutputDirectory,
		"spec.manifestPath":           d.Spec.ManifestPath,
		"spec.lockPath":               d.Spec.LockPath,
	} {
		if !safeRelativePath(path) {
			problems = append(problems, label+" must be a repository-relative confined path")
		}
	}
	if safeRelativePath(d.Spec.ManifestPath) && safeRelativePath(d.Spec.LockPath) &&
		normalizedPath(d.Spec.ManifestPath) == normalizedPath(d.Spec.LockPath) {
		problems = append(problems, "spec.manifestPath and spec.lockPath must be distinct")
	}
	if len(d.Spec.RequiredFiles) == 0 {
		problems = append(problems, "spec.requiredFiles must not be empty")
	}
	for _, required := range d.Spec.RequiredFiles {
		if !safeRelativePath(required) {
			problems = append(problems, "required file has unsafe path: "+required)
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(required)))
		if err != nil {
			problems = append(problems, "required file is missing: "+required)
			continue
		}
		if !info.Mode().IsRegular() {
			problems = append(problems, "required path is not a regular file: "+required)
		}
	}
	for _, excluded := range append(append([]string(nil), d.Spec.ExcludePaths...), d.Spec.ExcludePrefixes...) {
		if !safeRelativePath(strings.TrimSuffix(excluded, "/")) {
			problems = append(problems, "excluded path is unsafe: "+excluded)
		}
	}
	if len(d.Spec.TextExtensions) == 0 && len(d.Spec.TextNames) == 0 {
		problems = append(problems, "at least one text extension or text name is required")
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// ValidateApplication checks the identity used for downstream transformations.
func ValidateApplication(application Application) error {
	var problems []string
	if !blueprintNamePattern.MatchString(application.Name) {
		problems = append(problems, "application name must be lower-case kebab-case")
	}
	if strings.TrimSpace(application.DisplayName) == "" {
		problems = append(problems, "application display name is required")
	}
	if !validModule(application.Module) {
		problems = append(problems, "application Go module is invalid")
	}
	if application.Repository != "" && !repositoryPattern.MatchString(application.Repository) {
		problems = append(problems, "application repository must use owner/name form")
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Excluded reports whether a tracked path is omitted from downstream output.
func (d *Document) Excluded(relative string) bool {
	relative = normalizedPath(relative)
	for _, excluded := range d.Spec.ExcludePaths {
		if relative == excluded {
			return true
		}
	}
	for _, prefix := range d.Spec.ExcludePrefixes {
		if strings.HasPrefix(relative, prefix) {
			return true
		}
	}
	return false
}

// Text reports whether the file should receive safe UTF-8 transformations.
func (d *Document) Text(relative string, data []byte) bool {
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return false
	}
	base := filepath.Base(relative)
	for _, name := range d.Spec.TextNames {
		if base == name {
			return true
		}
	}
	extension := strings.ToLower(filepath.Ext(relative))
	for _, allowed := range d.Spec.TextExtensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func validModule(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.ContainsAny(value, " \\:") && goModulePattern.MatchString(value) && strings.Contains(value, "/")
}

func safeRelativePath(value string) bool {
	value = strings.TrimSpace(value)
	// Snapshot and blueprint paths use one portable slash-separated grammar.
	// In particular, a Windows drive/UNC path must remain unsafe when parsed on
	// Unix, rather than becoming dangerous only after the snapshot is moved.
	if value == "" || strings.HasPrefix(value, "/") ||
		strings.ContainsAny(value, "\\:\x00") {
		return false
	}
	clean := pathpkg.Clean(value)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func normalizePaths(values []string, prefix bool) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizedPath(value)
		if value == "." || value == "" {
			continue
		}
		if prefix && !strings.HasSuffix(value, "/") {
			value += "/"
		}
		result = append(result, value)
	}
	return uniqueSorted(result)
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
