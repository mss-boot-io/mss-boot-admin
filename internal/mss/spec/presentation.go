package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var presentationPageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$`)

var protectedPresentationPageNamespaces = map[string]bool{
	"account": true, "auth": true, "authentication": true,
	"authorization": true, "app-config": true, "application-config": true,
	"config": true, "configuration": true, "login": true,
	"presentation": true, "presentation-config": true, "recovery": true,
	"release": true, "system": true,
}

// PresentationSource is the strict, data-only AdminModule source for one
// generated page capability. It deliberately contains no routes, methods,
// permission strings, imports, handlers, transport bindings, or executable
// expressions; those facts remain derived from trusted module and catalog
// contracts.
type PresentationSource struct {
	PageKey           string                     `yaml:"pageKey" json:"pageKey"`
	DefinitionVersion string                     `yaml:"definitionVersion" json:"definitionVersion"`
	Title             PresentationLocalizedText  `yaml:"title" json:"title"`
	DataSource        string                     `yaml:"dataSource" json:"dataSource"`
	List              PresentationListSource     `yaml:"list" json:"list"`
	Search            PresentationSearchSource   `yaml:"search" json:"search"`
	Form              PresentationFormSource     `yaml:"form" json:"form"`
	Detail            PresentationDetailSource   `yaml:"detail" json:"detail"`
	Actions           []PresentationActionSource `yaml:"actions" json:"actions"`
}

type PresentationLocalizedText struct {
	ZhCN            string `yaml:"zh-CN,omitempty" json:"zh-CN"`
	EnUS            string `yaml:"en-US,omitempty" json:"en-US"`
	presenceTracked bool
	zhCNPresent     bool
	enUSPresent     bool
}

type PresentationListSource struct {
	Density     string                    `yaml:"density" json:"density"`
	PageSize    int                       `yaml:"pageSize" json:"pageSize"`
	DefaultSort []PresentationSortSource  `yaml:"defaultSort" json:"defaultSort"`
	Fields      []PresentationFieldSource `yaml:"fields" json:"fields"`
}

type PresentationSearchSource struct {
	CollapsedByDefault bool                      `yaml:"collapsedByDefault" json:"collapsedByDefault"`
	Fields             []PresentationFieldSource `yaml:"fields" json:"fields"`
	collapsedOmitted   bool
}

type PresentationFormSource struct {
	Columns int                       `yaml:"columns" json:"columns"`
	Fields  []PresentationFieldSource `yaml:"fields" json:"fields"`
}

type PresentationDetailSource struct {
	Columns int                       `yaml:"columns" json:"columns"`
	Fields  []PresentationFieldSource `yaml:"fields" json:"fields"`
}

type PresentationFieldSource struct {
	Field             string                     `yaml:"field" json:"field"`
	Label             *PresentationLocalizedText `yaml:"label,omitempty" json:"label,omitempty"`
	Component         string                     `yaml:"component" json:"component"`
	AllowedComponents []string                   `yaml:"allowedComponents,omitempty" json:"allowedComponents,omitempty"`
	Order             int                        `yaml:"order" json:"order"`
	Hidden            bool                       `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Width             *int                       `yaml:"width,omitempty" json:"width,omitempty"`
	Span              *int                       `yaml:"span,omitempty" json:"span,omitempty"`
	Placeholder       *PresentationLocalizedText `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Help              *PresentationLocalizedText `yaml:"help,omitempty" json:"help,omitempty"`
	orderOmitted      bool
}

type PresentationSortSource struct {
	Field     string `yaml:"field" json:"field"`
	Direction string `yaml:"direction" json:"direction"`
}

type PresentationActionSource struct {
	Action       string                     `yaml:"action" json:"action"`
	Label        *PresentationLocalizedText `yaml:"label,omitempty" json:"label,omitempty"`
	Placement    string                     `yaml:"placement" json:"placement"`
	Order        int                        `yaml:"order" json:"order"`
	Hidden       bool                       `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Confirm      *PresentationLocalizedText `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	orderOmitted bool
}

// NormalizedPresentationManifest is the sole semantic P2A object used to
// produce backend, frontend, adapter, snapshot, and registry projections.
// ModuleName is generation provenance and is intentionally excluded from the
// canonical definition hash.
type NormalizedPresentationManifest struct {
	ModuleName          string                             `json:"moduleName"`
	PageKey             string                             `json:"pageKey"`
	DefinitionVersion   string                             `json:"definitionVersion"`
	DefinitionHash      string                             `json:"definitionHash"`
	Components          []NormalizedPresentationComponent  `json:"components"`
	Fields              []NormalizedPresentationField      `json:"fields"`
	DataSources         []NormalizedPresentationDataSource `json:"dataSources"`
	Actions             []NormalizedPresentationAction     `json:"actions"`
	DefaultPresentation NormalizedCompletePresentation     `json:"defaultPresentation"`
}

type NormalizedPresentationComponent struct {
	ID string `json:"id"`
}

type NormalizedPresentationField struct {
	ID                string                                    `json:"id"`
	Label             PresentationLocalizedText                 `json:"label"`
	ValueType         string                                    `json:"valueType"`
	Format            string                                    `json:"format"`
	Required          bool                                      `json:"required"`
	Nullable          bool                                      `json:"nullable"`
	ReadOnly          bool                                      `json:"readOnly"`
	Searchable        bool                                      `json:"searchable"`
	Sortable          bool                                      `json:"sortable"`
	Filterable        bool                                      `json:"filterable"`
	Surfaces          []string                                  `json:"surfaces"`
	Components        []string                                  `json:"components"`
	SurfaceComponents []NormalizedPresentationSurfaceComponents `json:"surfaceComponents"`
	EnumValues        []NormalizedPresentationEnumValue         `json:"enumValues"`
	Validation        NormalizedPresentationValidation          `json:"validation"`
}

type NormalizedPresentationSurfaceComponents struct {
	Surface    string   `json:"surface"`
	Components []string `json:"components"`
}

type NormalizedPresentationEnumValue struct {
	Value string                    `json:"value"`
	Label PresentationLocalizedText `json:"label"`
	Color string                    `json:"color"`
}

// Decimal bounds are strings so the v2 canonical JSON contains only bounded
// integer JSON numbers while still preserving exact validation semantics.
type NormalizedPresentationValidation struct {
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Minimum   *string `json:"minimum,omitempty"`
	Maximum   *string `json:"maximum,omitempty"`
	Pattern   string  `json:"pattern,omitempty"`
	Precision *int    `json:"precision,omitempty"`
	Scale     *int    `json:"scale,omitempty"`
}

type NormalizedPresentationDataSource struct {
	ID                  string   `json:"id"`
	RequiredPermissions []string `json:"requiredPermissions"`
	PageSizeOptions     []int    `json:"pageSizeOptions"`
	MaxPageSize         int      `json:"maxPageSize"`
	MaxSortFields       int      `json:"maxSortFields"`
}

type NormalizedPresentationAction struct {
	ID                  string   `json:"id"`
	RequiredPermissions []string `json:"requiredPermissions"`
	Placements          []string `json:"placements"`
	Destructive         bool     `json:"destructive"`
}

type NormalizedCompletePresentation struct {
	Title      PresentationLocalizedText            `json:"title"`
	DataSource string                               `json:"dataSource"`
	List       NormalizedCompleteListPresentation   `json:"list"`
	Search     NormalizedCompleteSearchPresentation `json:"search"`
	Form       NormalizedCompleteFormPresentation   `json:"form"`
	Detail     NormalizedCompleteDetailPresentation `json:"detail"`
	Actions    []NormalizedCompleteAction           `json:"actions"`
}

type NormalizedCompleteListPresentation struct {
	Columns     []NormalizedCompleteField    `json:"columns"`
	Density     string                       `json:"density"`
	PageSize    int                          `json:"pageSize"`
	DefaultSort []NormalizedPresentationSort `json:"defaultSort"`
}

type NormalizedCompleteSearchPresentation struct {
	Fields             []NormalizedCompleteField `json:"fields"`
	CollapsedByDefault bool                      `json:"collapsedByDefault"`
}

type NormalizedCompleteFormPresentation struct {
	Fields  []NormalizedCompleteField `json:"fields"`
	Columns int                       `json:"columns"`
}

type NormalizedCompleteDetailPresentation struct {
	Fields  []NormalizedCompleteField `json:"fields"`
	Columns int                       `json:"columns"`
}

type NormalizedCompleteField struct {
	Field       string                     `json:"field"`
	Label       PresentationLocalizedText  `json:"label"`
	Component   string                     `json:"component"`
	Order       int                        `json:"order"`
	Hidden      bool                       `json:"hidden"`
	Width       *int                       `json:"width,omitempty"`
	Span        *int                       `json:"span,omitempty"`
	Placeholder *PresentationLocalizedText `json:"placeholder,omitempty"`
	Help        *PresentationLocalizedText `json:"help,omitempty"`
}

type NormalizedPresentationSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type NormalizedCompleteAction struct {
	Action    string                     `json:"action"`
	Label     PresentationLocalizedText  `json:"label"`
	Placement string                     `json:"placement"`
	Order     int                        `json:"order"`
	Hidden    bool                       `json:"hidden"`
	Confirm   *PresentationLocalizedText `json:"confirm,omitempty"`
}

func (p *PresentationSource) normalize() {
	p.PageKey = strings.TrimSpace(p.PageKey)
	p.DefinitionVersion = strings.TrimSpace(p.DefinitionVersion)
	p.Title.normalize()
	p.DataSource = strings.TrimSpace(p.DataSource)
	p.List.Density = strings.TrimSpace(p.List.Density)
	for index := range p.List.DefaultSort {
		p.List.DefaultSort[index].Field = strings.TrimSpace(p.List.DefaultSort[index].Field)
		p.List.DefaultSort[index].Direction = strings.TrimSpace(p.List.DefaultSort[index].Direction)
	}
	for _, fields := range [][]PresentationFieldSource{p.List.Fields, p.Search.Fields, p.Form.Fields, p.Detail.Fields} {
		for index := range fields {
			fields[index].normalize()
		}
	}
	for index := range p.Actions {
		p.Actions[index].Action = strings.TrimSpace(p.Actions[index].Action)
		p.Actions[index].Placement = strings.TrimSpace(p.Actions[index].Placement)
		if p.Actions[index].Label != nil {
			p.Actions[index].Label.normalize()
		}
		if p.Actions[index].Confirm != nil {
			p.Actions[index].Confirm.normalize()
		}
	}
}

func (p *PresentationFieldSource) normalize() {
	p.Field = strings.TrimSpace(p.Field)
	p.Component = strings.TrimSpace(p.Component)
	trimAndSortStrings(p.AllowedComponents)
	if p.Label != nil {
		p.Label.normalize()
	}
	if p.Placeholder != nil {
		p.Placeholder.normalize()
	}
	if p.Help != nil {
		p.Help.normalize()
	}
}

func (p *PresentationLocalizedText) normalize() {
	p.ZhCN = strings.TrimSpace(p.ZhCN)
	p.EnUS = strings.TrimSpace(p.EnUS)
}

// NormalizePresentation validates and materializes the complete version 2
// manifest using the immutable embedded Foundation catalog. Omission is a
// first-class backward-compatible state and returns (nil, nil).
func (m *Module) NormalizePresentation() (*NormalizedPresentationManifest, error) {
	catalog, err := DefaultPresentationCatalog()
	if err != nil {
		return nil, err
	}
	return m.NormalizePresentationWithCatalog(catalog)
}

// NormalizePresentationWithCatalog is the explicit test and tooling seam for
// catalog compatibility. Production generation should use
// NormalizePresentation so the released embedded catalog remains authoritative.
func (m *Module) NormalizePresentationWithCatalog(catalog *PresentationCatalog) (*NormalizedPresentationManifest, error) {
	if m == nil || m.Spec.Presentation == nil {
		return nil, nil
	}
	m.Spec.Presentation.normalize()
	if catalog == nil {
		return nil, &ValidationError{Issues: []Issue{{
			Path: "spec.presentation", Code: "presentation-catalog-unavailable", Message: "presentation catalog is required",
		}}}
	}
	manifest, issues := m.buildNormalizedPresentation(catalog)
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	canonical, err := manifest.CanonicalJSON()
	if err != nil {
		return nil, fmt.Errorf("canonicalize Admin presentation %s: %w", manifest.PageKey, err)
	}
	digest := sha256.Sum256(canonical)
	manifest.DefinitionHash = "sha256:" + hex.EncodeToString(digest[:])
	return manifest, nil
}

// CanonicalJSON returns the complete hash input for a version 2 manifest. It
// deliberately excludes only generation provenance (moduleName) and the hash
// itself, preserves normalized array order, emits UTF-8 without HTML-only
// escaping, and uses struct declaration order for fixed ASCII property names.
func (m *NormalizedPresentationManifest) CanonicalJSON() ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	canonical := struct {
		PageKey             string                             `json:"pageKey"`
		DefinitionVersion   string                             `json:"definitionVersion"`
		Components          []NormalizedPresentationComponent  `json:"components"`
		Fields              []NormalizedPresentationField      `json:"fields"`
		DataSources         []NormalizedPresentationDataSource `json:"dataSources"`
		Actions             []NormalizedPresentationAction     `json:"actions"`
		DefaultPresentation NormalizedCompletePresentation     `json:"defaultPresentation"`
	}{
		PageKey: m.PageKey, DefinitionVersion: m.DefinitionVersion,
		Components: m.Components, Fields: m.Fields, DataSources: m.DataSources,
		Actions: m.Actions, DefaultPresentation: m.DefaultPresentation,
	}
	return CanonicalPresentationJSON(canonical)
}

// CanonicalPresentationJSON encodes a presentation projection using the
// JSON.stringify-compatible canonical form shared by normalization and code
// generation. Object properties are sorted, arrays preserve their input order,
// HTML-only characters and U+2028/U+2029 remain UTF-8, and literal backslash-u
// sequences remain data rather than being interpreted as escapes.
func CanonicalPresentationJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var ordered any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&ordered); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := writePresentationCanonicalJSON(&output, ordered); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writePresentationCanonicalJSON(output *bytes.Buffer, value any) error {
	switch current := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(current))
	case string:
		writePresentationJSONString(output, current)
	case json.Number:
		if _, err := strconv.ParseFloat(current.String(), 64); err != nil {
			return fmt.Errorf("invalid JSON number %q", current)
		}
		output.WriteString(current.String())
	case []any:
		output.WriteByte('[')
		for index := range current {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writePresentationCanonicalJSON(output, current[index]); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			writePresentationJSONString(output, key)
			output.WriteByte(':')
			if err := writePresentationCanonicalJSON(output, current[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value %T", value)
	}
	return nil
}

// writePresentationJSONString deliberately follows JSON.stringify string
// escaping. In particular U+2028 and U+2029 remain UTF-8 while a literal
// backslash-u sequence remains escaped as data.
func writePresentationJSONString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, current := range value {
		switch current {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if current < 0x20 {
				fmt.Fprintf(output, `\u%04x`, current)
			} else {
				output.WriteRune(current)
			}
		}
	}
	output.WriteByte('"')
}

func (m *Module) buildNormalizedPresentation(catalog *PresentationCatalog) (*NormalizedPresentationManifest, []Issue) {
	source := m.Spec.Presentation
	if source == nil {
		return nil, nil
	}
	issues := make([]Issue, 0)
	add := func(path, code, message string) {
		issues = append(issues, Issue{Path: path, Code: code, Message: message})
	}
	if catalog == nil {
		add("spec.presentation", "presentation-catalog-unavailable", "presentation catalog is required")
		return nil, issues
	}
	for _, catalogIssue := range catalog.Validate() {
		add("presentationCatalog."+catalogIssue.Path, catalogIssue.Code, catalogIssue.Message)
	}
	if source.DefinitionVersion != PresentationDefinitionVersion {
		add("spec.presentation.definitionVersion", "unsupported-definition-version", "generated presentation definitions must use version "+PresentationDefinitionVersion)
	}
	if source.DefinitionVersion != catalog.Metadata.DefinitionVersion {
		add("spec.presentation.definitionVersion", "catalog-version-mismatch", "definition version must match the embedded catalog")
	}
	if !presentationPageKeyPattern.MatchString(source.PageKey) {
		add("spec.presentation.pageKey", "invalid-page-key", "must contain at least one dot and use lower-case stable segments")
	} else if len(source.PageKey) > 120 {
		add("spec.presentation.pageKey", "page-key-too-long", "page key must not exceed 120 bytes")
	} else if IsProtectedPresentationPageKey(source.PageKey) {
		add("spec.presentation.pageKey", "protected-page-key", "protected core page namespaces cannot be presentation-configurable")
	}
	validateLocalizedText("spec.presentation.title", source.Title, true, add)
	validatePresentationEntityFacts(m.Spec.Entity.Fields, add)

	fields, expectedBySurface := m.normalizedPresentationFields()
	fieldByID := make(map[string]*NormalizedPresentationField, len(fields))
	for index := range fields {
		fieldByID[fields[index].ID] = &fields[index]
	}

	listFields := validatePresentationSurfaceFields("list", source.List.Fields, expectedBySurface["list"], fieldByID, catalog, add)
	searchFields := validatePresentationSurfaceFields("search", source.Search.Fields, expectedBySurface["search"], fieldByID, catalog, add)
	formFields := validatePresentationSurfaceFields("form", source.Form.Fields, expectedBySurface["form"], fieldByID, catalog, add)
	detailFields := validatePresentationSurfaceFields("detail", source.Detail.Fields, expectedBySurface["detail"], fieldByID, catalog, add)

	if !contains([]string{"compact", "middle", "large"}, source.List.Density) {
		add("spec.presentation.list.density", "unsupported-density", "supported values are compact, middle, large")
	}
	if source.List.DefaultSort == nil {
		add("spec.presentation.list.defaultSort", "required", "complete list defaults must explicitly declare defaultSort, including an empty array")
	}
	if source.Actions == nil {
		add("spec.presentation.actions", "required", "complete presentation defaults must explicitly declare actions, including an empty array")
	}
	if source.Search.collapsedOmitted {
		add("spec.presentation.search.collapsedByDefault", "required", "collapsedByDefault must be explicitly declared, including false")
	}
	if source.Form.Columns < 1 || source.Form.Columns > 4 {
		add("spec.presentation.form.columns", "columns-out-of-range", "columns must be between 1 and 4")
	}
	if source.Detail.Columns < 1 || source.Detail.Columns > 4 {
		add("spec.presentation.detail.columns", "columns-out-of-range", "columns must be between 1 and 4")
	}

	dataSources := make([]NormalizedPresentationDataSource, 0, 1)
	qualifiedDataSource := ""
	dataSourceCatalog, dataSourceExists := validatePresentationDataSource(m, source.DataSource, catalog, add)
	if dataSourceExists {
		qualifiedDataSource = m.Metadata.Name + "." + dataSourceCatalog.ID
		permissions := []string{m.Spec.Menu.Path, presentationPermissionPath(m.Spec.Menu.Path, dataSourceCatalog.PermissionAction)}
		sort.Strings(permissions)
		dataSources = append(dataSources, NormalizedPresentationDataSource{
			ID: qualifiedDataSource, RequiredPermissions: permissions,
			PageSizeOptions: append([]int(nil), dataSourceCatalog.PageSizeOptions...),
			MaxPageSize:     dataSourceCatalog.MaxPageSize, MaxSortFields: dataSourceCatalog.MaxSortFields,
		})
		if !containsInt(dataSourceCatalog.PageSizeOptions, source.List.PageSize) {
			add("spec.presentation.list.pageSize", "unsupported-page-size", "page size must be one of the compiled data-source options")
		}
		if source.List.PageSize > dataSourceCatalog.MaxPageSize {
			add("spec.presentation.list.pageSize", "page-size-exceeds-maximum", "page size exceeds the compiled data-source maximum")
		}
	}
	defaultSort := validatePresentationSort(source.List.DefaultSort, fieldByID, dataSourceCatalog, dataSourceExists, add)
	actions, completeActions := validatePresentationActions(m, source.Actions, catalog, add)

	componentSet := map[string]bool{}
	for index := range fields {
		field := &fields[index]
		trimAndSortByOrder(field.Surfaces, presentationSurfaceOrder)
		sort.SliceStable(field.SurfaceComponents, func(i, j int) bool {
			left := presentationSurfaceOrder[field.SurfaceComponents[i].Surface]
			right := presentationSurfaceOrder[field.SurfaceComponents[j].Surface]
			return left < right
		})
		componentSetForField := map[string]bool{}
		for surfaceIndex := range field.SurfaceComponents {
			trimAndSortStrings(field.SurfaceComponents[surfaceIndex].Components)
			for _, component := range field.SurfaceComponents[surfaceIndex].Components {
				componentSetForField[component] = true
				componentSet[component] = true
			}
		}
		field.Components = sortedStringSet(componentSetForField)
		if field.EnumValues == nil {
			field.EnumValues = make([]NormalizedPresentationEnumValue, 0)
		}
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
	components := make([]NormalizedPresentationComponent, 0, len(componentSet))
	for _, id := range sortedStringSet(componentSet) {
		components = append(components, NormalizedPresentationComponent{ID: id})
	}

	manifest := &NormalizedPresentationManifest{
		ModuleName: m.Metadata.Name, PageKey: source.PageKey,
		DefinitionVersion: source.DefinitionVersion, DefinitionHash: "",
		Components: components, Fields: fields, DataSources: dataSources, Actions: actions,
		DefaultPresentation: NormalizedCompletePresentation{
			Title: source.Title, DataSource: qualifiedDataSource,
			List: NormalizedCompleteListPresentation{
				Columns: listFields, Density: source.List.Density,
				PageSize: source.List.PageSize, DefaultSort: defaultSort,
			},
			Search: NormalizedCompleteSearchPresentation{
				Fields: searchFields, CollapsedByDefault: source.Search.CollapsedByDefault,
			},
			Form:    NormalizedCompleteFormPresentation{Fields: formFields, Columns: source.Form.Columns},
			Detail:  NormalizedCompleteDetailPresentation{Fields: detailFields, Columns: source.Detail.Columns},
			Actions: completeActions,
		},
	}
	if manifest.Components == nil {
		manifest.Components = make([]NormalizedPresentationComponent, 0)
	}
	if manifest.DataSources == nil {
		manifest.DataSources = make([]NormalizedPresentationDataSource, 0)
	}
	if manifest.Actions == nil {
		manifest.Actions = make([]NormalizedPresentationAction, 0)
	}
	issues = append(issues, ValidateNormalizedPresentationManifest(manifest)...)
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
	return manifest, issues
}

func validatePresentationEntityFacts(fields []FieldSpec, add func(string, string, string)) {
	const maxSafeJSONInteger = int64(1<<53 - 1)
	for index, field := range fields {
		if field.UI.Hidden {
			continue
		}
		path := "spec.entity.fields[" + strconv.Itoa(index) + "].validation"
		for name, value := range map[string]*int{
			"minLength": field.Validation.MinLength,
			"maxLength": field.Validation.MaxLength,
		} {
			if value != nil && (*value < 0 || int64(*value) > maxSafeJSONInteger) {
				add(path+"."+name, "presentation-unsafe-integer", name+" must be a non-negative JSON safe integer when presentation generation is enabled")
			}
		}
		if field.Validation.MinLength != nil && field.Validation.MaxLength != nil && *field.Validation.MinLength > *field.Validation.MaxLength {
			add(path, "presentation-invalid-length-range", "minLength cannot exceed maxLength")
		}
		if field.Validation.Precision != nil && (*field.Validation.Precision < 1 || *field.Validation.Precision > 38) {
			add(path+".precision", "presentation-precision-out-of-range", "precision must be between 1 and 38 when presentation generation is enabled")
		}
		if field.Validation.Scale != nil && (*field.Validation.Scale < 0 || *field.Validation.Scale > 38) {
			add(path+".scale", "presentation-scale-out-of-range", "scale must be between 0 and 38 when presentation generation is enabled")
		}
		if field.Validation.Precision != nil && field.Validation.Scale != nil && *field.Validation.Scale > *field.Validation.Precision {
			add(path, "presentation-invalid-decimal-shape", "scale cannot exceed precision")
		}
		if field.Validation.Pattern != "" && !isPortablePresentationPattern(field.Validation.Pattern) {
			add(path+".pattern", "presentation-non-portable-pattern", "pattern must use the portable Go and ECMAScript regular-expression subset")
		}
		if field.Validation.Minimum != nil && (math.IsNaN(*field.Validation.Minimum) || math.IsInf(*field.Validation.Minimum, 0)) {
			add(path+".minimum", "presentation-non-finite-bound", "minimum must be finite when presentation generation is enabled")
		}
		if field.Validation.Maximum != nil && (math.IsNaN(*field.Validation.Maximum) || math.IsInf(*field.Validation.Maximum, 0)) {
			add(path+".maximum", "presentation-non-finite-bound", "maximum must be finite when presentation generation is enabled")
		}
	}
}

func isPortablePresentationPattern(value string) bool {
	if strings.Contains(value, "(?") || strings.Contains(value, "[[:") {
		return false
	}
	for _, current := range value {
		if current > 0xffff || current == '\u2028' || current == '\u2029' {
			return false
		}
	}
	inClass := false
	classHasContent := false
	classAtStart := false
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '\\':
			index++
			if index >= len(value) {
				return false
			}
			escaped := value[index]
			if escaped == 'x' {
				if index+2 >= len(value) ||
					!isPresentationPatternHex(value[index+1]) ||
					!isPresentationPatternHex(value[index+2]) {
					return false
				}
				index += 2
			} else {
				allowed := `bBdDfnrtvwW$()*+./?[\]^{|}`
				if inClass {
					allowed = `dDfnrtvwW$()*+-./?[\]^{|}`
				}
				if !strings.ContainsRune(allowed, rune(escaped)) {
					return false
				}
			}
			if inClass {
				classHasContent = true
				classAtStart = false
			}
		case '[':
			if inClass {
				classHasContent = true
				classAtStart = false
				continue
			}
			inClass = true
			classHasContent = false
			classAtStart = true
		case ']':
			if !inClass || !classHasContent {
				return false
			}
			inClass = false
			classHasContent = false
			classAtStart = false
		case '^':
			// A leading caret negates the class and does not count as class
			// content. Rejecting a following ']' closes the Go/JS semantic
			// gap around empty classes and classes whose first literal is ']'.
			if inClass {
				if classAtStart {
					classAtStart = false
				} else {
					classHasContent = true
				}
			}
		case '{', '}':
			if !inClass {
				return false
			}
			classHasContent = true
			classAtStart = false
		case '.':
			if !inClass {
				return false
			}
			classHasContent = true
			classAtStart = false
		default:
			if inClass {
				classHasContent = true
				classAtStart = false
			}
		}
	}
	return !inClass
}

func isPresentationPatternHex(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

func (m *Module) normalizedPresentationFields() ([]NormalizedPresentationField, map[string][]string) {
	fields := make([]NormalizedPresentationField, 0, len(m.Spec.Entity.Fields)+3)
	expected := map[string][]string{"list": {}, "search": {}, "form": {}, "detail": {}}
	selectedSearchFields := map[string]bool{}
	if m.Spec.Presentation != nil {
		for _, sourceField := range m.Spec.Presentation.Search.Fields {
			selectedSearchFields[sourceField.Field] = true
		}
	}
	for _, sourceField := range m.Spec.Entity.Fields {
		if sourceField.UI.Hidden {
			continue
		}
		field := normalizedEntityPresentationField(sourceField)
		if sourceField.List == nil || *sourceField.List {
			field.Surfaces = append(field.Surfaces, "list")
			expected["list"] = append(expected["list"], sourceField.Name)
		}
		// Searchable and filterable are eligibility facts, not independent
		// transport bindings. The presentation source selects only controls the
		// compiled query adapter actually exposes (for example Supplier's one
		// keyword field may bind several searchable backend columns to q).
		if sourceField.Form == nil || *sourceField.Form {
			field.Surfaces = append(field.Surfaces, "form")
			expected["form"] = append(expected["form"], sourceField.Name)
		}
		if sourceField.Detail == nil || *sourceField.Detail {
			field.Surfaces = append(field.Surfaces, "detail")
			expected["detail"] = append(expected["detail"], sourceField.Name)
		}
		if len(field.Surfaces) > 0 || ((sourceField.Searchable || sourceField.Filterable) && selectedSearchFields[sourceField.Name]) {
			fields = append(fields, field)
		}
	}
	idValueType := "string"
	if m.Spec.Entity.IDType == "int64" {
		idValueType = "integer"
	}
	fields = append(fields, NormalizedPresentationField{
		ID: "id", Label: PresentationLocalizedText{ZhCN: "ID", EnUS: "ID"},
		ValueType: idValueType, Format: "identifier", Required: true,
		Nullable: false, ReadOnly: true, Surfaces: []string{"detail"},
		Components: []string{}, SurfaceComponents: []NormalizedPresentationSurfaceComponents{},
		EnumValues: []NormalizedPresentationEnumValue{},
	})
	expected["detail"] = append(expected["detail"], "id")
	if m.Spec.Entity.Timestamps != nil && *m.Spec.Entity.Timestamps {
		for _, timestamp := range []struct {
			id, zh, en string
		}{{"createdAt", "创建时间", "Created at"}, {"updatedAt", "更新时间", "Updated at"}} {
			fields = append(fields, NormalizedPresentationField{
				ID: timestamp.id, Label: PresentationLocalizedText{ZhCN: timestamp.zh, EnUS: timestamp.en},
				ValueType: "date-time", Format: "date-time", Required: true,
				Nullable: false, ReadOnly: true, Surfaces: []string{"detail"},
				Components: []string{}, SurfaceComponents: []NormalizedPresentationSurfaceComponents{},
				EnumValues: []NormalizedPresentationEnumValue{},
			})
			expected["detail"] = append(expected["detail"], timestamp.id)
		}
	}
	for surface := range expected {
		sort.Strings(expected[surface])
	}
	return fields, expected
}

func normalizedEntityPresentationField(field FieldSpec) NormalizedPresentationField {
	valueType, format := presentationFieldTypeAndFormat(field)
	minimum := normalizedFloat(field.Validation.Minimum)
	maximum := normalizedFloat(field.Validation.Maximum)
	englishLabel := field.DisplayNameEn
	if englishLabel == "" {
		englishLabel = field.DisplayName
	}
	result := NormalizedPresentationField{
		ID:        field.Name,
		Label:     PresentationLocalizedText{ZhCN: field.DisplayName, EnUS: englishLabel},
		ValueType: valueType, Format: format, Required: field.Required,
		Nullable: field.Nullable, ReadOnly: field.Immutable,
		Searchable: field.Searchable, Sortable: field.Sortable, Filterable: field.Filterable,
		Surfaces: []string{}, Components: []string{},
		SurfaceComponents: []NormalizedPresentationSurfaceComponents{},
		EnumValues:        []NormalizedPresentationEnumValue{},
		Validation: NormalizedPresentationValidation{
			MinLength: field.Validation.MinLength, MaxLength: field.Validation.MaxLength,
			Minimum: minimum, Maximum: maximum, Pattern: field.Validation.Pattern,
			Precision: field.Validation.Precision, Scale: field.Validation.Scale,
		},
	}
	for _, enumValue := range field.EnumValues {
		english := enumValue.LabelEn
		if english == "" {
			english = enumValue.Label
		}
		result.EnumValues = append(result.EnumValues, NormalizedPresentationEnumValue{
			Value: enumValue.Value,
			Label: PresentationLocalizedText{ZhCN: enumValue.Label, EnUS: english},
			Color: enumValue.Color,
		})
	}
	return result
}

func presentationFieldTypeAndFormat(field FieldSpec) (string, string) {
	format := field.Validation.Format
	if format == "" {
		format = "plain"
	}
	switch field.Type {
	case "string", "text", "file", "files":
		return "string", format
	case "uuid", "relation":
		if field.Validation.Format == "" {
			format = "identifier"
		}
		return "string", format
	case "enum":
		return "enum", format
	case "bool":
		return "boolean", format
	case "int", "int64", "uint":
		return "integer", format
	case "float64", "decimal":
		return "number", format
	case "date":
		return "date", "date"
	case "datetime":
		return "date-time", "date-time"
	case "json":
		return "json", format
	default:
		return field.Type, format
	}
}

func validatePresentationSurfaceFields(
	surface string,
	sources []PresentationFieldSource,
	expected []string,
	fieldByID map[string]*NormalizedPresentationField,
	catalog *PresentationCatalog,
	add func(string, string, string),
) []NormalizedCompleteField {
	path := "spec.presentation." + surface + ".fields"
	if len(sources) == 0 {
		add(path, "required", "at least one complete field default is required")
	}
	seen := map[string]bool{}
	complete := make([]NormalizedCompleteField, 0, len(sources))
	for index, source := range sources {
		itemPath := path + "[" + strconv.Itoa(index) + "]"
		if len(source.Field) > 64 || !camelNamePattern.MatchString(source.Field) || strings.Contains(source.Field, ".") {
			add(itemPath+".field", "invalid-local-field-reference", "must be an unqualified lower-camel field identifier not exceeding 64 bytes")
		}
		if seen[source.Field] {
			add(itemPath+".field", "duplicate-field-reference", "field reference must be unique on one surface")
		}
		seen[source.Field] = true
		field, exists := fieldByID[source.Field]
		if !exists {
			add(itemPath+".field", "unknown-presentation-field", "field is not declared by the entity or generated response contract")
			continue
		}
		surfaceEligible := contains(field.Surfaces, surface)
		if surface == "search" && (field.Searchable || field.Filterable) {
			surfaceEligible = true
			if !contains(field.Surfaces, surface) {
				field.Surfaces = append(field.Surfaces, surface)
			}
		}
		if !surfaceEligible {
			add(itemPath+".field", "field-surface-incompatible", "field is not available on the "+surface+" surface")
		}
		compatible := catalog.compatibleComponents(field.ValueType, field.Format, field.ReadOnly, surface)
		allowed := compatible
		if len(source.AllowedComponents) > 0 {
			allowed = make([]string, 0, len(source.AllowedComponents))
			allowedSeen := map[string]bool{}
			for componentIndex, component := range source.AllowedComponents {
				componentPath := itemPath + ".allowedComponents[" + strconv.Itoa(componentIndex) + "]"
				if allowedSeen[component] {
					add(componentPath, "duplicate-component", "allowed component must be unique")
				}
				allowedSeen[component] = true
				if !contains(compatible, component) {
					add(componentPath, "component-incompatible", "component is not compatible with the field type, format, read-only state, and surface")
					continue
				}
				allowed = append(allowed, component)
			}
			sort.Strings(allowed)
		}
		if len(compatible) == 0 {
			add(itemPath+".component", "no-compatible-component", "catalog has no compatible component for this field and surface")
		}
		if _, exists := catalog.component(source.Component); !exists {
			add(itemPath+".component", "unknown-component", "component is not registered by the Foundation catalog")
		} else if !contains(compatible, source.Component) {
			add(itemPath+".component", "component-incompatible", "component is not compatible with the field type, format, read-only state, and surface")
		}
		if len(source.AllowedComponents) > 0 && !contains(allowed, source.Component) {
			add(itemPath+".component", "component-not-allowed", "default component must be included in allowedComponents")
		}
		if source.Order < 0 || source.Order > 10000 {
			add(itemPath+".order", "order-out-of-range", "order must be between 0 and 10000")
		}
		if source.orderOmitted {
			add(itemPath+".order", "required", "order must be explicitly declared, including zero")
		}
		if source.Width != nil {
			if surface != "list" {
				add(itemPath+".width", "width-surface-incompatible", "width is supported only on list fields")
			}
			if *source.Width < 60 || *source.Width > 1200 {
				add(itemPath+".width", "width-out-of-range", "width must be between 60 and 1200")
			}
		}
		if source.Span != nil {
			if surface != "form" && surface != "detail" {
				add(itemPath+".span", "span-surface-incompatible", "span is supported only on form and detail fields")
			}
			if *source.Span < 1 || *source.Span > 24 {
				add(itemPath+".span", "span-out-of-range", "span must be between 1 and 24")
			}
		}
		if surface == "form" && field.Required && source.Hidden {
			add(itemPath+".hidden", "required-form-field-hidden", "required form fields cannot be hidden")
		}
		if source.Label != nil {
			validateLocalizedText(itemPath+".label", *source.Label, false, add)
		}
		if source.Placeholder != nil {
			validateLocalizedText(itemPath+".placeholder", *source.Placeholder, true, add)
		}
		if source.Help != nil {
			validateLocalizedText(itemPath+".help", *source.Help, true, add)
		}
		field.SurfaceComponents = append(field.SurfaceComponents, NormalizedPresentationSurfaceComponents{
			Surface: surface, Components: append([]string(nil), allowed...),
		})
		complete = append(complete, NormalizedCompleteField{
			Field: source.Field, Label: effectiveLocalizedText(source.Label, field.Label),
			Component: source.Component, Order: source.Order, Hidden: source.Hidden,
			Width: source.Width, Span: source.Span,
			Placeholder: cloneLocalizedText(source.Placeholder), Help: cloneLocalizedText(source.Help),
		})
	}
	for _, expectedField := range expected {
		if !seen[expectedField] {
			add(path, "incomplete-surface-defaults", "missing complete default for field "+expectedField)
		}
	}
	sort.SliceStable(complete, func(i, j int) bool {
		if complete[i].Order == complete[j].Order {
			return complete[i].Field < complete[j].Field
		}
		return complete[i].Order < complete[j].Order
	})
	return complete
}

func validatePresentationDataSource(
	m *Module,
	localID string,
	catalog *PresentationCatalog,
	add func(string, string, string),
) (PresentationCatalogDataSource, bool) {
	path := "spec.presentation.dataSource"
	if !moduleNamePattern.MatchString(localID) || strings.Contains(localID, ".") {
		add(path, "invalid-local-data-source-reference", "must be an unqualified local identifier without dots")
	}
	dataSource, exists := catalog.dataSource(localID)
	if !exists {
		add(path, "unknown-data-source", "data source is not registered by the Foundation catalog")
		return PresentationCatalogDataSource{}, false
	}
	if !contains(m.Spec.API.Operations, dataSource.APIOperation) {
		add(path, "data-source-operation-unavailable", "module API does not declare the required operation "+dataSource.APIOperation)
	}
	if _, exists := m.Permission(dataSource.PermissionAction); !exists {
		add(path, "data-source-permission-unavailable", "module permissions do not declare "+dataSource.PermissionAction)
	}
	return dataSource, true
}

func validatePresentationSort(
	sources []PresentationSortSource,
	fieldByID map[string]*NormalizedPresentationField,
	dataSource PresentationCatalogDataSource,
	hasDataSource bool,
	add func(string, string, string),
) []NormalizedPresentationSort {
	result := make([]NormalizedPresentationSort, 0, len(sources))
	seen := map[string]bool{}
	if hasDataSource && len(sources) > dataSource.MaxSortFields {
		add("spec.presentation.list.defaultSort", "too-many-sort-fields", "default sort exceeds the compiled data-source maximum")
	}
	for index, source := range sources {
		path := "spec.presentation.list.defaultSort[" + strconv.Itoa(index) + "]"
		if len(source.Field) > 64 || !camelNamePattern.MatchString(source.Field) || strings.Contains(source.Field, ".") {
			add(path+".field", "invalid-local-field-reference", "sort field must be an unqualified local identifier not exceeding 64 bytes")
		}
		if seen[source.Field] {
			add(path+".field", "duplicate-sort-field", "sort field must be unique")
		}
		seen[source.Field] = true
		field, exists := fieldByID[source.Field]
		if !exists {
			add(path+".field", "unknown-sort-field", "sort references an unknown field")
		} else if !field.Sortable || !contains(field.Surfaces, "list") {
			add(path+".field", "unsupported-sort-field", "field is not sortable on the list surface")
		}
		if !contains([]string{"asc", "desc"}, source.Direction) {
			add(path+".direction", "unsupported-sort-direction", "supported values are asc and desc")
		}
		result = append(result, NormalizedPresentationSort{Field: source.Field, Direction: source.Direction})
	}
	return result
}

func validatePresentationActions(
	m *Module,
	sources []PresentationActionSource,
	catalog *PresentationCatalog,
	add func(string, string, string),
) ([]NormalizedPresentationAction, []NormalizedCompleteAction) {
	expected := map[string]PresentationCatalogAction{}
	for _, action := range catalog.Spec.Actions {
		if contains(m.Spec.API.Operations, action.APIOperation) {
			if _, hasPermission := m.Permission(action.PermissionAction); hasPermission {
				expected[action.ID] = action
			}
		}
	}
	seen := map[string]bool{}
	capabilityByID := map[string]NormalizedPresentationAction{}
	complete := make([]NormalizedCompleteAction, 0, len(sources))
	for index, source := range sources {
		path := "spec.presentation.actions[" + strconv.Itoa(index) + "]"
		if !moduleNamePattern.MatchString(source.Action) || strings.Contains(source.Action, ".") {
			add(path+".action", "invalid-local-action-reference", "must be an unqualified local identifier without dots")
		}
		if seen[source.Action] {
			add(path+".action", "duplicate-action-reference", "action reference must be unique")
		}
		seen[source.Action] = true
		catalogAction, exists := catalog.action(source.Action)
		if !exists {
			add(path+".action", "unknown-action", "action is not registered by the Foundation catalog")
			continue
		}
		if !contains(m.Spec.API.Operations, catalogAction.APIOperation) {
			add(path+".action", "action-operation-unavailable", "module API does not declare "+catalogAction.APIOperation)
		}
		permission, permissionExists := m.Permission(catalogAction.PermissionAction)
		if !permissionExists {
			add(path+".action", "action-permission-unavailable", "module permissions do not declare "+catalogAction.PermissionAction)
		}
		if !contains(catalogAction.Placements, source.Placement) {
			add(path+".placement", "action-placement-incompatible", "placement is not supported by the compiled action contract")
		}
		if source.Order < 0 || source.Order > 10000 {
			add(path+".order", "order-out-of-range", "order must be between 0 and 10000")
		}
		if source.orderOmitted {
			add(path+".order", "required", "order must be explicitly declared, including zero")
		}
		if source.Label != nil {
			validateLocalizedText(path+".label", *source.Label, false, add)
		}
		if source.Confirm != nil {
			validateLocalizedText(path+".confirm", *source.Confirm, true, add)
			if !catalogAction.Destructive {
				add(path+".confirm", "confirmation-action-incompatible", "confirmation text is supported only for destructive actions")
			}
		}
		permissionLabel := PresentationLocalizedText{ZhCN: permission.DisplayName, EnUS: permission.DisplayName}
		capabilityByID[source.Action] = NormalizedPresentationAction{
			ID:                  m.Metadata.Name + "." + catalogAction.ID,
			RequiredPermissions: []string{m.Spec.Menu.Path, presentationPermissionPath(m.Spec.Menu.Path, catalogAction.PermissionAction)},
			Placements:          append([]string(nil), catalogAction.Placements...),
			Destructive:         catalogAction.Destructive,
		}
		complete = append(complete, NormalizedCompleteAction{
			Action:    m.Metadata.Name + "." + catalogAction.ID,
			Label:     effectiveLocalizedText(source.Label, permissionLabel),
			Placement: source.Placement, Order: source.Order, Hidden: source.Hidden,
			Confirm: cloneLocalizedText(source.Confirm),
		})
	}
	for expectedID := range expected {
		if !seen[expectedID] {
			add("spec.presentation.actions", "incomplete-action-defaults", "missing complete default for action "+expectedID)
		}
	}
	actions := make([]NormalizedPresentationAction, 0, len(capabilityByID))
	for _, action := range capabilityByID {
		sort.Strings(action.RequiredPermissions)
		sort.Strings(action.Placements)
		actions = append(actions, action)
	}
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].ID < actions[j].ID })
	sort.SliceStable(complete, func(i, j int) bool {
		if complete[i].Order == complete[j].Order {
			return complete[i].Action < complete[j].Action
		}
		return complete[i].Order < complete[j].Order
	})
	return actions, complete
}

func validateLocalizedText(path string, text PresentationLocalizedText, requireBoth bool, add func(string, string, string)) {
	if text.presenceTracked {
		for _, locale := range []struct {
			name    string
			present bool
			value   string
		}{
			{name: "zh-CN", present: text.zhCNPresent, value: text.ZhCN},
			{name: "en-US", present: text.enUSPresent, value: text.EnUS},
		} {
			if locale.present && locale.value == "" {
				add(path+"."+locale.name, "localized-text-empty", "explicit localized text must not be empty")
			}
		}
	}
	if text.ZhCN == "" && text.EnUS == "" {
		add(path, "localized-text-required", "at least one localized plain-text value is required")
		return
	}
	if requireBoth && text.ZhCN == "" {
		add(path+".zh-CN", "required", "Chinese localized text is required")
	}
	if requireBoth && text.EnUS == "" {
		add(path+".en-US", "required", "English localized text is required")
	}
	for locale, value := range map[string]string{"zh-CN": text.ZhCN, "en-US": text.EnUS} {
		if len([]rune(value)) > 200 {
			add(path+"."+locale, "localized-text-too-long", "localized text must not exceed 200 characters")
		}
	}
}

func effectiveLocalizedText(source *PresentationLocalizedText, fallback PresentationLocalizedText) PresentationLocalizedText {
	if source == nil {
		return fallback
	}
	result := *source
	if source.presenceTracked {
		if !source.zhCNPresent {
			result.ZhCN = fallback.ZhCN
		}
		if !source.enUSPresent {
			result.EnUS = fallback.EnUS
		}
	} else {
		if result.ZhCN == "" {
			result.ZhCN = fallback.ZhCN
		}
		if result.EnUS == "" {
			result.EnUS = fallback.EnUS
		}
	}
	return result
}

func cloneLocalizedText(source *PresentationLocalizedText) *PresentationLocalizedText {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}

func normalizedFloat(value *float64) *string {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) {
		text := "invalid"
		return &text
	}
	number := *value
	if number == 0 {
		number = 0
	}
	text := strconv.FormatFloat(number, 'g', -1, 64)
	return &text
}

func presentationPermissionPath(menuPath, action string) string {
	return strings.TrimSuffix(menuPath, "/") + "/permissions/" + action
}

func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sortedStringSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// IsProtectedPresentationPageKey reports whether the page belongs to a core
// login, identity, configuration, release, recovery, or presentation-
// governance namespace that profiles must never control.
func IsProtectedPresentationPageKey(pageKey string) bool {
	namespace, _, _ := strings.Cut(pageKey, ".")
	return protectedPresentationPageNamespaces[namespace]
}

// ValidateUniquePresentationPageKeys performs the repository/discovery-level
// uniqueness check that cannot be proven while loading one AdminModule file.
func ValidateUniquePresentationPageKeys(modules []*Module) []Issue {
	issues := make([]Issue, 0)
	ownerByPageKey := map[string]string{}
	for index, module := range modules {
		if module == nil || module.Spec.Presentation == nil {
			continue
		}
		pageKey := module.Spec.Presentation.PageKey
		path := "modules[" + strconv.Itoa(index) + "].spec.presentation.pageKey"
		if owner, exists := ownerByPageKey[pageKey]; exists {
			issues = append(issues, Issue{
				Path: path, Code: "duplicate-page-key",
				Message: "page key is already owned by module " + owner,
			})
			continue
		}
		ownerByPageKey[pageKey] = module.Metadata.Name
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Path < issues[j].Path })
	return issues
}
