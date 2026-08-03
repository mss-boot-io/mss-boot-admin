package dev

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	developmentAPIVersion = "mss.io/v1alpha1"
	developmentKind       = "DevelopmentEnvironment"
	developmentPath       = ".mss/dev.yaml"
)

var serviceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Document is the machine-readable local development environment contract.
type Document struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

// Metadata identifies the project owning the development environment.
type Metadata struct {
	Project string `yaml:"project" json:"project"`
}

// Spec defines runtime storage and ordered development services.
type Spec struct {
	RuntimeDirectory string        `yaml:"runtimeDirectory" json:"runtimeDirectory"`
	LogDirectory     string        `yaml:"logDirectory" json:"logDirectory"`
	StartupTimeout   string        `yaml:"startupTimeout" json:"startupTimeout"`
	StopTimeout      string        `yaml:"stopTimeout" json:"stopTimeout"`
	Services         []ServiceSpec `yaml:"services" json:"services"`
}

// ServiceSpec defines one local development process.
type ServiceSpec struct {
	ID          string            `yaml:"id" json:"id"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Directory   string            `yaml:"directory" json:"directory"`
	Command     []string          `yaml:"command" json:"command"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	DependsOn   []string          `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Required    bool              `yaml:"required" json:"required"`
	Health      *HealthSpec       `yaml:"health,omitempty" json:"health,omitempty"`
}

// HealthSpec defines an HTTP readiness check.
type HealthSpec struct {
	URL           string `yaml:"url" json:"url"`
	Interval      string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout       string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	SuccessStatus []int  `yaml:"successStatus,omitempty" json:"successStatus,omitempty"`
}

// Config is a validated development contract rooted in one repository.
type Config struct {
	Root             string
	Document         Document
	StartupTimeout   time.Duration
	StopTimeout      time.Duration
	RuntimeDirectory string
	LogDirectory     string
	services         map[string]ServiceSpec
}

// Load reads and validates .mss/dev.yaml.
func Load(root string) (*Config, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(absoluteRoot, filepath.FromSlash(developmentPath)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", developmentPath, err)
	}
	document := Document{}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", developmentPath, err)
	}
	config := &Config{Root: absoluteRoot, Document: document}
	if err := config.normalizeAndValidate(); err != nil {
		return nil, err
	}
	return config, nil
}

// Services returns all services in deterministic dependency order.
func (c *Config) Services(selected []string) ([]ServiceSpec, error) {
	requested := make(map[string]bool)
	if len(selected) == 0 {
		for _, service := range c.Document.Spec.Services {
			requested[service.ID] = true
		}
	} else {
		for _, id := range selected {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, exists := c.services[id]; !exists {
				return nil, fmt.Errorf("unknown development service %q", id)
			}
			requested[id] = true
		}
	}

	var includeDependencies func(string)
	includeDependencies = func(id string) {
		for _, dependency := range c.services[id].DependsOn {
			if requested[dependency] {
				continue
			}
			requested[dependency] = true
			includeDependencies(dependency)
		}
	}
	for id := range requested {
		includeDependencies(id)
	}

	ordered, err := c.topologicalOrder()
	if err != nil {
		return nil, err
	}
	result := make([]ServiceSpec, 0, len(requested))
	for _, service := range ordered {
		if requested[service.ID] {
			result = append(result, service)
		}
	}
	return result, nil
}

// Service returns one validated service by ID.
func (c *Config) Service(id string) (ServiceSpec, bool) {
	service, exists := c.services[id]
	return service, exists
}

// ServiceIDs returns stable service IDs.
func (c *Config) ServiceIDs() []string {
	ids := make([]string, 0, len(c.services))
	for id := range c.services {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ResolveDirectory returns a repository-confined service working directory.
func (c *Config) ResolveDirectory(service ServiceSpec) string {
	return filepath.Join(c.Root, filepath.FromSlash(service.Directory))
}

// StatePath returns the state file for one service.
func (c *Config) StatePath(serviceID string) string {
	return filepath.Join(c.RuntimeDirectory, serviceID+".json")
}

// LogPath returns the durable development log for one service.
func (c *Config) LogPath(serviceID string) string {
	return filepath.Join(c.LogDirectory, serviceID+".log")
}

func (c *Config) normalizeAndValidate() error {
	var problems []string
	if c.Document.APIVersion != developmentAPIVersion {
		problems = append(problems, "apiVersion must equal "+developmentAPIVersion)
	}
	if c.Document.Kind != developmentKind {
		problems = append(problems, "kind must equal "+developmentKind)
	}
	if strings.TrimSpace(c.Document.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}

	if c.Document.Spec.RuntimeDirectory == "" {
		c.Document.Spec.RuntimeDirectory = ".mss/run"
	}
	if c.Document.Spec.LogDirectory == "" {
		c.Document.Spec.LogDirectory = ".mss/logs"
	}
	var err error
	c.RuntimeDirectory, err = confinedPath(c.Root, c.Document.Spec.RuntimeDirectory)
	if err != nil {
		problems = append(problems, "runtimeDirectory: "+err.Error())
	}
	c.LogDirectory, err = confinedPath(c.Root, c.Document.Spec.LogDirectory)
	if err != nil {
		problems = append(problems, "logDirectory: "+err.Error())
	}

	c.StartupTimeout, err = parseDuration(c.Document.Spec.StartupTimeout, 90*time.Second)
	if err != nil {
		problems = append(problems, "startupTimeout: "+err.Error())
	}
	c.StopTimeout, err = parseDuration(c.Document.Spec.StopTimeout, 10*time.Second)
	if err != nil {
		problems = append(problems, "stopTimeout: "+err.Error())
	}

	if len(c.Document.Spec.Services) == 0 {
		problems = append(problems, "spec.services must contain at least one service")
	}
	c.services = make(map[string]ServiceSpec, len(c.Document.Spec.Services))
	for index := range c.Document.Spec.Services {
		service := &c.Document.Spec.Services[index]
		service.ID = strings.TrimSpace(service.ID)
		if !serviceIDPattern.MatchString(service.ID) {
			problems = append(problems, fmt.Sprintf("services[%d].id must be lower-case kebab-case", index))
			continue
		}
		if _, exists := c.services[service.ID]; exists {
			problems = append(problems, fmt.Sprintf("services[%d].id %q is duplicated", index, service.ID))
			continue
		}
		if len(service.Command) == 0 || strings.TrimSpace(service.Command[0]) == "" {
			problems = append(problems, fmt.Sprintf("service %s command is required", service.ID))
		}
		if service.Directory == "" {
			service.Directory = "."
		}
		if _, pathErr := confinedPath(c.Root, service.Directory); pathErr != nil {
			problems = append(problems, fmt.Sprintf("service %s directory: %v", service.ID, pathErr))
		}
		for key := range service.Environment {
			if strings.TrimSpace(key) == "" || strings.ContainsRune(key, '=') {
				problems = append(problems, fmt.Sprintf("service %s has invalid environment key %q", service.ID, key))
			}
		}
		if service.Health != nil {
			if healthErr := normalizeHealth(service.Health, c.StartupTimeout); healthErr != nil {
				problems = append(problems, fmt.Sprintf("service %s health: %v", service.ID, healthErr))
			}
		}
		c.services[service.ID] = *service
	}

	for _, service := range c.Document.Spec.Services {
		for _, dependency := range service.DependsOn {
			if dependency == service.ID {
				problems = append(problems, fmt.Sprintf("service %s cannot depend on itself", service.ID))
				continue
			}
			if _, exists := c.services[dependency]; !exists {
				problems = append(problems, fmt.Sprintf("service %s depends on unknown service %s", service.ID, dependency))
			}
		}
	}
	if _, orderErr := c.topologicalOrder(); orderErr != nil {
		problems = append(problems, orderErr.Error())
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) topologicalOrder() ([]ServiceSpec, error) {
	state := make(map[string]int, len(c.services))
	ordered := make([]ServiceSpec, 0, len(c.services))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("development service dependency cycle includes %s", id)
		case 2:
			return nil
		}
		state[id] = 1
		service, exists := c.services[id]
		if !exists {
			return fmt.Errorf("unknown development service %s", id)
		}
		dependencies := append([]string(nil), service.DependsOn...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = 2
		ordered = append(ordered, service)
		return nil
	}
	ids := make([]string, 0, len(c.services))
	for id := range c.services {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func normalizeHealth(health *HealthSpec, defaultTimeout time.Duration) error {
	parsed, err := url.Parse(health.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return errors.New("URL host is required")
	}
	interval, err := parseDuration(health.Interval, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("interval: %w", err)
	}
	timeout, err := parseDuration(health.Timeout, defaultTimeout)
	if err != nil {
		return fmt.Errorf("timeout: %w", err)
	}
	if interval > timeout {
		return errors.New("interval cannot exceed timeout")
	}
	health.Interval = interval.String()
	health.Timeout = timeout.String()
	if len(health.SuccessStatus) == 0 {
		health.SuccessStatus = []int{200, 204}
	}
	for _, status := range health.SuccessStatus {
		if status < 100 || status > 599 {
			return fmt.Errorf("invalid success status %d", status)
		}
	}
	sort.Ints(health.SuccessStatus)
	return nil
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, errors.New("duration must be positive")
	}
	return duration, nil
}

func confinedPath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes repository root")
	}
	resolved := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes repository root")
	}
	return resolved, nil
}
