package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	projectPath      = ".mss/project.yaml"
	capabilitiesPath = ".mss/capabilities.yaml"
	commandsPath     = ".mss/commands.yaml"
)

// Metadata is common metadata used by the mss project contracts.
type Metadata struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Repository  string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Project     string `yaml:"project,omitempty" json:"project,omitempty"`
}

// ProjectDocument describes repository layout and supported toolchains.
type ProjectDocument struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   Metadata    `yaml:"metadata" json:"metadata"`
	Spec       ProjectSpec `yaml:"spec" json:"spec"`
}

// ProjectSpec contains the subset of project.yaml consumed by the CLI.
type ProjectSpec struct {
	Mission           string            `yaml:"mission" json:"mission"`
	FoundationVersion string            `yaml:"foundationVersion" json:"foundationVersion"`
	RepositoryLayout  map[string]string `yaml:"repositoryLayout" json:"repositoryLayout"`
	Backend           BackendSpec       `yaml:"backend" json:"backend"`
	Frontend          FrontendSpec      `yaml:"frontend" json:"frontend"`
	Documentation     DocumentationSpec `yaml:"documentation" json:"documentation"`
	Database          DatabaseSpec      `yaml:"database" json:"database"`
	LocalDependencies DependencySpec    `yaml:"localDependencies" json:"localDependencies"`
	Conventions       map[string]any    `yaml:"conventions" json:"conventions"`
	Entrypoints       map[string]string `yaml:"entrypoints" json:"entrypoints"`
	Validation        ValidationSpec    `yaml:"validation" json:"validation"`
}

// BackendSpec describes the Go backend contract.
type BackendSpec struct {
	Language        string `yaml:"language" json:"language"`
	GoVersion       string `yaml:"goVersion" json:"goVersion"`
	Module          string `yaml:"module" json:"module"`
	FrameworkModule string `yaml:"frameworkModule" json:"frameworkModule"`
	HTTPFramework   string `yaml:"httpFramework" json:"httpFramework"`
	ORM             string `yaml:"orm" json:"orm"`
	CLI             string `yaml:"cli" json:"cli"`
	Authorization   string `yaml:"authorization" json:"authorization"`
	Contract        string `yaml:"contract" json:"contract"`
}

// FrontendSpec describes the web application contract.
type FrontendSpec struct {
	Language              string                    `yaml:"language" json:"language"`
	NodeVersion           string                    `yaml:"nodeVersion" json:"nodeVersion"`
	PackageManager        string                    `yaml:"packageManager" json:"packageManager"`
	PackageManagerVersion string                    `yaml:"packageManagerVersion" json:"packageManagerVersion"`
	Framework             string                    `yaml:"framework" json:"framework"`
	ApplicationFramework  string                    `yaml:"applicationFramework" json:"applicationFramework"`
	ComponentLibrary      string                    `yaml:"componentLibrary" json:"componentLibrary"`
	DefaultApplication    string                    `yaml:"defaultApplication,omitempty" json:"defaultApplication,omitempty"`
	Applications          []FrontendApplicationSpec `yaml:"applications,omitempty" json:"applications,omitempty"`
}

// FrontendApplicationSpec identifies one independently built and released web
// application. The fields on FrontendSpec are shared toolchain defaults; this
// foundation currently accepts Ant Design 6 as its sole application.
type FrontendApplicationSpec struct {
	ID                    string `yaml:"id" json:"id"`
	Path                  string `yaml:"path" json:"path"`
	Role                  string `yaml:"role" json:"role"`
	NodeVersion           string `yaml:"nodeVersion" json:"nodeVersion"`
	PackageManager        string `yaml:"packageManager" json:"packageManager"`
	PackageManagerVersion string `yaml:"packageManagerVersion" json:"packageManagerVersion"`
	DevelopmentPort       int    `yaml:"developmentPort" json:"developmentPort"`
	ReleaseTagTemplate    string `yaml:"releaseTagTemplate" json:"releaseTagTemplate"`
	Image                 string `yaml:"image" json:"image"`
}

// DocumentationSpec describes the documentation toolchain.
type DocumentationSpec struct {
	Framework      string `yaml:"framework" json:"framework"`
	PackageManager string `yaml:"packageManager" json:"packageManager"`
}

// DatabaseSpec describes local and supported databases.
type DatabaseSpec struct {
	Local     string   `yaml:"local" json:"local"`
	Supported []string `yaml:"supported" json:"supported"`
}

// DependencySpec describes required and optional local services.
type DependencySpec struct {
	Required []string `yaml:"required" json:"required"`
	Optional []string `yaml:"optional" json:"optional"`
}

// ValidationSpec describes canonical validation entrypoints and report paths.
type ValidationSpec struct {
	Changed string            `yaml:"changed" json:"changed"`
	All     string            `yaml:"all" json:"all"`
	Reports map[string]string `yaml:"reports" json:"reports"`
}

// CapabilityCatalog is the machine-readable inventory used to avoid duplicate implementations.
type CapabilityCatalog struct {
	APIVersion string                `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                `yaml:"kind" json:"kind"`
	Metadata   Metadata              `yaml:"metadata" json:"metadata"`
	Spec       CapabilityCatalogSpec `yaml:"spec" json:"spec"`
}

// CapabilityCatalogSpec contains status definitions and individual capabilities.
type CapabilityCatalogSpec struct {
	Statuses     map[string]string `yaml:"statuses" json:"statuses"`
	Capabilities []Capability      `yaml:"capabilities" json:"capabilities"`
}

// Capability is one reusable project capability.
type Capability struct {
	ID             string   `yaml:"id" json:"id"`
	DisplayName    string   `yaml:"displayName" json:"displayName"`
	Status         string   `yaml:"status" json:"status"`
	Owners         []string `yaml:"owners" json:"owners"`
	Paths          []string `yaml:"paths,omitempty" json:"paths,omitempty"`
	BackendPaths   []string `yaml:"backendPaths,omitempty" json:"backendPaths,omitempty"`
	FrameworkPaths []string `yaml:"frameworkPaths,omitempty" json:"frameworkPaths,omitempty"`
	FrontendPaths  []string `yaml:"frontendPaths,omitempty" json:"frontendPaths,omitempty"`
	Guidance       string   `yaml:"guidance" json:"guidance"`
}

// CommandCatalog describes canonical repository commands.
type CommandCatalog struct {
	APIVersion string             `yaml:"apiVersion" json:"apiVersion"`
	Kind       string             `yaml:"kind" json:"kind"`
	Metadata   Metadata           `yaml:"metadata" json:"metadata"`
	Spec       CommandCatalogSpec `yaml:"spec" json:"spec"`
}

// CommandCatalogSpec contains conventions and named commands.
type CommandCatalogSpec struct {
	Conventions map[string]any     `yaml:"conventions" json:"conventions"`
	Commands    map[string]Command `yaml:"commands" json:"commands"`
}

// Command is a canonical project operation.
type Command struct {
	Command         string   `yaml:"command" json:"command"`
	Description     string   `yaml:"description" json:"description"`
	OutputFormats   []string `yaml:"outputFormats,omitempty" json:"outputFormats,omitempty"`
	Category        string   `yaml:"category" json:"category"`
	Idempotent      bool     `yaml:"idempotent,omitempty" json:"idempotent,omitempty"`
	DryRunByDefault bool     `yaml:"dryRunByDefault,omitempty" json:"dryRunByDefault,omitempty"`
}

// Context is the normalized project context returned to agents and internal tooling.
type Context struct {
	Root         string            `json:"root"`
	Project      ProjectDocument   `json:"project"`
	Capabilities CapabilityCatalog `json:"capabilities"`
	Commands     CommandCatalog    `json:"commands"`
}

// DecodeProjectDocument parses one Project contract without consulting any
// sibling files. Blueprint provenance uses this on the committed Git object so
// a worktree cannot supply a repository identity that differs from the commit.
func DecodeProjectDocument(data []byte) (ProjectDocument, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return ProjectDocument{}, fmt.Errorf("parse project contract: %w", err)
	}
	document := ProjectDocument{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return ProjectDocument{}, fmt.Errorf("parse project contract: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ProjectDocument{}, errors.New("parse project contract: multiple YAML documents are not supported")
		}
		return ProjectDocument{}, fmt.Errorf("parse project contract: %w", err)
	}
	if document.APIVersion != "mss.io/v1alpha1" || document.Kind != "Project" {
		return ProjectDocument{}, errors.New("project contract must be mss.io/v1alpha1 Project")
	}
	if strings.TrimSpace(document.Metadata.Name) == "" || strings.TrimSpace(document.Metadata.Repository) == "" {
		return ProjectDocument{}, errors.New("project contract identity is incomplete")
	}
	if strings.TrimSpace(document.Spec.FoundationVersion) == "" {
		return ProjectDocument{}, errors.New("project spec.foundationVersion is required")
	}
	return document, nil
}

// FindRoot searches upward for the project contract.
func FindRoot(start string) (string, error) {
	if strings.TrimSpace(start) == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	if info, statErr := os.Stat(current); statErr == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		if isRegularFile(filepath.Join(current, projectPath)) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("mss project root not found from %q: missing %s", start, projectPath)
}

// Load reads and validates the project, capability, and command contracts.
func Load(start string) (*Context, error) {
	root, err := FindRoot(start)
	if err != nil {
		return nil, err
	}

	ctx := &Context{Root: root}
	if err := readYAML(filepath.Join(root, projectPath), &ctx.Project); err != nil {
		return nil, err
	}
	if err := readYAML(filepath.Join(root, capabilitiesPath), &ctx.Capabilities); err != nil {
		return nil, err
	}
	if err := readYAML(filepath.Join(root, commandsPath), &ctx.Commands); err != nil {
		return nil, err
	}
	if err := ctx.Validate(); err != nil {
		return nil, err
	}
	ctx.Sort()
	return ctx, nil
}

// Validate checks contract identity and required fields that downstream tooling relies on.
func (c *Context) Validate() error {
	var problems []string
	if c.Project.APIVersion != "mss.io/v1alpha1" || c.Project.Kind != "Project" {
		problems = append(problems, "project.yaml must be mss.io/v1alpha1 Project")
	}
	if strings.TrimSpace(c.Project.Metadata.Name) == "" {
		problems = append(problems, "project metadata.name is required")
	}
	if strings.TrimSpace(c.Project.Spec.Backend.Module) == "" {
		problems = append(problems, "project spec.backend.module is required")
	}
	for _, key := range []string{"backend", "framework", "frontend", "documentation", "specifications"} {
		path := strings.TrimSpace(c.Project.Spec.RepositoryLayout[key])
		if path == "" {
			problems = append(problems, "project repositoryLayout."+key+" is required")
			continue
		}
		if !pathWithinRoot(c.Root, path) {
			problems = append(problems, "project repositoryLayout."+key+" escapes repository root")
		}
	}
	applicationIDs := make(map[string]bool, len(c.Project.Spec.Frontend.Applications))
	applicationPaths := make(map[string]bool, len(c.Project.Spec.Frontend.Applications))
	for index, application := range c.Project.Spec.Frontend.Applications {
		id := strings.TrimSpace(application.ID)
		path := strings.TrimSpace(application.Path)
		if id == "" {
			problems = append(problems, fmt.Sprintf("project frontend applications[%d].id is required", index))
		} else if applicationIDs[id] {
			problems = append(problems, "project frontend application id "+id+" is duplicated")
		} else {
			applicationIDs[id] = true
		}
		if path == "" {
			problems = append(problems, fmt.Sprintf("project frontend applications[%d].path is required", index))
		} else if !pathWithinRoot(c.Root, path) {
			problems = append(problems, "project frontend application "+id+" path escapes repository root")
		} else if applicationPaths[path] {
			problems = append(problems, "project frontend application path "+path+" is duplicated")
		} else {
			applicationPaths[path] = true
		}
	}
	if defaultID := strings.TrimSpace(c.Project.Spec.Frontend.DefaultApplication); defaultID != "" {
		if !applicationIDs[defaultID] {
			problems = append(problems, "project frontend defaultApplication must identify a configured application")
		} else if application, ok := c.DefaultFrontendApplication(); ok &&
			strings.TrimSpace(c.Project.Spec.RepositoryLayout["frontend"]) != strings.TrimSpace(application.Path) {
			problems = append(problems, "project repositoryLayout.frontend must equal the default frontend application path")
		}
	} else if len(c.Project.Spec.Frontend.Applications) > 1 {
		defaultPath := strings.TrimSpace(c.Project.Spec.RepositoryLayout["frontend"])
		if !applicationPaths[defaultPath] {
			problems = append(problems, "project frontend defaultApplication is required when repositoryLayout.frontend does not identify one configured application")
		}
	}
	if c.Capabilities.APIVersion != "mss.io/v1alpha1" || c.Capabilities.Kind != "CapabilityCatalog" {
		problems = append(problems, "capabilities.yaml must be mss.io/v1alpha1 CapabilityCatalog")
	}
	if c.Commands.APIVersion != "mss.io/v1alpha1" || c.Commands.Kind != "CommandCatalog" {
		problems = append(problems, "commands.yaml must be mss.io/v1alpha1 CommandCatalog")
	}
	if len(c.Commands.Spec.Commands) == 0 {
		problems = append(problems, "commands catalog must contain at least one command")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// DefaultFrontendApplication resolves the explicitly selected primary
// application. Snapshots without defaultApplication resolve through
// repositoryLayout.frontend, while validation still requires that path to
// identify the sole configured Ant Design 6 application.
func (c *Context) DefaultFrontendApplication() (FrontendApplicationSpec, bool) {
	frontend := c.Project.Spec.Frontend
	defaultID := strings.TrimSpace(frontend.DefaultApplication)
	defaultPath := strings.TrimSpace(c.Project.Spec.RepositoryLayout["frontend"])
	for _, application := range frontend.Applications {
		if (defaultID != "" && strings.TrimSpace(application.ID) == defaultID) ||
			(defaultID == "" && strings.TrimSpace(application.Path) == defaultPath) {
			return application, true
		}
	}
	if len(frontend.Applications) == 0 && defaultPath != "" {
		return FrontendApplicationSpec{
			ID:                    "frontend",
			Path:                  defaultPath,
			Role:                  "primary",
			NodeVersion:           frontend.NodeVersion,
			PackageManager:        frontend.PackageManager,
			PackageManagerVersion: frontend.PackageManagerVersion,
		}, true
	}
	return FrontendApplicationSpec{}, false
}

// Sort makes agent-visible output deterministic.
func (c *Context) Sort() {
	sort.SliceStable(c.Capabilities.Spec.Capabilities, func(i, j int) bool {
		return c.Capabilities.Spec.Capabilities[i].ID < c.Capabilities.Spec.Capabilities[j].ID
	})
	for i := range c.Capabilities.Spec.Capabilities {
		capability := &c.Capabilities.Spec.Capabilities[i]
		sort.Strings(capability.Owners)
		sort.Strings(capability.Paths)
		sort.Strings(capability.BackendPaths)
		sort.Strings(capability.FrameworkPaths)
		sort.Strings(capability.FrontendPaths)
	}
}

// JSON returns stable indented JSON for agent consumption.
func (c *Context) JSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// CommandNames returns command keys in stable order.
func (c *Context) CommandNames() []string {
	names := make([]string, 0, len(c.Commands.Spec.Commands))
	for name := range c.Commands.Spec.Commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func pathWithinRoot(root, relative string) bool {
	if filepath.IsAbs(relative) {
		return false
	}
	cleaned := filepath.Clean(relative)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}
	resolved := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
