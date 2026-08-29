package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	CorePagePresentationKind = "AdminCorePagePresentation"
	userListCoreBindingID    = "administration.users"
)

// CorePagePresentation is the strict, Foundation-only source for a handwritten
// core page. Authority, transport and renderer implementations are absent from
// this document and are compiled from a closed binding below.
type CorePagePresentation struct {
	APIVersion string                       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                       `yaml:"kind" json:"kind"`
	Metadata   CorePagePresentationMetadata `yaml:"metadata" json:"metadata"`
	Spec       CorePagePresentationSpec     `yaml:"spec" json:"spec"`
	SourcePath string                       `yaml:"-" json:"sourcePath,omitempty"`
}

type CorePagePresentationMetadata struct {
	Name string `yaml:"name" json:"name"`
}

// CorePagePresentationSpec intentionally exposes only presentation defaults.
// In particular it has no permission, route, URL, method, import, guard,
// handler, operation or executable-expression property.
type CorePagePresentationSpec struct {
	Binding           string                               `yaml:"binding" json:"binding"`
	PageKey           string                               `yaml:"pageKey" json:"pageKey"`
	DefinitionVersion string                               `yaml:"definitionVersion" json:"definitionVersion"`
	Title             PresentationLocalizedText            `yaml:"title" json:"title"`
	List              CorePagePresentationListSource       `yaml:"list" json:"list"`
	Search            CorePagePresentationSearchSource     `yaml:"search" json:"search"`
	Form              []CorePagePresentationFieldSentinel  `yaml:"form" json:"form"`
	Detail            []CorePagePresentationFieldSentinel  `yaml:"detail" json:"detail"`
	Actions           []CorePagePresentationActionSentinel `yaml:"actions" json:"actions"`
}

type CorePagePresentationListSource struct {
	Density  string `yaml:"density" json:"density"`
	PageSize int    `yaml:"pageSize" json:"pageSize"`
}

type CorePagePresentationSearchSource struct {
	Fields []string `yaml:"fields" json:"fields"`
}

// Empty sentinel types make form, detail and action collections structurally
// closed while allowing the source to prove they were explicitly declared as
// empty arrays.
type CorePagePresentationFieldSentinel struct{}
type CorePagePresentationActionSentinel struct{}

type corePagePresentationBinding struct {
	ID                  string
	PageKey             string
	DataSourceID        string
	RequiredPermissions []string
	PageSizeOptions     []int
	MaxPageSize         int
	MaxSortFields       int
	ListFields          []corePagePresentationFieldBinding
	SearchFields        []string
}

type corePagePresentationFieldBinding struct {
	ID              string
	Label           PresentationLocalizedText
	ValueType       string
	Format          string
	Required        bool
	Nullable        bool
	Searchable      bool
	Filterable      bool
	ListComponent   string
	SearchComponent string
	ListWidth       int
	EnumValues      []NormalizedPresentationEnumValue
	Validation      NormalizedPresentationValidation
}

var foundationCorePagePresentationBindings = map[string]corePagePresentationBinding{
	userListCoreBindingID: {
		ID:                  userListCoreBindingID,
		PageKey:             "user.list",
		DataSourceID:        "user.list",
		RequiredPermissions: []string{"/users"},
		PageSizeOptions:     []int{20, 50, 100},
		MaxPageSize:         100,
		MaxSortFields:       0,
		SearchFields:        []string{"name", "status"},
		ListFields: []corePagePresentationFieldBinding{
			{
				ID: "username", Label: corePresentationText("账号", "Account"),
				ValueType: "string", Format: "identifier", Required: true,
				ListComponent: "user-identity", ListWidth: 210,
			},
			{
				ID: "name", Label: corePresentationText("姓名", "Name"),
				ValueType: "string", Format: "plain", Nullable: true, Searchable: true,
				ListComponent: "text", SearchComponent: "input",
			},
			{
				ID: "email", Label: corePresentationText("邮箱", "Email"),
				ValueType: "string", Format: "email", Nullable: true,
				ListComponent: "text",
			},
			{
				ID: "roleName", Label: corePresentationText("角色", "Role"),
				ValueType: "string", Format: "plain", Nullable: true,
				ListComponent: "user-role", ListWidth: 150,
			},
			{
				ID: "organization", Label: corePresentationText("组织", "Organization"),
				ValueType: "string", Format: "plain", Nullable: true,
				ListComponent: "user-organization",
			},
			{
				ID: "status", Label: corePresentationText("状态", "Status"),
				ValueType: "enum", Format: "plain", Required: true, Filterable: true,
				ListComponent: "status-tag", SearchComponent: "status-filter", ListWidth: 120,
				EnumValues: []NormalizedPresentationEnumValue{
					{Value: "disabled", Label: corePresentationText("禁用", "Disabled"), Color: "red"},
					{Value: "enabled", Label: corePresentationText("启用", "Enabled"), Color: "green"},
					{Value: "locked", Label: corePresentationText("锁定", "Locked"), Color: "orange"},
				},
			},
		},
	},
}

func corePresentationText(zhCN, enUS string) PresentationLocalizedText {
	return PresentationLocalizedText{ZhCN: zhCN, EnUS: enUS}
}

// ParseCorePagePresentation strictly decodes and validates one Foundation core
// page source. Unknown keys, duplicate keys, YAML merge keys, nulls and extra
// documents fail closed.
func ParseCorePagePresentation(data []byte, sourcePath string) (*CorePagePresentation, error) {
	document := &CorePagePresentation{SourcePath: sourcePath}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(document); err != nil {
		return nil, fmt.Errorf("parse core page presentation %s: %w", sourcePath, err)
	}
	var additional yaml.Node
	if err := decoder.Decode(&additional); err == nil {
		return nil, fmt.Errorf("parse core page presentation %s: multiple YAML documents are not allowed", sourcePath)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse core page presentation %s: %w", sourcePath, err)
	}
	root, err := presentationYAMLRoot(data)
	if err != nil {
		return nil, err
	}
	if issues := presentationYAMLMappingKeyIssues("", root); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	if issues := presentationYAMLMergeKeyIssues("", root); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	if issues := presentationYAMLNullIssues("", root); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	document.normalize()
	if issues := document.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return document, nil
}

func (d *CorePagePresentation) normalize() {
	d.APIVersion = strings.TrimSpace(d.APIVersion)
	d.Kind = strings.TrimSpace(d.Kind)
	d.Metadata.Name = strings.TrimSpace(d.Metadata.Name)
	d.Spec.Binding = strings.TrimSpace(d.Spec.Binding)
	d.Spec.PageKey = strings.TrimSpace(d.Spec.PageKey)
	d.Spec.DefinitionVersion = strings.TrimSpace(d.Spec.DefinitionVersion)
	d.Spec.Title.normalize()
	d.Spec.List.Density = strings.TrimSpace(d.Spec.List.Density)
	for index := range d.Spec.Search.Fields {
		d.Spec.Search.Fields[index] = strings.TrimSpace(d.Spec.Search.Fields[index])
	}
}

// Validate returns deterministic semantic diagnostics for one normalized core
// source. Binding facts are immutable Foundation code, never source values.
func (d *CorePagePresentation) Validate() []Issue {
	issues := make([]Issue, 0)
	add := func(path, code, message string) {
		issues = append(issues, Issue{Path: path, Code: code, Message: message})
	}
	if d == nil {
		return []Issue{{Path: "corePagePresentation", Code: "document-required", Message: "core page presentation is required"}}
	}
	if d.APIVersion != ModuleAPIVersion {
		add("apiVersion", "invalid-api-version", "must equal "+ModuleAPIVersion)
	}
	if d.Kind != CorePagePresentationKind {
		add("kind", "invalid-kind", "must equal "+CorePagePresentationKind)
	}
	if !moduleNamePattern.MatchString(d.Metadata.Name) || len(d.Metadata.Name) > 120 {
		add("metadata.name", "invalid-core-page-name", "must be lower-case kebab-case and not exceed 120 bytes")
	}
	binding, exists := foundationCorePagePresentationBindings[d.Spec.Binding]
	if !exists {
		add("spec.binding", "unknown-core-page-binding", "binding is not compiled into the Foundation")
	}
	if d.Spec.DefinitionVersion != PresentationDefinitionVersion {
		add("spec.definitionVersion", "unsupported-definition-version", "must equal "+PresentationDefinitionVersion)
	}
	if !presentationPageKeyPattern.MatchString(d.Spec.PageKey) || len(d.Spec.PageKey) > 120 {
		add("spec.pageKey", "invalid-page-key", "must be a dotted lower-case identifier not exceeding 120 bytes")
	} else if exists && d.Spec.PageKey != binding.PageKey {
		add("spec.pageKey", "binding-page-key-mismatch", "page key does not match the compiled Foundation binding")
	}
	validateLocalizedText("spec.title", d.Spec.Title, true, add)
	if !contains([]string{"compact", "middle", "large"}, d.Spec.List.Density) {
		add("spec.list.density", "unsupported-density", "supported values are compact, middle, large")
	}
	if exists {
		if !containsInt(binding.PageSizeOptions, d.Spec.List.PageSize) {
			add("spec.list.pageSize", "unsupported-page-size", "page size must be one of the compiled Foundation binding options")
		}
		if d.Spec.List.PageSize > binding.MaxPageSize {
			add("spec.list.pageSize", "page-size-exceeds-maximum", "page size exceeds the compiled Foundation binding maximum")
		}
	}
	if d.Spec.Search.Fields == nil {
		add("spec.search.fields", "required", "search fields must be explicitly declared")
	}
	seen := map[string]bool{}
	for index, field := range d.Spec.Search.Fields {
		path := "spec.search.fields[" + strconv.Itoa(index) + "]"
		if isSensitiveCorePageField(field) {
			add(path, "sensitive-field-forbidden", "identity, authority and credential fields cannot be presentation-configurable")
		}
		if seen[field] {
			add(path, "duplicate-search-field", "search field must be unique")
		}
		seen[field] = true
	}
	if exists && !equalStrings(d.Spec.Search.Fields, binding.SearchFields) {
		add("spec.search.fields", "binding-search-contract-mismatch", "search fields must exactly match the compiled Foundation binding")
	}
	if d.Spec.Form == nil {
		add("spec.form", "required", "form must be explicitly declared as an empty array")
	} else if len(d.Spec.Form) != 0 {
		add("spec.form", "core-form-forbidden", "the first core page contract does not expose configurable form fields")
	}
	if d.Spec.Detail == nil {
		add("spec.detail", "required", "detail must be explicitly declared as an empty array")
	} else if len(d.Spec.Detail) != 0 {
		add("spec.detail", "core-detail-forbidden", "the first core page contract does not expose configurable detail fields")
	}
	if d.Spec.Actions == nil {
		add("spec.actions", "required", "actions must be explicitly declared as an empty array")
	} else if len(d.Spec.Actions) != 0 {
		add("spec.actions", "core-actions-forbidden", "security-sensitive user actions are not presentation-configurable")
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
	return issues
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isSensitiveCorePageField(value string) bool {
	var normalized strings.Builder
	for _, current := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			normalized.WriteRune(current)
		}
	}
	field := normalized.String()
	if field == "id" || strings.HasSuffix(field, "id") {
		return true
	}
	for _, fragment := range []string{
		"password", "passwd", "credential", "secret", "salt", "hash",
		"root", "token", "session", "oauth", "principal", "permission", "authority",
	} {
		if strings.Contains(field, fragment) {
			return true
		}
	}
	return false
}

// NormalizePresentation compiles a core source through its immutable binding
// into the same canonical v2 manifest used by AdminModule presentation output.
func (d *CorePagePresentation) NormalizePresentation() (*NormalizedPresentationManifest, error) {
	if d == nil {
		return nil, errors.New("core page presentation is required")
	}
	d.normalize()
	if issues := d.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	binding := foundationCorePagePresentationBindings[d.Spec.Binding]
	fields := make([]NormalizedPresentationField, 0, len(binding.ListFields))
	list := make([]NormalizedCompleteField, 0, len(binding.ListFields))
	search := make([]NormalizedCompleteField, 0, len(binding.SearchFields))
	componentSet := map[string]bool{}
	fieldByID := make(map[string]corePagePresentationFieldBinding, len(binding.ListFields))
	for index, field := range binding.ListFields {
		fieldByID[field.ID] = field
		surfaces := []string{"list"}
		surfaceComponents := []NormalizedPresentationSurfaceComponents{{Surface: "list", Components: []string{field.ListComponent}}}
		components := []string{field.ListComponent}
		componentSet[field.ListComponent] = true
		if field.SearchComponent != "" {
			surfaces = append(surfaces, "search")
			surfaceComponents = append(surfaceComponents, NormalizedPresentationSurfaceComponents{Surface: "search", Components: []string{field.SearchComponent}})
			components = append(components, field.SearchComponent)
			componentSet[field.SearchComponent] = true
		}
		sort.Strings(components)
		enumValues := append([]NormalizedPresentationEnumValue{}, field.EnumValues...)
		fields = append(fields, NormalizedPresentationField{
			ID: field.ID, Label: field.Label, ValueType: field.ValueType, Format: field.Format,
			Required: field.Required, Nullable: field.Nullable, ReadOnly: true,
			Searchable: field.Searchable, Sortable: false, Filterable: field.Filterable,
			Surfaces: surfaces, Components: components, SurfaceComponents: surfaceComponents,
			EnumValues: enumValues, Validation: field.Validation,
		})
		var width *int
		if field.ListWidth > 0 {
			value := field.ListWidth
			width = &value
		}
		list = append(list, NormalizedCompleteField{
			Field: field.ID, Label: field.Label, Component: field.ListComponent,
			Order: (index + 1) * 10, Hidden: false, Width: width,
		})
	}
	for index, fieldID := range binding.SearchFields {
		field := fieldByID[fieldID]
		search = append(search, NormalizedCompleteField{
			Field: field.ID, Label: field.Label, Component: field.SearchComponent,
			Order: (index + 1) * 10, Hidden: false,
		})
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
	components := make([]NormalizedPresentationComponent, 0, len(componentSet))
	for _, component := range sortedStringSet(componentSet) {
		components = append(components, NormalizedPresentationComponent{ID: component})
	}
	manifest := &NormalizedPresentationManifest{
		ModuleName: "foundation-core", PageKey: binding.PageKey,
		DefinitionVersion: PresentationDefinitionVersion,
		Components:        components, Fields: fields,
		DataSources: []NormalizedPresentationDataSource{{
			ID:                  binding.DataSourceID,
			RequiredPermissions: append([]string(nil), binding.RequiredPermissions...),
			PageSizeOptions:     append([]int(nil), binding.PageSizeOptions...),
			MaxPageSize:         binding.MaxPageSize, MaxSortFields: binding.MaxSortFields,
		}},
		Actions: make([]NormalizedPresentationAction, 0),
		DefaultPresentation: NormalizedCompletePresentation{
			Title: d.Spec.Title, DataSource: binding.DataSourceID,
			List: NormalizedCompleteListPresentation{
				Columns: list, Density: d.Spec.List.Density, PageSize: d.Spec.List.PageSize,
				DefaultSort: make([]NormalizedPresentationSort, 0),
			},
			Search:  NormalizedCompleteSearchPresentation{Fields: search, CollapsedByDefault: false},
			Form:    NormalizedCompleteFormPresentation{Fields: make([]NormalizedCompleteField, 0), Columns: 1},
			Detail:  NormalizedCompleteDetailPresentation{Fields: make([]NormalizedCompleteField, 0), Columns: 1},
			Actions: make([]NormalizedCompleteAction, 0),
		},
	}
	if issues := ValidateNormalizedPresentationManifest(manifest); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	canonical, err := manifest.CanonicalJSON()
	if err != nil {
		return nil, fmt.Errorf("canonicalize core page presentation %s: %w", manifest.PageKey, err)
	}
	digest := sha256.Sum256(canonical)
	manifest.DefinitionHash = "sha256:" + hex.EncodeToString(digest[:])
	return manifest, nil
}
