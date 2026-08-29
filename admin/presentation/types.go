package presentation

import "encoding/json"

const (
	APIVersion = "mss.io/v1alpha1"
	Kind       = "AdminPagePresentation"

	// DefinitionVersionV1 is retained for already published P0/P1 capability
	// identities. Version 1 deliberately excludes complete defaults from its
	// hash and must never be silently reinterpreted.
	DefinitionVersionV1 = "1"
	// DefinitionVersionV2 is the first generated production-capability
	// contract. Its hash covers the complete normalized definition.
	DefinitionVersionV2 = "2"
	// DefinitionVersion preserves the historical test/API default. New
	// generator projections must use DefinitionVersionV2 explicitly.
	DefinitionVersion = DefinitionVersionV1
	MaxDocumentBytes  = 128 << 10
)

type ScopeKind string

const (
	ScopeApplication ScopeKind = "application"
	ScopeRole        ScopeKind = "role"
	ScopeUser        ScopeKind = "user"
)

type Surface string

const (
	SurfaceList   Surface = "list"
	SurfaceSearch Surface = "search"
	SurfaceForm   Surface = "form"
	SurfaceDetail Surface = "detail"
)

type ActionPlacement string

const (
	PlacementToolbar ActionPlacement = "toolbar"
	PlacementRow     ActionPlacement = "row"
	PlacementForm    ActionPlacement = "form"
	PlacementDetail  ActionPlacement = "detail"
)

type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type LocalizedText struct {
	ZhCN *string `json:"zh-CN,omitempty"`
	EnUS *string `json:"en-US,omitempty"`
}

type Scope struct {
	Kind    ScopeKind `json:"kind"`
	Subject *string   `json:"subject,omitempty"`
}

type Metadata struct {
	Name           string  `json:"name"`
	PageKey        string  `json:"pageKey"`
	Description    *string `json:"description,omitempty"`
	DefinitionHash string  `json:"definitionHash"`
	Scope          Scope   `json:"scope"`
}

type Profile struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   Metadata    `json:"metadata"`
	Spec       ProfileSpec `json:"spec"`
}

type ProfileSpec struct {
	Title      *LocalizedText `json:"title,omitempty"`
	DataSource *string        `json:"dataSource,omitempty"`
	List       *ListPatch     `json:"list,omitempty"`
	Search     *SearchPatch   `json:"search,omitempty"`
	Form       *FormPatch     `json:"form,omitempty"`
	Detail     *DetailPatch   `json:"detail,omitempty"`
	Actions    *[]ActionPatch `json:"actions,omitempty"`
}

type ListPatch struct {
	Columns     *[]FieldPatch `json:"columns,omitempty"`
	Density     *string       `json:"density,omitempty"`
	PageSize    *int          `json:"pageSize,omitempty"`
	DefaultSort *[]Sort       `json:"defaultSort,omitempty"`
}

type SearchPatch struct {
	Fields             *[]FieldPatch `json:"fields,omitempty"`
	CollapsedByDefault *bool         `json:"collapsedByDefault,omitempty"`
}

type FormPatch struct {
	Fields  *[]FieldPatch `json:"fields,omitempty"`
	Columns *int          `json:"columns,omitempty"`
}

type DetailPatch struct {
	Fields  *[]FieldPatch `json:"fields,omitempty"`
	Columns *int          `json:"columns,omitempty"`
}

type FieldPatch struct {
	Field       string         `json:"field"`
	Label       *LocalizedText `json:"label,omitempty"`
	Component   *string        `json:"component,omitempty"`
	Order       *int           `json:"order,omitempty"`
	Hidden      *bool          `json:"hidden,omitempty"`
	Width       *int           `json:"width,omitempty"`
	Span        *int           `json:"span,omitempty"`
	Placeholder *LocalizedText `json:"placeholder,omitempty"`
	Help        *LocalizedText `json:"help,omitempty"`
	VisibleWhen *Condition     `json:"visibleWhen,omitempty"`
}

type ActionPatch struct {
	Action      string           `json:"action"`
	Label       *LocalizedText   `json:"label,omitempty"`
	Placement   *ActionPlacement `json:"placement,omitempty"`
	Order       *int             `json:"order,omitempty"`
	Hidden      *bool            `json:"hidden,omitempty"`
	Confirm     *LocalizedText   `json:"confirm,omitempty"`
	VisibleWhen *Condition       `json:"visibleWhen,omitempty"`
}

type Sort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// Condition uses pointers for every variant so strict validation can
// distinguish omission from explicit empty arrays and false-y scalar values.
type Condition struct {
	Field    *string          `json:"field,omitempty"`
	Operator *string          `json:"operator,omitempty"`
	Value    *json.RawMessage `json:"value,omitempty"`
	All      *[]Condition     `json:"all,omitempty"`
	Any      *[]Condition     `json:"any,omitempty"`
	Not      *Condition       `json:"not,omitempty"`
}

type CapabilityDefinition struct {
	PageKey             string                 `json:"pageKey"`
	DefinitionVersion   string                 `json:"definitionVersion"`
	DefinitionHash      string                 `json:"definitionHash"`
	Components          []CapabilityComponent  `json:"components"`
	Fields              []CapabilityField      `json:"fields"`
	DataSources         []CapabilityDataSource `json:"dataSources"`
	Actions             []CapabilityAction     `json:"actions"`
	DefaultPresentation CompletePresentation   `json:"defaultPresentation"`
}

type CapabilityComponent struct {
	ID string `json:"id"`
}

type CapabilityField struct {
	ID                string                        `json:"id"`
	Label             LocalizedText                 `json:"label"`
	ValueType         string                        `json:"valueType"`
	Format            string                        `json:"format"`
	Required          bool                          `json:"required"`
	Nullable          bool                          `json:"nullable"`
	ReadOnly          bool                          `json:"readOnly"`
	Searchable        bool                          `json:"searchable"`
	Sortable          bool                          `json:"sortable"`
	Filterable        bool                          `json:"filterable"`
	Surfaces          []Surface                     `json:"surfaces"`
	Components        []string                      `json:"components"`
	SurfaceComponents []CapabilitySurfaceComponents `json:"surfaceComponents"`
	EnumValues        []CapabilityEnumValue         `json:"enumValues"`
	Validation        CapabilityFieldValidation     `json:"validation"`
}

type CapabilityFieldValidation struct {
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Minimum   *string `json:"minimum,omitempty"`
	Maximum   *string `json:"maximum,omitempty"`
	Pattern   string  `json:"pattern,omitempty"`
	Precision *int    `json:"precision,omitempty"`
	Scale     *int    `json:"scale,omitempty"`
}

type CapabilitySurfaceComponents struct {
	Surface    Surface  `json:"surface"`
	Components []string `json:"components"`
}

type CapabilityEnumValue struct {
	Value string        `json:"value"`
	Label LocalizedText `json:"label"`
	Color string        `json:"color"`
}

type CapabilityDataSource struct {
	ID                  string   `json:"id"`
	RequiredPermissions []string `json:"requiredPermissions"`
	PageSizeOptions     []int    `json:"pageSizeOptions,omitempty"`
	MaxPageSize         int      `json:"maxPageSize,omitempty"`
	MaxSortFields       int      `json:"maxSortFields,omitempty"`
}

type CapabilityAction struct {
	ID                  string            `json:"id"`
	RequiredPermissions []string          `json:"requiredPermissions"`
	Placements          []ActionPlacement `json:"placements"`
	Destructive         bool              `json:"destructive"`
}

type CompletePresentation struct {
	Title      LocalizedText              `json:"title"`
	DataSource string                     `json:"dataSource"`
	List       CompleteListPresentation   `json:"list"`
	Search     CompleteSearchPresentation `json:"search"`
	Form       CompleteFormPresentation   `json:"form"`
	Detail     CompleteDetailPresentation `json:"detail"`
	Actions    []CompleteAction           `json:"actions"`
}

type CompleteListPresentation struct {
	Columns     []CompleteField `json:"columns"`
	Density     string          `json:"density"`
	PageSize    int             `json:"pageSize"`
	DefaultSort []Sort          `json:"defaultSort"`
}

type CompleteSearchPresentation struct {
	Fields             []CompleteField `json:"fields"`
	CollapsedByDefault bool            `json:"collapsedByDefault"`
}

type CompleteFormPresentation struct {
	Fields  []CompleteField `json:"fields"`
	Columns int             `json:"columns"`
}

type CompleteDetailPresentation struct {
	Fields  []CompleteField `json:"fields"`
	Columns int             `json:"columns"`
}

type CompleteField struct {
	Field       string         `json:"field"`
	Label       *LocalizedText `json:"label,omitempty"`
	Component   string         `json:"component"`
	Order       int            `json:"order"`
	Hidden      bool           `json:"hidden"`
	Width       *int           `json:"width,omitempty"`
	Span        *int           `json:"span,omitempty"`
	Placeholder *LocalizedText `json:"placeholder,omitempty"`
	Help        *LocalizedText `json:"help,omitempty"`
	VisibleWhen *Condition     `json:"visibleWhen,omitempty"`
}

type CompleteAction struct {
	Action      string          `json:"action"`
	Label       *LocalizedText  `json:"label,omitempty"`
	Placement   ActionPlacement `json:"placement"`
	Order       int             `json:"order"`
	Hidden      bool            `json:"hidden"`
	Confirm     *LocalizedText  `json:"confirm,omitempty"`
	VisibleWhen *Condition      `json:"visibleWhen,omitempty"`
}

type Document struct {
	Profile   *Profile `json:"-"`
	Canonical []byte   `json:"-"`
	Digest    string   `json:"digest"`
}
