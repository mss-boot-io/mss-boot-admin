package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	ModuleAPIVersion = "mss.io/v1alpha1"
	ModuleKind       = "AdminModule"
)

var (
	moduleNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	camelNamePattern   = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	goNamePattern      = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	tableNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	pathPattern        = regexp.MustCompile(`^/[a-z0-9][a-z0-9/-]*$`)
	migrationIDPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)
	eventNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9.-]*$`)
)

// Module is the deterministic management-module specification.
type Module struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   ModuleMetadata `yaml:"metadata" json:"metadata"`
	Spec       ModuleSpec     `yaml:"spec" json:"spec"`
	SourcePath string         `yaml:"-" json:"sourcePath,omitempty"`
}

// ModuleMetadata identifies a module.
type ModuleMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	DisplayName string            `yaml:"displayName" json:"displayName"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ModuleSpec defines backend, frontend, permission, and verification behavior.
type ModuleSpec struct {
	Entity      EntitySpec     `yaml:"entity" json:"entity"`
	API         APISpec        `yaml:"api,omitempty" json:"api,omitempty"`
	Permissions []Permission   `yaml:"permissions" json:"permissions"`
	Ownership   OwnershipSpec  `yaml:"ownership,omitempty" json:"ownership,omitempty"`
	Menu        MenuSpec       `yaml:"menu" json:"menu"`
	UI          UISpec         `yaml:"ui" json:"ui"`
	Workflow    *WorkflowSpec  `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Events      []EventSpec    `yaml:"events,omitempty" json:"events,omitempty"`
	Tests       TestSpec       `yaml:"tests" json:"tests"`
	Generation  GenerationSpec `yaml:"generation,omitempty" json:"generation,omitempty"`
}

// EntitySpec defines persistent data shape.
type EntitySpec struct {
	GoName     string      `yaml:"goName,omitempty" json:"goName,omitempty"`
	Table      string      `yaml:"table" json:"table"`
	IDType     string      `yaml:"idType,omitempty" json:"idType,omitempty"`
	Timestamps *bool       `yaml:"timestamps,omitempty" json:"timestamps,omitempty"`
	SoftDelete *bool       `yaml:"softDelete,omitempty" json:"softDelete,omitempty"`
	Fields     []FieldSpec `yaml:"fields" json:"fields"`
	Indexes    []IndexSpec `yaml:"indexes,omitempty" json:"indexes,omitempty"`
}

// FieldSpec defines one entity field and its generated API/UI behavior.
type FieldSpec struct {
	Name        string         `yaml:"name" json:"name"`
	Column      string         `yaml:"column,omitempty" json:"column,omitempty"`
	GoName      string         `yaml:"goName,omitempty" json:"goName,omitempty"`
	DisplayName string         `yaml:"displayName" json:"displayName"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Type        string         `yaml:"type" json:"type"`
	Required    bool           `yaml:"required,omitempty" json:"required,omitempty"`
	Nullable    bool           `yaml:"nullable,omitempty" json:"nullable,omitempty"`
	Unique      bool           `yaml:"unique,omitempty" json:"unique,omitempty"`
	Index       bool           `yaml:"index,omitempty" json:"index,omitempty"`
	Searchable  bool           `yaml:"searchable,omitempty" json:"searchable,omitempty"`
	Sortable    bool           `yaml:"sortable,omitempty" json:"sortable,omitempty"`
	Filterable  bool           `yaml:"filterable,omitempty" json:"filterable,omitempty"`
	List        *bool          `yaml:"list,omitempty" json:"list,omitempty"`
	Form        *bool          `yaml:"form,omitempty" json:"form,omitempty"`
	Detail      *bool          `yaml:"detail,omitempty" json:"detail,omitempty"`
	Immutable   bool           `yaml:"immutable,omitempty" json:"immutable,omitempty"`
	Default     any            `yaml:"default,omitempty" json:"default,omitempty"`
	Validation  ValidationSpec `yaml:"validation,omitempty" json:"validation,omitempty"`
	EnumValues  []EnumValue    `yaml:"enumValues,omitempty" json:"enumValues,omitempty"`
	Relation    *RelationSpec  `yaml:"relation,omitempty" json:"relation,omitempty"`
	UI          FieldUISpec    `yaml:"ui,omitempty" json:"ui,omitempty"`
}

// ValidationSpec defines generated validation constraints.
type ValidationSpec struct {
	MinLength *int     `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength *int     `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Minimum   *float64 `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum   *float64 `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	Pattern   string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Format    string   `yaml:"format,omitempty" json:"format,omitempty"`
	Precision *int     `yaml:"precision,omitempty" json:"precision,omitempty"`
	Scale     *int     `yaml:"scale,omitempty" json:"scale,omitempty"`
}

// EnumValue defines one enum option.
type EnumValue struct {
	Value   string `yaml:"value" json:"value"`
	Label   string `yaml:"label" json:"label"`
	LabelEn string `yaml:"labelEn,omitempty" json:"labelEn,omitempty"`
	Color   string `yaml:"color,omitempty" json:"color,omitempty"`
}

// RelationSpec defines a cross-module relation.
type RelationSpec struct {
	TargetModule string `yaml:"targetModule" json:"targetModule"`
	TargetField  string `yaml:"targetField" json:"targetField"`
	DisplayField string `yaml:"displayField,omitempty" json:"displayField,omitempty"`
	Cardinality  string `yaml:"cardinality,omitempty" json:"cardinality,omitempty"`
	OnDelete     string `yaml:"onDelete" json:"onDelete"`
}

// FieldUISpec defines the default generated form/list component.
type FieldUISpec struct {
	Component   string `yaml:"component,omitempty" json:"component,omitempty"`
	Width       any    `yaml:"width,omitempty" json:"width,omitempty"`
	Placeholder string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Help        string `yaml:"help,omitempty" json:"help,omitempty"`
	Hidden      bool   `yaml:"hidden,omitempty" json:"hidden,omitempty"`
}

// IndexSpec defines a compound or unique database index.
type IndexSpec struct {
	Name   string   `yaml:"name" json:"name"`
	Fields []string `yaml:"fields" json:"fields"`
	Unique bool     `yaml:"unique,omitempty" json:"unique,omitempty"`
}

// APISpec defines generated resource routes.
type APISpec struct {
	BasePath   string   `yaml:"basePath,omitempty" json:"basePath,omitempty"`
	Version    string   `yaml:"version,omitempty" json:"version,omitempty"`
	Operations []string `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// Permission defines one module action.
type Permission struct {
	Action       string   `yaml:"action" json:"action"`
	DisplayName  string   `yaml:"displayName" json:"displayName"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	DefaultRoles []string `yaml:"defaultRoles,omitempty" json:"defaultRoles,omitempty"`
}

// OwnershipSpec defines row-level ownership behavior.
type OwnershipSpec struct {
	Mode        string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Field       string `yaml:"field,omitempty" json:"field,omitempty"`
	AdminBypass *bool  `yaml:"adminBypass,omitempty" json:"adminBypass,omitempty"`
}

// MenuSpec defines the generated menu and route.
type MenuSpec struct {
	Path          string `yaml:"path" json:"path"`
	DisplayName   string `yaml:"displayName" json:"displayName"`
	DisplayNameEn string `yaml:"displayNameEn,omitempty" json:"displayNameEn,omitempty"`
	Icon          string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Parent        string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Order         int    `yaml:"order,omitempty" json:"order,omitempty"`
	Hidden        bool   `yaml:"hidden,omitempty" json:"hidden,omitempty"`
}

// UISpec defines which standard pages and operations are generated.
type UISpec struct {
	List        bool  `yaml:"list" json:"list"`
	Form        bool  `yaml:"form" json:"form"`
	Detail      bool  `yaml:"detail" json:"detail"`
	Mobile      *bool `yaml:"mobile,omitempty" json:"mobile,omitempty"`
	BatchDelete bool  `yaml:"batchDelete,omitempty" json:"batchDelete,omitempty"`
	Export      bool  `yaml:"export,omitempty" json:"export,omitempty"`
	Import      bool  `yaml:"import,omitempty" json:"import,omitempty"`
}

// WorkflowSpec defines a finite-state workflow for one enum field.
type WorkflowSpec struct {
	Field       string       `yaml:"field" json:"field"`
	Initial     string       `yaml:"initial,omitempty" json:"initial,omitempty"`
	States      []string     `yaml:"states" json:"states"`
	Transitions []Transition `yaml:"transitions" json:"transitions"`
}

// Transition defines an authorized workflow state change.
type Transition struct {
	Name       string   `yaml:"name" json:"name"`
	From       []string `yaml:"from" json:"from"`
	To         string   `yaml:"to" json:"to"`
	Permission string   `yaml:"permission" json:"permission"`
}

// EventSpec defines one generated domain event declaration.
type EventSpec struct {
	Name string `yaml:"name" json:"name"`
	When string `yaml:"when" json:"when"`
}

// TestSpec defines required generated test classes.
type TestSpec struct {
	Unit               bool  `yaml:"unit" json:"unit"`
	API                bool  `yaml:"api" json:"api"`
	E2E                bool  `yaml:"e2e" json:"e2e"`
	PermissionMatrix   bool  `yaml:"permissionMatrix" json:"permissionMatrix"`
	OwnershipIsolation bool  `yaml:"ownershipIsolation,omitempty" json:"ownershipIsolation,omitempty"`
	MigrationUpgrade   *bool `yaml:"migrationUpgrade,omitempty" json:"migrationUpgrade,omitempty"`
}

// GenerationSpec controls generated surfaces.
type GenerationSpec struct {
	MigrationID string `yaml:"migrationID,omitempty" json:"migrationID,omitempty"`
	Backend     *bool  `yaml:"backend,omitempty" json:"backend,omitempty"`
	Frontend    *bool  `yaml:"frontend,omitempty" json:"frontend,omitempty"`
	Docs        *bool  `yaml:"docs,omitempty" json:"docs,omitempty"`
	Tests       *bool  `yaml:"tests,omitempty" json:"tests,omitempty"`
}

// Issue is a stable validation diagnostic.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationError groups all semantic problems in one error.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s [%s]: %s", issue.Path, issue.Code, issue.Message))
	}
	return strings.Join(parts, "; ")
}

// LoadModule reads, normalizes, and validates one AdminModule YAML file.
func LoadModule(path string) (*Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module spec %s: %w", path, err)
	}
	return ParseModule(data, path)
}

// ParseModule strictly decodes, normalizes, and validates one AdminModule
// document. Unknown fields, duplicate mapping keys, and additional YAML
// documents are rejected so generation never proceeds from ambiguous input.
func ParseModule(data []byte, sourcePath string) (*Module, error) {
	module := &Module{SourcePath: sourcePath}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(module); err != nil {
		return nil, fmt.Errorf("parse module spec %s: %w", sourcePath, err)
	}
	var additionalDocument yaml.Node
	if err := decoder.Decode(&additionalDocument); err == nil {
		return nil, fmt.Errorf("parse module spec %s: multiple YAML documents are not allowed", sourcePath)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse module spec %s: %w", sourcePath, err)
	}
	module.Normalize()
	if issues := module.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return module, nil
}

// Normalize fills deterministic defaults and derived names.
func (m *Module) Normalize() {
	m.Metadata.Name = strings.TrimSpace(m.Metadata.Name)
	m.Metadata.DisplayName = strings.TrimSpace(m.Metadata.DisplayName)
	m.Metadata.Description = strings.TrimSpace(m.Metadata.Description)
	if m.Spec.Entity.GoName == "" {
		m.Spec.Entity.GoName = PascalCase(m.Metadata.Name)
	}
	if m.Spec.Entity.IDType == "" {
		m.Spec.Entity.IDType = "uuid"
	}
	if m.Spec.Entity.Timestamps == nil {
		m.Spec.Entity.Timestamps = boolPointer(true)
	}
	if m.Spec.Entity.SoftDelete == nil {
		m.Spec.Entity.SoftDelete = boolPointer(true)
	}
	if m.Spec.API.BasePath == "" && m.Metadata.Name != "" {
		m.Spec.API.BasePath = "/" + simplePlural(m.Metadata.Name)
	}
	if m.Spec.API.Version == "" {
		m.Spec.API.Version = "v1"
	}
	if len(m.Spec.API.Operations) == 0 {
		m.Spec.API.Operations = []string{"list", "get", "create", "update", "delete"}
	}
	if m.Spec.Ownership.Mode == "" {
		m.Spec.Ownership.Mode = "none"
	}
	if m.Spec.Ownership.AdminBypass == nil {
		m.Spec.Ownership.AdminBypass = boolPointer(true)
	}
	if m.Spec.UI.Mobile == nil {
		m.Spec.UI.Mobile = boolPointer(true)
	}
	if m.Spec.Tests.MigrationUpgrade == nil {
		m.Spec.Tests.MigrationUpgrade = boolPointer(true)
	}
	if m.Spec.Generation.Backend == nil {
		m.Spec.Generation.Backend = boolPointer(true)
	}
	if m.Spec.Generation.Frontend == nil {
		m.Spec.Generation.Frontend = boolPointer(true)
	}
	if m.Spec.Generation.Docs == nil {
		m.Spec.Generation.Docs = boolPointer(true)
	}
	if m.Spec.Generation.Tests == nil {
		m.Spec.Generation.Tests = boolPointer(true)
	}

	for i := range m.Spec.Entity.Fields {
		field := &m.Spec.Entity.Fields[i]
		field.Name = strings.TrimSpace(field.Name)
		if field.Column == "" {
			field.Column = SnakeCase(field.Name)
		}
		if field.GoName == "" {
			field.GoName = PascalCase(field.Name)
		}
		if field.List == nil {
			field.List = boolPointer(true)
		}
		if field.Form == nil {
			field.Form = boolPointer(true)
		}
		if field.Detail == nil {
			field.Detail = boolPointer(true)
		}
		if field.Relation != nil && field.Relation.Cardinality == "" {
			field.Relation.Cardinality = "many-to-one"
		}
	}

	for i := range m.Spec.Permissions {
		sort.Strings(m.Spec.Permissions[i].DefaultRoles)
	}
	sort.SliceStable(m.Spec.Permissions, func(i, j int) bool {
		return m.Spec.Permissions[i].Action < m.Spec.Permissions[j].Action
	})
	sort.Strings(m.Spec.API.Operations)
	sort.SliceStable(m.Spec.Entity.Indexes, func(i, j int) bool {
		return m.Spec.Entity.Indexes[i].Name < m.Spec.Entity.Indexes[j].Name
	})
	sort.SliceStable(m.Spec.Events, func(i, j int) bool {
		return m.Spec.Events[i].Name < m.Spec.Events[j].Name
	})
}

// Validate returns all semantic issues instead of failing at the first problem.
func (m *Module) Validate() []Issue {
	issues := make([]Issue, 0)
	add := func(path, code, message string) {
		issues = append(issues, Issue{Path: path, Code: code, Message: message})
	}

	if m.APIVersion != ModuleAPIVersion {
		add("apiVersion", "invalid-api-version", "must equal "+ModuleAPIVersion)
	}
	if m.Kind != ModuleKind {
		add("kind", "invalid-kind", "must equal "+ModuleKind)
	}
	if !moduleNamePattern.MatchString(m.Metadata.Name) {
		add("metadata.name", "invalid-module-name", "must be lower-case kebab-case")
	}
	if m.Metadata.DisplayName == "" {
		add("metadata.displayName", "required", "display name is required")
	}
	if !goNamePattern.MatchString(m.Spec.Entity.GoName) {
		add("spec.entity.goName", "invalid-go-name", "must be an exported Go identifier")
	}
	if !tableNamePattern.MatchString(m.Spec.Entity.Table) {
		add("spec.entity.table", "invalid-table-name", "must be lower-case snake_case")
	}
	if !contains([]string{"uuid", "string", "int64"}, m.Spec.Entity.IDType) {
		add("spec.entity.idType", "unsupported-id-type", "supported values are uuid, string, int64")
	}
	if len(m.Spec.Entity.Fields) == 0 {
		add("spec.entity.fields", "required", "at least one field is required")
	}

	fieldByName := make(map[string]FieldSpec, len(m.Spec.Entity.Fields))
	columnOwner := make(map[string]string, len(m.Spec.Entity.Fields))
	goNameOwner := make(map[string]string, len(m.Spec.Entity.Fields))
	for index, field := range m.Spec.Entity.Fields {
		path := "spec.entity.fields[" + strconv.Itoa(index) + "]"
		if !camelNamePattern.MatchString(field.Name) {
			add(path+".name", "invalid-field-name", "must be lower camelCase")
		}
		if _, exists := fieldByName[field.Name]; exists {
			add(path+".name", "duplicate-field", "field name must be unique")
		}
		fieldByName[field.Name] = field
		if !tableNamePattern.MatchString(field.Column) {
			add(path+".column", "invalid-column-name", "must be lower-case snake_case")
		}
		if previous, exists := columnOwner[field.Column]; exists {
			add(path+".column", "duplicate-column", "column is already used by field "+previous)
		} else {
			columnOwner[field.Column] = field.Name
		}
		if !goNamePattern.MatchString(field.GoName) {
			add(path+".goName", "invalid-go-name", "must be an exported Go identifier")
		}
		if previous, exists := goNameOwner[field.GoName]; exists {
			add(path+".goName", "duplicate-go-name", "Go field is already used by "+previous)
		} else {
			goNameOwner[field.GoName] = field.Name
		}
		if field.DisplayName == "" {
			add(path+".displayName", "required", "display name is required")
		}
		if !contains(supportedFieldTypes(), field.Type) {
			add(path+".type", "unsupported-field-type", "unsupported field type "+field.Type)
		}
		if field.Required && field.Nullable {
			add(path, "conflicting-nullability", "required and nullable cannot both be true")
		}
		if field.Type == "enum" {
			validateEnum(path, field, add)
		} else if len(field.EnumValues) > 0 {
			add(path+".enumValues", "unexpected-enum-values", "enumValues are only valid for enum fields")
		}
		if field.Type == "relation" {
			validateRelation(path, field, add)
		} else if field.Relation != nil {
			add(path+".relation", "unexpected-relation", "relation is only valid for relation fields")
		}
		validateFieldConstraint(path, field, add)
	}

	indexNameSeen := make(map[string]bool, len(m.Spec.Entity.Indexes))
	for index, databaseIndex := range m.Spec.Entity.Indexes {
		path := "spec.entity.indexes[" + strconv.Itoa(index) + "]"
		if !tableNamePattern.MatchString(databaseIndex.Name) {
			add(path+".name", "invalid-index-name", "must be lower-case snake_case")
		}
		if len(databaseIndex.Fields) == 0 {
			add(path+".fields", "required", "index must contain at least one field")
		}
		if indexNameSeen[databaseIndex.Name] {
			add(path+".name", "duplicate-index", "index name must be unique")
		}
		indexNameSeen[databaseIndex.Name] = true
		for _, fieldName := range databaseIndex.Fields {
			if _, exists := fieldByName[fieldName]; !exists {
				add(path+".fields", "unknown-field", "index references unknown field "+fieldName)
			}
		}
	}

	if !pathPattern.MatchString(m.Spec.API.BasePath) {
		add("spec.api.basePath", "invalid-api-path", "must be a lower-case absolute resource path")
	}
	if !regexp.MustCompile(`^v[0-9]+$`).MatchString(m.Spec.API.Version) {
		add("spec.api.version", "invalid-api-version", "must use v<number> format")
	}
	validOperations := []string{"list", "get", "create", "update", "delete", "export", "import"}
	operationSeen := map[string]bool{}
	for index, operation := range m.Spec.API.Operations {
		if !contains(validOperations, operation) {
			add("spec.api.operations["+strconv.Itoa(index)+"]", "unsupported-operation", "unsupported operation "+operation)
		}
		if operationSeen[operation] {
			add("spec.api.operations["+strconv.Itoa(index)+"]", "duplicate-operation", "operation must be unique")
		}
		operationSeen[operation] = true
	}

	permissionSeen := map[string]bool{}
	for index, permission := range m.Spec.Permissions {
		path := "spec.permissions[" + strconv.Itoa(index) + "]"
		if !moduleNamePattern.MatchString(permission.Action) {
			add(path+".action", "invalid-permission-action", "must be lower-case kebab-case")
		}
		if permissionSeen[permission.Action] {
			add(path+".action", "duplicate-permission", "permission action must be unique")
		}
		permissionSeen[permission.Action] = true
		if permission.DisplayName == "" {
			add(path+".displayName", "required", "display name is required")
		}
	}
	for _, operation := range m.Spec.API.Operations {
		requiredPermission := operationPermission(operation)
		if requiredPermission != "" && !permissionSeen[requiredPermission] {
			add("spec.permissions", "missing-operation-permission", fmt.Sprintf("operation %s requires permission %s", operation, requiredPermission))
		}
	}

	if !contains([]string{"none", "creator", "department", "custom"}, m.Spec.Ownership.Mode) {
		add("spec.ownership.mode", "unsupported-ownership", "supported values are none, creator, department, custom")
	}
	if m.Spec.Ownership.Mode != "none" {
		if m.Spec.Ownership.Field == "" {
			add("spec.ownership.field", "required", "ownership field is required for non-none ownership")
		} else if _, exists := fieldByName[m.Spec.Ownership.Field]; !exists {
			add("spec.ownership.field", "unknown-field", "ownership references an unknown field")
		}
	}

	if !pathPattern.MatchString(m.Spec.Menu.Path) {
		add("spec.menu.path", "invalid-menu-path", "must be a lower-case absolute route")
	}
	if m.Spec.Menu.DisplayName == "" {
		add("spec.menu.displayName", "required", "menu display name is required")
	}
	if m.Spec.Menu.Parent != "" && !pathPattern.MatchString(m.Spec.Menu.Parent) {
		add("spec.menu.parent", "invalid-menu-parent", "parent must be an absolute lower-case route")
	}

	if m.Spec.Workflow != nil {
		validateWorkflow(m.Spec.Workflow, fieldByName, permissionSeen, add)
	}
	if (m.Spec.Generation.Backend == nil || *m.Spec.Generation.Backend) && m.Spec.Generation.MigrationID == "" {
		add("spec.generation.migrationID", "required", "backend generation requires an explicit complete migration ID")
	} else if m.Spec.Generation.MigrationID != "" && !migrationIDPattern.MatchString(m.Spec.Generation.MigrationID) {
		add("spec.generation.migrationID", "invalid-migration-id", "must be a complete decimal identifier without leading zeroes")
	}

	eventSeen := map[string]bool{}
	eventTriggerSeen := map[string]bool{}
	for index, event := range m.Spec.Events {
		path := "spec.events[" + strconv.Itoa(index) + "]"
		if event.Name == "" {
			add(path+".name", "required", "event name is required")
		} else if !eventNamePattern.MatchString(event.Name) {
			add(path+".name", "invalid-event-name", "must start with a lower-case letter and contain only lower-case letters, digits, dots, or hyphens")
		}
		if eventSeen[event.Name] {
			add(path+".name", "duplicate-event", "event name must be unique")
		}
		eventSeen[event.Name] = true
		if !contains([]string{"created", "updated", "deleted", "workflow-transition"}, event.When) {
			add(path+".when", "unsupported-event-trigger", "unsupported event trigger "+event.When)
		} else if eventTriggerSeen[event.When] {
			add(path+".when", "duplicate-event-trigger", "event trigger must be unique")
		}
		eventTriggerSeen[event.When] = true
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
	return issues
}

// YAML returns normalized deterministic YAML.
func (m *Module) YAML() ([]byte, error) {
	clone := *m
	clone.SourcePath = ""
	return yaml.Marshal(&clone)
}

// PermissionCode returns the stable fully-qualified permission code.
func (m *Module) PermissionCode(action string) string {
	return m.Metadata.Name + ":" + action
}

// Field returns one field by its lower-camel name.
func (m *Module) Field(name string) (FieldSpec, bool) {
	for _, field := range m.Spec.Entity.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return FieldSpec{}, false
}

// PascalCase converts kebab, snake, and camel names to an exported Go identifier.
func PascalCase(value string) string {
	words := splitIdentifier(value)
	initialisms := map[string]string{
		"api": "API", "css": "CSS", "html": "HTML", "http": "HTTP",
		"https": "HTTPS", "id": "ID", "ip": "IP", "json": "JSON",
		"oauth": "OAuth", "sql": "SQL", "ui": "UI", "uri": "URI",
		"url": "URL", "uuid": "UUID",
	}
	for index, word := range words {
		if replacement, exists := initialisms[word]; exists {
			words[index] = replacement
			continue
		}
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}
	return strings.Join(words, "")
}

// SnakeCase converts kebab, snake, and camel names to lower snake_case.
func SnakeCase(value string) string {
	return strings.Join(splitIdentifier(value), "_")
}

func splitIdentifier(value string) []string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return nil
	}
	words := make([]string, 0, 4)
	current := make([]rune, 0, len(runes))
	flush := func() {
		if len(current) == 0 {
			return
		}
		words = append(words, strings.ToLower(string(current)))
		current = current[:0]
	}
	for index, char := range runes {
		if char == '-' || char == '_' || unicode.IsSpace(char) {
			flush()
			continue
		}
		if unicode.IsUpper(char) && len(current) > 0 {
			previous := current[len(current)-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				(unicode.IsUpper(previous) && nextIsLower) {
				flush()
			}
		}
		current = append(current, char)
	}
	flush()
	return words
}

func validateEnum(path string, field FieldSpec, add func(string, string, string)) {
	if len(field.EnumValues) == 0 {
		add(path+".enumValues", "required", "enum field requires at least one enum value")
		return
	}
	seen := map[string]bool{}
	for index, value := range field.EnumValues {
		valuePath := path + ".enumValues[" + strconv.Itoa(index) + "]"
		if value.Value == "" || !moduleNamePattern.MatchString(value.Value) {
			add(valuePath+".value", "invalid-enum-value", "must be lower-case kebab-case")
		}
		if seen[value.Value] {
			add(valuePath+".value", "duplicate-enum-value", "enum value must be unique")
		}
		seen[value.Value] = true
		if value.Label == "" {
			add(valuePath+".label", "required", "enum label is required")
		}
	}
	if field.Default != nil {
		defaultValue := fmt.Sprint(field.Default)
		if !seen[defaultValue] {
			add(path+".default", "unknown-enum-default", "default must reference a declared enum value")
		}
	}
}

func validateRelation(path string, field FieldSpec, add func(string, string, string)) {
	if field.Relation == nil {
		add(path+".relation", "required", "relation field requires relation configuration")
		return
	}
	if !moduleNamePattern.MatchString(field.Relation.TargetModule) {
		add(path+".relation.targetModule", "invalid-target-module", "must be lower-case kebab-case")
	}
	if !camelNamePattern.MatchString(field.Relation.TargetField) {
		add(path+".relation.targetField", "invalid-target-field", "must be lower camelCase")
	}
	if !contains([]string{"many-to-one", "one-to-one", "one-to-many", "many-to-many"}, field.Relation.Cardinality) {
		add(path+".relation.cardinality", "unsupported-cardinality", "unsupported relation cardinality")
	}
	if !contains([]string{"restrict", "cascade", "set-null", "no-action"}, field.Relation.OnDelete) {
		add(path+".relation.onDelete", "unsupported-delete-policy", "unsupported relation delete policy")
	}
	if field.Relation.OnDelete == "set-null" && !field.Nullable {
		add(path, "set-null-requires-nullable", "set-null relation must be nullable")
	}
}

func validateFieldConstraint(path string, field FieldSpec, add func(string, string, string)) {
	constraint := field.Validation
	if constraint.MinLength != nil && *constraint.MinLength < 0 {
		add(path+".validation.minLength", "invalid-min-length", "must be non-negative")
	}
	if constraint.MaxLength != nil && *constraint.MaxLength < 1 {
		add(path+".validation.maxLength", "invalid-max-length", "must be positive")
	}
	if constraint.MinLength != nil && constraint.MaxLength != nil && *constraint.MinLength > *constraint.MaxLength {
		add(path+".validation", "invalid-length-range", "minLength cannot exceed maxLength")
	}
	if constraint.Minimum != nil && constraint.Maximum != nil && *constraint.Minimum > *constraint.Maximum {
		add(path+".validation", "invalid-number-range", "minimum cannot exceed maximum")
	}
	if constraint.Pattern != "" {
		if _, err := regexp.Compile(constraint.Pattern); err != nil {
			add(path+".validation.pattern", "invalid-pattern", err.Error())
		}
	}
	if constraint.Precision != nil && (*constraint.Precision < 1 || *constraint.Precision > 65) {
		add(path+".validation.precision", "invalid-precision", "must be between 1 and 65")
	}
	if constraint.Scale != nil && (*constraint.Scale < 0 || *constraint.Scale > 30) {
		add(path+".validation.scale", "invalid-scale", "must be between 0 and 30")
	}
	if constraint.Precision != nil && constraint.Scale != nil && *constraint.Scale > *constraint.Precision {
		add(path+".validation", "invalid-decimal-shape", "scale cannot exceed precision")
	}
	if field.Type == "decimal" && constraint.Precision == nil {
		add(path+".validation.precision", "required", "decimal field requires precision")
	}
}

func validateWorkflow(
	workflow *WorkflowSpec,
	fieldByName map[string]FieldSpec,
	permissionSeen map[string]bool,
	add func(string, string, string),
) {
	field, exists := fieldByName[workflow.Field]
	if !exists {
		add("spec.workflow.field", "unknown-field", "workflow references an unknown field")
	} else if field.Type != "enum" {
		add("spec.workflow.field", "invalid-workflow-field", "workflow field must be an enum")
	}
	stateSeen := map[string]bool{}
	for index, state := range workflow.States {
		path := "spec.workflow.states[" + strconv.Itoa(index) + "]"
		if !moduleNamePattern.MatchString(state) {
			add(path, "invalid-workflow-state", "state must be lower-case kebab-case")
		}
		if stateSeen[state] {
			add(path, "duplicate-workflow-state", "state must be unique")
		}
		stateSeen[state] = true
	}
	if workflow.Initial != "" && !stateSeen[workflow.Initial] {
		add("spec.workflow.initial", "unknown-initial-state", "initial state is not declared")
	}
	transitionSeen := map[string]bool{}
	for index, transition := range workflow.Transitions {
		path := "spec.workflow.transitions[" + strconv.Itoa(index) + "]"
		if transitionSeen[transition.Name] {
			add(path+".name", "duplicate-transition", "transition name must be unique")
		}
		transitionSeen[transition.Name] = true
		for _, from := range transition.From {
			if !stateSeen[from] {
				add(path+".from", "unknown-source-state", "source state "+from+" is not declared")
			}
		}
		if !stateSeen[transition.To] {
			add(path+".to", "unknown-target-state", "target state is not declared")
		}
		if !permissionSeen[transition.Permission] {
			add(path+".permission", "unknown-permission", "transition permission is not declared")
		}
	}
}

func operationPermission(operation string) string {
	switch operation {
	case "list":
		return "list"
	case "get":
		return "read"
	default:
		return operation
	}
}

func supportedFieldTypes() []string {
	return []string{
		"string", "text", "int", "int64", "uint", "float64", "decimal", "bool",
		"date", "datetime", "uuid", "json", "enum", "relation", "file", "files",
	}
}

func simplePlural(value string) string {
	if strings.HasSuffix(value, "y") && len(value) > 1 {
		previous := value[len(value)-2]
		if !strings.ContainsRune("aeiou", rune(previous)) {
			return value[:len(value)-1] + "ies"
		}
	}
	if strings.HasSuffix(value, "s") || strings.HasSuffix(value, "x") || strings.HasSuffix(value, "ch") || strings.HasSuffix(value, "sh") {
		return value + "es"
	}
	return value + "s"
}

func boolPointer(value bool) *bool {
	return &value
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// IsValidationError reports whether err contains semantic specification issues.
func IsValidationError(err error) bool {
	var validationError *ValidationError
	return errors.As(err, &validationError)
}
