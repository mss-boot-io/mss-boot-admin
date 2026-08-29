package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	distribution "github.com/mss-boot-io/mss-boot-admin"
	"gopkg.in/yaml.v3"
)

const (
	PresentationCatalogKind       = "AdminPresentationCatalog"
	PresentationDefinitionVersion = "2"
	embeddedPresentationCatalog   = ".mss/admin-presentation-catalog.yaml"
)

var presentationSurfaceOrder = map[string]int{
	"list": 0, "search": 1, "form": 2, "detail": 3,
}

// PresentationCatalog is the closed, Foundation-owned component and compiled
// query compatibility source used by AdminModule presentation normalization.
// It is embedded in the released mss binary; module specifications may only
// narrow its capabilities and cannot register implementations through data.
type PresentationCatalog struct {
	APIVersion string                      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                      `yaml:"kind" json:"kind"`
	Metadata   PresentationCatalogMetadata `yaml:"metadata" json:"metadata"`
	Spec       PresentationCatalogSpec     `yaml:"spec" json:"spec"`
	SourcePath string                      `yaml:"-" json:"-"`
}

type PresentationCatalogMetadata struct {
	Name              string `yaml:"name" json:"name"`
	DefinitionVersion string `yaml:"definitionVersion" json:"definitionVersion"`
}

type PresentationCatalogSpec struct {
	Components  []PresentationCatalogComponent  `yaml:"components" json:"components"`
	DataSources []PresentationCatalogDataSource `yaml:"dataSources" json:"dataSources"`
	Actions     []PresentationCatalogAction     `yaml:"actions" json:"actions"`
}

type PresentationCatalogComponent struct {
	ID         string   `yaml:"id" json:"id"`
	ValueTypes []string `yaml:"valueTypes" json:"valueTypes"`
	Formats    []string `yaml:"formats" json:"formats"`
	Surfaces   []string `yaml:"surfaces" json:"surfaces"`
	ReadOnly   string   `yaml:"readOnly" json:"readOnly"`
}

type PresentationCatalogDataSource struct {
	ID               string `yaml:"id" json:"id"`
	APIOperation     string `yaml:"apiOperation" json:"apiOperation"`
	PermissionAction string `yaml:"permissionAction" json:"permissionAction"`
	PageSizeOptions  []int  `yaml:"pageSizeOptions" json:"pageSizeOptions"`
	MaxPageSize      int    `yaml:"maxPageSize" json:"maxPageSize"`
	MaxSortFields    int    `yaml:"maxSortFields" json:"maxSortFields"`
}

type PresentationCatalogAction struct {
	ID                 string   `yaml:"id" json:"id"`
	APIOperation       string   `yaml:"apiOperation" json:"apiOperation"`
	PermissionAction   string   `yaml:"permissionAction" json:"permissionAction"`
	Placements         []string `yaml:"placements" json:"placements"`
	Destructive        bool     `yaml:"destructive" json:"destructive"`
	destructiveOmitted bool
}

// DefaultPresentationCatalog parses a fresh copy of the immutable catalog
// embedded into the current Distribution binary.
func DefaultPresentationCatalog() (*PresentationCatalog, error) {
	data, err := distribution.EmbeddedFS().ReadFile(embeddedPresentationCatalog)
	if err != nil {
		return nil, fmt.Errorf("read embedded Admin presentation catalog: %w", err)
	}
	return ParsePresentationCatalog(data, embeddedPresentationCatalog)
}

// ParsePresentationCatalog strictly decodes, normalizes, and validates one
// catalog document. Unknown fields, duplicate YAML keys, and extra documents
// are rejected before module generation can use the catalog.
func ParsePresentationCatalog(data []byte, sourcePath string) (*PresentationCatalog, error) {
	catalog := &PresentationCatalog{SourcePath: sourcePath}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(catalog); err != nil {
		return nil, fmt.Errorf("parse Admin presentation catalog %s: %w", sourcePath, err)
	}
	var additionalDocument yaml.Node
	if err := decoder.Decode(&additionalDocument); err == nil {
		return nil, fmt.Errorf("parse Admin presentation catalog %s: multiple YAML documents are not allowed", sourcePath)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse Admin presentation catalog %s: %w", sourcePath, err)
	}
	if err := trackPresentationCatalogPresence(data, catalog); err != nil {
		return nil, err
	}
	catalog.Normalize()
	if issues := catalog.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return catalog, nil
}

// Normalize makes catalog inventory order deterministic without changing
// compatibility meaning.
func (c *PresentationCatalog) Normalize() {
	c.Metadata.Name = strings.TrimSpace(c.Metadata.Name)
	c.Metadata.DefinitionVersion = strings.TrimSpace(c.Metadata.DefinitionVersion)
	for index := range c.Spec.Components {
		component := &c.Spec.Components[index]
		component.ID = strings.TrimSpace(component.ID)
		component.ReadOnly = strings.TrimSpace(component.ReadOnly)
		trimAndSortStrings(component.ValueTypes)
		trimAndSortStrings(component.Formats)
		trimAndSortByOrder(component.Surfaces, presentationSurfaceOrder)
	}
	for index := range c.Spec.DataSources {
		dataSource := &c.Spec.DataSources[index]
		dataSource.ID = strings.TrimSpace(dataSource.ID)
		dataSource.APIOperation = strings.TrimSpace(dataSource.APIOperation)
		dataSource.PermissionAction = strings.TrimSpace(dataSource.PermissionAction)
		sort.Ints(dataSource.PageSizeOptions)
	}
	for index := range c.Spec.Actions {
		action := &c.Spec.Actions[index]
		action.ID = strings.TrimSpace(action.ID)
		action.APIOperation = strings.TrimSpace(action.APIOperation)
		action.PermissionAction = strings.TrimSpace(action.PermissionAction)
		trimAndSortStrings(action.Placements)
	}
	sort.SliceStable(c.Spec.Components, func(i, j int) bool {
		return c.Spec.Components[i].ID < c.Spec.Components[j].ID
	})
	sort.SliceStable(c.Spec.DataSources, func(i, j int) bool {
		return c.Spec.DataSources[i].ID < c.Spec.DataSources[j].ID
	})
	sort.SliceStable(c.Spec.Actions, func(i, j int) bool {
		return c.Spec.Actions[i].ID < c.Spec.Actions[j].ID
	})
}

// Validate returns stable semantic diagnostics for a normalized catalog.
func (c *PresentationCatalog) Validate() []Issue {
	issues := make([]Issue, 0)
	add := func(path, code, message string) {
		issues = append(issues, Issue{Path: path, Code: code, Message: message})
	}
	if c.APIVersion != ModuleAPIVersion {
		add("apiVersion", "invalid-api-version", "must equal "+ModuleAPIVersion)
	}
	if c.Kind != PresentationCatalogKind {
		add("kind", "invalid-kind", "must equal "+PresentationCatalogKind)
	}
	if !moduleNamePattern.MatchString(c.Metadata.Name) {
		add("metadata.name", "invalid-catalog-name", "must be lower-case kebab-case")
	}
	if c.Metadata.DefinitionVersion != PresentationDefinitionVersion {
		add("metadata.definitionVersion", "unsupported-definition-version", "must equal "+PresentationDefinitionVersion)
	}

	componentSeen := make(map[string]bool, len(c.Spec.Components))
	for index, component := range c.Spec.Components {
		path := "spec.components[" + strconv.Itoa(index) + "]"
		if len(component.ID) < 2 || len(component.ID) > 64 || !moduleNamePattern.MatchString(component.ID) {
			add(path+".id", "invalid-component-id", "must be an unqualified lower-case identifier between 2 and 64 bytes")
		}
		if componentSeen[component.ID] {
			add(path+".id", "duplicate-component", "component id must be unique")
		}
		componentSeen[component.ID] = true
		validateCatalogStrings(path+".valueTypes", component.ValueTypes, []string{"string", "enum", "boolean", "integer", "number", "date", "date-time", "json"}, "value type", add)
		validateCatalogStrings(path+".formats", component.Formats, []string{"plain", "email", "identifier", "date", "date-time"}, "format", add)
		validateCatalogStrings(path+".surfaces", component.Surfaces, []string{"list", "search", "form", "detail"}, "surface", add)
		if !contains([]string{"any", "required", "forbidden"}, component.ReadOnly) {
			add(path+".readOnly", "unsupported-read-only-policy", "supported values are any, required, forbidden")
		}
	}
	if len(c.Spec.Components) == 0 {
		add("spec.components", "required", "at least one component is required")
	}

	dataSourceSeen := make(map[string]bool, len(c.Spec.DataSources))
	for index, dataSource := range c.Spec.DataSources {
		path := "spec.dataSources[" + strconv.Itoa(index) + "]"
		validateCatalogLocalID(path+".id", dataSource.ID, add)
		validateCatalogLocalID(path+".apiOperation", dataSource.APIOperation, add)
		validateCatalogLocalID(path+".permissionAction", dataSource.PermissionAction, add)
		if dataSourceSeen[dataSource.ID] {
			add(path+".id", "duplicate-data-source", "data source id must be unique")
		}
		dataSourceSeen[dataSource.ID] = true
		pageSizeSeen := map[int]bool{}
		for optionIndex, option := range dataSource.PageSizeOptions {
			optionPath := path + ".pageSizeOptions[" + strconv.Itoa(optionIndex) + "]"
			if option < 1 || option > 200 {
				add(optionPath, "page-size-out-of-range", "page size must be between 1 and 200")
			}
			if option > dataSource.MaxPageSize {
				add(optionPath, "page-size-exceeds-maximum", "page size must not exceed maxPageSize")
			}
			if pageSizeSeen[option] {
				add(optionPath, "duplicate-page-size", "page size option must be unique")
			}
			pageSizeSeen[option] = true
		}
		if len(dataSource.PageSizeOptions) == 0 {
			add(path+".pageSizeOptions", "required", "at least one page size option is required")
		}
		if dataSource.MaxPageSize < 1 || dataSource.MaxPageSize > 200 {
			add(path+".maxPageSize", "max-page-size-out-of-range", "maximum page size must be between 1 and 200")
		}
		if dataSource.MaxSortFields < 1 || dataSource.MaxSortFields > 3 {
			add(path+".maxSortFields", "max-sort-fields-out-of-range", "maximum sort fields must be between 1 and 3")
		}
	}
	if len(c.Spec.DataSources) == 0 {
		add("spec.dataSources", "required", "at least one data source is required")
	}

	actionSeen := make(map[string]bool, len(c.Spec.Actions))
	for index, action := range c.Spec.Actions {
		path := "spec.actions[" + strconv.Itoa(index) + "]"
		validateCatalogLocalID(path+".id", action.ID, add)
		validateCatalogLocalID(path+".apiOperation", action.APIOperation, add)
		validateCatalogLocalID(path+".permissionAction", action.PermissionAction, add)
		if actionSeen[action.ID] {
			add(path+".id", "duplicate-action", "action id must be unique")
		}
		actionSeen[action.ID] = true
		validateCatalogStrings(path+".placements", action.Placements, []string{"toolbar", "row", "form", "detail"}, "placement", add)
		if action.destructiveOmitted {
			add(path+".destructive", "required", "destructive must be explicitly declared, including false")
		}
	}
	if len(c.Spec.Actions) == 0 {
		add("spec.actions", "required", "at least one action is required")
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
	return issues
}

func (c *PresentationCatalog) component(id string) (PresentationCatalogComponent, bool) {
	for _, component := range c.Spec.Components {
		if component.ID == id {
			return component, true
		}
	}
	return PresentationCatalogComponent{}, false
}

func (c *PresentationCatalog) dataSource(id string) (PresentationCatalogDataSource, bool) {
	for _, dataSource := range c.Spec.DataSources {
		if dataSource.ID == id {
			return dataSource, true
		}
	}
	return PresentationCatalogDataSource{}, false
}

func (c *PresentationCatalog) action(id string) (PresentationCatalogAction, bool) {
	for _, action := range c.Spec.Actions {
		if action.ID == id {
			return action, true
		}
	}
	return PresentationCatalogAction{}, false
}

func (c *PresentationCatalog) compatibleComponents(valueType, format string, readOnly bool, surface string) []string {
	components := make([]string, 0)
	for _, component := range c.Spec.Components {
		if !contains(component.ValueTypes, valueType) || !contains(component.Formats, format) || !contains(component.Surfaces, surface) {
			continue
		}
		if component.ReadOnly == "required" && !readOnly {
			continue
		}
		if component.ReadOnly == "forbidden" && readOnly {
			continue
		}
		components = append(components, component.ID)
	}
	sort.Strings(components)
	return components
}

func validateCatalogLocalID(path, value string, add func(string, string, string)) {
	if len(value) < 2 || len(value) > 64 || !moduleNamePattern.MatchString(value) || strings.Contains(value, ".") {
		add(path, "invalid-local-id", "must be an unqualified lower-case identifier between 2 and 64 bytes")
	}
}

func validateCatalogStrings(path string, values, supported []string, noun string, add func(string, string, string)) {
	if len(values) == 0 {
		add(path, "required", "at least one "+noun+" is required")
		return
	}
	seen := make(map[string]bool, len(values))
	for index, value := range values {
		itemPath := path + "[" + strconv.Itoa(index) + "]"
		if !contains(supported, value) {
			add(itemPath, "unsupported-"+strings.ReplaceAll(noun, " ", "-"), "unsupported "+noun+" "+value)
		}
		if seen[value] {
			add(itemPath, "duplicate-"+strings.ReplaceAll(noun, " ", "-"), noun+" must be unique")
		}
		seen[value] = true
	}
}

func trimAndSortStrings(values []string) {
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	sort.Strings(values)
}

func trimAndSortByOrder(values []string, order map[string]int) {
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, leftExists := order[values[i]]
		right, rightExists := order[values[j]]
		if leftExists != rightExists {
			return leftExists
		}
		if leftExists && left != right {
			return left < right
		}
		return values[i] < values[j]
	})
}
