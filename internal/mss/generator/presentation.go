package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

// presentationProjection is the exact language-neutral v2 capability surface
// emitted by the generator. It deliberately excludes generator context such as
// source paths and module names; those facts cannot influence runtime identity.
type presentationProjection struct {
	PageKey             string                        `json:"pageKey"`
	DefinitionVersion   string                        `json:"definitionVersion"`
	DefinitionHash      string                        `json:"definitionHash"`
	Components          []presentationComponent       `json:"components"`
	Fields              []presentationCapabilityField `json:"fields"`
	DataSources         []presentationDataSource      `json:"dataSources"`
	Actions             []presentationAction          `json:"actions"`
	DefaultPresentation presentationDefaults          `json:"defaultPresentation"`
}

type presentationLocalizedText struct {
	ZhCN *string `json:"zh-CN,omitempty"`
	EnUS *string `json:"en-US,omitempty"`
}

type presentationComponent struct {
	ID string `json:"id"`
}

type presentationCapabilityField struct {
	ID                string                                `json:"id"`
	Label             presentationLocalizedText             `json:"label"`
	ValueType         string                                `json:"valueType"`
	Format            string                                `json:"format"`
	Required          bool                                  `json:"required"`
	Nullable          bool                                  `json:"nullable"`
	ReadOnly          bool                                  `json:"readOnly"`
	Searchable        bool                                  `json:"searchable"`
	Sortable          bool                                  `json:"sortable"`
	Filterable        bool                                  `json:"filterable"`
	Surfaces          []string                              `json:"surfaces"`
	Components        []string                              `json:"components"`
	SurfaceComponents []presentationSurfaceComponentBinding `json:"surfaceComponents"`
	EnumValues        []presentationEnumValue               `json:"enumValues"`
	Validation        presentationFieldValidation           `json:"validation"`
}

type presentationSurfaceComponentBinding struct {
	Surface    string   `json:"surface"`
	Components []string `json:"components"`
}

type presentationFieldValidation struct {
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Minimum   *string `json:"minimum,omitempty"`
	Maximum   *string `json:"maximum,omitempty"`
	Pattern   string  `json:"pattern,omitempty"`
	Precision *int    `json:"precision,omitempty"`
	Scale     *int    `json:"scale,omitempty"`
}

type presentationEnumValue struct {
	Value string                    `json:"value"`
	Label presentationLocalizedText `json:"label"`
	Color string                    `json:"color"`
}

type presentationDataSource struct {
	ID                  string   `json:"id"`
	RequiredPermissions []string `json:"requiredPermissions"`
	PageSizeOptions     []int    `json:"pageSizeOptions"`
	MaxPageSize         int      `json:"maxPageSize"`
	MaxSortFields       int      `json:"maxSortFields"`
}

type presentationAction struct {
	ID                  string   `json:"id"`
	RequiredPermissions []string `json:"requiredPermissions"`
	Placements          []string `json:"placements"`
	Destructive         bool     `json:"destructive"`
}

type presentationDefaults struct {
	Title      presentationLocalizedText   `json:"title"`
	DataSource string                      `json:"dataSource"`
	List       presentationListDefaults    `json:"list"`
	Search     presentationSearchDefaults  `json:"search"`
	Form       presentationFormDefaults    `json:"form"`
	Detail     presentationDetailDefaults  `json:"detail"`
	Actions    []presentationDefaultAction `json:"actions"`
}

type presentationListDefaults struct {
	Columns     []presentationDefaultField `json:"columns"`
	Density     string                     `json:"density"`
	PageSize    int                        `json:"pageSize"`
	DefaultSort []presentationSort         `json:"defaultSort"`
}

type presentationSearchDefaults struct {
	Fields             []presentationDefaultField `json:"fields"`
	CollapsedByDefault bool                       `json:"collapsedByDefault"`
}

type presentationFormDefaults struct {
	Fields  []presentationDefaultField `json:"fields"`
	Columns int                        `json:"columns"`
}

type presentationDetailDefaults struct {
	Fields  []presentationDefaultField `json:"fields"`
	Columns int                        `json:"columns"`
}

type presentationDefaultField struct {
	Field       string                     `json:"field"`
	Label       *presentationLocalizedText `json:"label"`
	Component   string                     `json:"component"`
	Order       int                        `json:"order"`
	Hidden      bool                       `json:"hidden"`
	Width       *int                       `json:"width,omitempty"`
	Span        *int                       `json:"span,omitempty"`
	Placeholder *presentationLocalizedText `json:"placeholder,omitempty"`
	Help        *presentationLocalizedText `json:"help,omitempty"`
	VisibleWhen json.RawMessage            `json:"visibleWhen,omitempty"`
}

type presentationDefaultAction struct {
	Action      string                     `json:"action"`
	Label       *presentationLocalizedText `json:"label"`
	Placement   string                     `json:"placement"`
	Order       int                        `json:"order"`
	Hidden      bool                       `json:"hidden"`
	Confirm     *presentationLocalizedText `json:"confirm,omitempty"`
	VisibleWhen json.RawMessage            `json:"visibleWhen,omitempty"`
}

type presentationSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type presentationViewAdapterMetadata struct {
	PageKey        string                         `json:"pageKey"`
	DefinitionHash string                         `json:"definitionHash"`
	Components     []string                       `json:"components"`
	Fields         []presentationViewFieldBinding `json:"fields"`
}

type presentationViewFieldBinding struct {
	Field        string                      `json:"field"`
	ValueType    string                      `json:"valueType"`
	Format       string                      `json:"format"`
	Nullable     bool                        `json:"nullable"`
	ReadOnly     bool                        `json:"readOnly"`
	Components   []string                    `json:"components"`
	Validation   presentationFieldValidation `json:"validation"`
	EnumValues   []presentationEnumValue     `json:"enumValues"`
	Codec        string                      `json:"codec"`
	PreviewValue any                         `json:"previewValue"`
}

type presentationBusinessAdapterMetadata struct {
	PageKey        string                                  `json:"pageKey"`
	DefinitionHash string                                  `json:"definitionHash"`
	DataSources    []presentationBusinessDataSourceBinding `json:"dataSources"`
	Actions        []presentationBusinessActionBinding     `json:"actions"`
}

type presentationBusinessDataSourceBinding struct {
	ID                  string                             `json:"id"`
	Operation           string                             `json:"operation"`
	RequiredPermissions []string                           `json:"requiredPermissions"`
	PageSizeOptions     []int                              `json:"pageSizeOptions"`
	MaxPageSize         int                                `json:"maxPageSize"`
	MaxSortFields       int                                `json:"maxSortFields"`
	QueryBindings       []presentationBusinessQueryBinding `json:"queryBindings"`
}

type presentationBusinessQueryBinding struct {
	Field     string `json:"field"`
	Parameter string `json:"parameter"`
	Kind      string `json:"kind"`
}

type presentationBusinessActionBinding struct {
	ID                  string   `json:"id"`
	Operation           string   `json:"operation"`
	RequiredPermissions []string `json:"requiredPermissions"`
	Placements          []string `json:"placements"`
	Destructive         bool     `json:"destructive"`
}

type presentationBusinessOperationBinding struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

type presentationBusinessOperationBindings struct {
	DataSources []presentationBusinessOperationBinding `json:"dataSources"`
	Actions     []presentationBusinessOperationBinding `json:"actions"`
}

type presentationSnapshot struct {
	Generated string                 `json:"$generated"`
	Manifest  presentationProjection `json:"manifest"`
}

type presentationRegistryEntry struct {
	Module     *spec.Module
	Projection *presentationProjection
	Identifier string
}

func presentationOutputGroupPaths(moduleName string, layout targetLayout) []string {
	moduleDir := filepath.Join(filepath.FromSlash(layout.ModulesDir), moduleName)
	frontendDir := filepath.Join(filepath.FromSlash(layout.GeneratedDir), "modules", moduleName)
	return []string{
		filepath.ToSlash(filepath.Join(moduleDir, "presentation_generated.go")),
		filepath.ToSlash(filepath.Join(moduleDir, "presentation_generated_test.go")),
		filepath.ToSlash(filepath.Join(moduleDir, "presentation_manifest.generated.json")),
		filepath.ToSlash(filepath.Join(frontendDir, "presentation.generated.ts")),
		filepath.ToSlash(filepath.Join(frontendDir, "presentation.generated.test.ts")),
		filepath.ToSlash(filepath.Join(frontendDir, "presentation.adapter.generated.tsx")),
	}
}

func validatePresentationOutputGroup(repository *os.Root, moduleName string, layout targetLayout) error {
	return validateManagedOutputGroup(
		repository,
		"presentation:"+moduleName,
		presentationOutputGroupPaths(moduleName, layout),
	)
}

func presentationRegistryOutputPairPaths(layout targetLayout) []string {
	return []string{
		filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.ModulesDir), "all", "presentation_generated.go")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.GeneratedDir), "presentation-registry.generated.ts")),
	}
}

func validatePresentationRegistryOutputPair(repository *os.Root, layout targetLayout) error {
	return validateManagedOutputGroup(
		repository,
		"presentation-registry",
		presentationRegistryOutputPairPaths(layout),
	)
}

func validateManagedOutputGroup(repository *os.Root, group string, paths []string) error {
	present := make([]string, 0, len(paths))
	missing := make([]string, 0, len(paths))
	for _, path := range paths {
		relative, err := rootedName(path)
		if err != nil {
			return err
		}
		if _, err := repository.Stat(relative); err == nil {
			present = append(present, path)
		} else if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, path)
		} else {
			return fmt.Errorf("inspect managed presentation output %s: %w", path, err)
		}
	}
	if len(present) == 0 || len(missing) == 0 {
		return nil
	}
	return &OutputGroupConflictError{
		Group:   group,
		Present: present,
		Missing: missing,
	}
}

func renderPresentationOutputs(module *spec.Module, data templateData, layout targetLayout) ([]output, error) {
	projection, canonical, err := normalizedPresentationProjection(module)
	if err != nil {
		return nil, err
	}
	if projection == nil {
		return nil, nil
	}
	if module.Spec.Generation.Backend == nil || !*module.Spec.Generation.Backend ||
		module.Spec.Generation.Frontend == nil || !*module.Spec.Generation.Frontend {
		return nil, errors.New("presentation generation requires both backend and frontend generation")
	}

	data.Presentation.TemplateRevision = presentationTemplateRevision
	data.Presentation.Identifier = lowerFirst(spec.PascalCase(module.Metadata.Name))
	compact, err := encodePresentationJSON(projection, "")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation definition: %w", err)
	}
	pretty, err := encodePresentationJSON(projection, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation definition: %w", err)
	}
	view := buildPresentationViewAdapter(projection)
	business, err := buildPresentationBusinessAdapter(module.Metadata.Name, projection)
	if err != nil {
		return nil, err
	}
	viewJSON, err := encodePresentationJSON(view, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation view adapter metadata: %w", err)
	}
	businessJSON, err := encodePresentationJSON(business, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation business adapter metadata: %w", err)
	}
	queryBindings, err := buildPresentationQueryBindings(projection)
	if err != nil {
		return nil, err
	}
	queryBindingsJSON, err := encodePresentationJSON(queryBindings, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation query bindings: %w", err)
	}
	operationBindings := presentationBusinessOperationBindings{
		DataSources: make([]presentationBusinessOperationBinding, 0, len(business.DataSources)),
		Actions:     make([]presentationBusinessOperationBinding, 0, len(business.Actions)),
	}
	for _, dataSource := range business.DataSources {
		operationBindings.DataSources = append(operationBindings.DataSources, presentationBusinessOperationBinding{
			ID: dataSource.ID, Operation: dataSource.Operation,
		})
	}
	for _, action := range business.Actions {
		operationBindings.Actions = append(operationBindings.Actions, presentationBusinessOperationBinding{
			ID: action.ID, Operation: action.Operation,
		})
	}
	operationBindingsJSON, err := encodePresentationJSON(operationBindings, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation operation bindings: %w", err)
	}
	data.Presentation.JSONGo = strconv.Quote(string(compact))
	data.Presentation.JSONPretty = string(pretty)
	data.Presentation.DefinitionHashTS = strconv.Quote(projection.DefinitionHash)
	data.Presentation.ViewAdapterJSONPretty = string(viewJSON)
	data.Presentation.BusinessAdapterJSONPretty = string(businessJSON)
	data.Presentation.QueryBindingsJSONPretty = strings.ReplaceAll(string(queryBindingsJSON), "\n", "\n    ")
	data.Presentation.OperationBindingsJSONPretty = strings.ReplaceAll(string(operationBindingsJSON), "\n", "\n    ")

	moduleDir := filepath.Join(filepath.FromSlash(layout.ModulesDir), module.Metadata.Name)
	frontendDir := filepath.Join(filepath.FromSlash(layout.GeneratedDir), "modules", module.Metadata.Name)
	mappings := []struct {
		template string
		path     string
	}{
		{template: "templates/module/backend/presentation.go.tmpl", path: filepath.Join(moduleDir, "presentation_generated.go")},
		{template: "templates/module/backend/presentation_test.go.tmpl", path: filepath.Join(moduleDir, "presentation_generated_test.go")},
		{template: "templates/module/frontend-v6/presentation.ts.tmpl", path: filepath.Join(frontendDir, "presentation.generated.ts")},
		{template: "templates/module/frontend-v6/presentation.test.ts.tmpl", path: filepath.Join(frontendDir, "presentation.generated.test.ts")},
		{template: "templates/module/frontend-v6/presentation.adapter.tsx.tmpl", path: filepath.Join(frontendDir, "presentation.adapter.generated.tsx")},
	}
	outputs := make([]output, 0, len(mappings)+1)
	for _, mapping := range mappings {
		content, renderErr := renderTemplate(mapping.template, data)
		if renderErr != nil {
			return nil, renderErr
		}
		if strings.HasSuffix(mapping.path, ".go") {
			content, renderErr = formatGoPresentation(mapping.path, content)
			if renderErr != nil {
				return nil, renderErr
			}
		}
		outputs = append(outputs, output{
			path: filepath.ToSlash(mapping.path), content: normalizeNewline(content), managed: true,
			source: filepath.ToSlash(mapping.template), fileMode: 0o644,
		})
	}

	// The canonical bytes are consumed here as an explicit invariant check even
	// though the human-readable snapshot stores the equivalent normalized object.
	// This prevents a projection from being emitted with a hash that came from a
	// different semantic value.
	projectedCanonical, err := canonicalPresentationProjection(projection)
	if err != nil {
		return nil, fmt.Errorf("canonicalize generated presentation projection: %w", err)
	}
	if !bytes.Equal(canonical, projectedCanonical) {
		return nil, fmt.Errorf("normalized presentation projection diverges from canonical manifest for %s", projection.PageKey)
	}
	expected := sha256.Sum256(projectedCanonical)
	if projection.DefinitionHash != "sha256:"+hex.EncodeToString(expected[:]) {
		return nil, fmt.Errorf("normalized presentation hash mismatch for %s", projection.PageKey)
	}
	snapshot := presentationSnapshot{
		Generated: "Code generated by mss module presentation template " + presentationTemplateRevision + " from " + filepath.ToSlash(module.SourcePath) + ". DO NOT EDIT.",
		Manifest:  *projection,
	}
	snapshotJSON, err := encodePresentationJSON(snapshot, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal generated presentation manifest snapshot: %w", err)
	}
	outputs = append(outputs, output{
		path:    filepath.ToSlash(filepath.Join(moduleDir, "presentation_manifest.generated.json")),
		content: snapshotJSON, managed: true, source: filepath.ToSlash(module.SourcePath), fileMode: 0o644,
	})
	return outputs, nil
}

func normalizedPresentationProjection(module *spec.Module) (*presentationProjection, []byte, error) {
	manifest, err := module.NormalizePresentation()
	if err != nil {
		return nil, nil, fmt.Errorf("normalize presentation for module %s: %w", module.Metadata.Name, err)
	}
	if manifest == nil {
		return nil, nil, nil
	}
	raw, err := encodePresentationJSON(manifest, "")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal normalized presentation for module %s: %w", module.Metadata.Name, err)
	}
	projection := &presentationProjection{}
	if err := json.Unmarshal(raw, projection); err != nil {
		return nil, nil, fmt.Errorf("project normalized presentation for module %s: %w", module.Metadata.Name, err)
	}
	if projection.DefinitionVersion != "2" {
		return nil, nil, fmt.Errorf("generated presentation %s must use definition version 2", projection.PageKey)
	}
	canonical, err := manifest.CanonicalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("canonicalize normalized presentation for module %s: %w", module.Metadata.Name, err)
	}
	return projection, canonical, nil
}

func renderPresentationRegistryOutputs(modules []*spec.Module, layout targetLayout) ([]output, error) {
	if issues := spec.ValidateUniquePresentationPageKeys(modules); len(issues) > 0 {
		return nil, &spec.ValidationError{Issues: issues}
	}
	entries := make([]presentationRegistryEntry, 0, len(modules))
	for _, module := range modules {
		projection, _, err := normalizedPresentationProjection(module)
		if err != nil {
			return nil, err
		}
		if projection == nil {
			continue
		}
		entries = append(entries, presentationRegistryEntry{
			Module: module, Projection: projection,
			Identifier: lowerFirst(spec.PascalCase(module.Metadata.Name)),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Projection.PageKey == entries[j].Projection.PageKey {
			return entries[i].Module.Metadata.Name < entries[j].Module.Metadata.Name
		}
		return entries[i].Projection.PageKey < entries[j].Projection.PageKey
	})

	var backend strings.Builder
	backend.WriteString("// Code generated by mss module presentation template " + presentationTemplateRevision + " sync. DO NOT EDIT.\n\n")
	backend.WriteString("// Package all exposes explicit generated business-module inventories.\n")
	backend.WriteString("package all\n\n")
	backend.WriteString("import (\n")
	fmt.Fprintf(&backend, "\t%q\n", layout.AdminModule+"/presentation")
	for index, entry := range entries {
		moduleImport, err := layout.moduleImportPath(entry.Module.Metadata.Name)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&backend, "\tpresentationModule%d %q\n", index, moduleImport)
	}
	backend.WriteString(")\n\n")
	backend.WriteString("// Presentations returns a new ordered slice of generated v2 definitions.\n")
	backend.WriteString("// It is inventory only; P2B application composition owns validation and freezing.\n")
	backend.WriteString("func Presentations() []presentation.CapabilityDefinition {\n")
	backend.WriteString("\treturn []presentation.CapabilityDefinition{\n")
	for index, entry := range entries {
		fmt.Fprintf(&backend, "\t\tpresentationModule%d.PresentationDefinition(), // %s %s\n", index, entry.Projection.PageKey, entry.Projection.DefinitionHash)
	}
	backend.WriteString("\t}\n}\n")
	formattedBackend, err := format.Source([]byte(backend.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated presentation registry: %w", err)
	}

	var frontend strings.Builder
	frontend.WriteString("// Code generated by mss module presentation template " + presentationTemplateRevision + " sync. DO NOT EDIT.\n\n")
	for _, entry := range entries {
		fmt.Fprintf(&frontend,
			"import { %sPresentationDefinition } from './modules/%s/presentation.generated';\n",
			entry.Identifier, entry.Module.Metadata.Name,
		)
		fmt.Fprintf(&frontend,
			"import { %sPresentationBusinessAdapterMetadata, %sPresentationViewAdapterMetadata } from './modules/%s/presentation.adapter.generated';\n",
			entry.Identifier, entry.Identifier, entry.Module.Metadata.Name,
		)
	}
	if len(entries) > 0 {
		frontend.WriteString("\n")
	}
	frontend.WriteString("export const generatedPresentationInventory = [\n")
	for _, entry := range entries {
		fmt.Fprintf(&frontend, "  %s,\n", strconv.Quote(entry.Projection.PageKey))
	}
	frontend.WriteString("] as const;\n\n")
	frontend.WriteString("export const generatedPresentationRegistry = {\n")
	for _, entry := range entries {
		fmt.Fprintf(&frontend, "  %s: {\n", strconv.Quote(entry.Projection.PageKey))
		fmt.Fprintf(&frontend, "    definitionHash: %s,\n", strconv.Quote(entry.Projection.DefinitionHash))
		fmt.Fprintf(&frontend, "    definition: %sPresentationDefinition,\n", entry.Identifier)
		fmt.Fprintf(&frontend, "    viewAdapter: %sPresentationViewAdapterMetadata,\n", entry.Identifier)
		fmt.Fprintf(&frontend, "    businessAdapter: %sPresentationBusinessAdapterMetadata,\n", entry.Identifier)
		frontend.WriteString("  },\n")
	}
	frontend.WriteString("} as const;\n")

	source := filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.ModulesDir), "*", "module.yaml"))
	return []output{
		{
			path:    filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.ModulesDir), "all", "presentation_generated.go")),
			content: normalizeNewline(formattedBackend), managed: true, source: source, fileMode: 0o644,
		},
		{
			path:    filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.GeneratedDir), "presentation-registry.generated.ts")),
			content: normalizeNewline([]byte(frontend.String())), managed: true, source: source, fileMode: 0o644,
		},
	}, nil
}

func encodePresentationJSON(value any, indent string) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if indent != "" {
		encoder.SetIndent("", indent)
	}
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func canonicalPresentationProjection(projection *presentationProjection) ([]byte, error) {
	if projection == nil {
		return nil, errors.New("presentation projection is nil")
	}
	value := struct {
		PageKey             string                        `json:"pageKey"`
		DefinitionVersion   string                        `json:"definitionVersion"`
		Components          []presentationComponent       `json:"components"`
		Fields              []presentationCapabilityField `json:"fields"`
		DataSources         []presentationDataSource      `json:"dataSources"`
		Actions             []presentationAction          `json:"actions"`
		DefaultPresentation presentationDefaults          `json:"defaultPresentation"`
	}{
		PageKey: projection.PageKey, DefinitionVersion: projection.DefinitionVersion,
		Components: projection.Components, Fields: projection.Fields, DataSources: projection.DataSources,
		Actions: projection.Actions, DefaultPresentation: projection.DefaultPresentation,
	}
	return spec.CanonicalPresentationJSON(value)
}

func formatGoPresentation(path string, content []byte) ([]byte, error) {
	formatted, err := format.Source(content)
	if err != nil {
		return nil, fmt.Errorf("format generated Go file %s: %w", path, err)
	}
	return formatted, nil
}

func buildPresentationViewAdapter(projection *presentationProjection) presentationViewAdapterMetadata {
	components := make([]string, 0, len(projection.Components))
	for _, component := range projection.Components {
		components = append(components, component.ID)
	}
	sort.Strings(components)
	fields := make([]presentationViewFieldBinding, 0, len(projection.Fields))
	for _, field := range projection.Fields {
		fields = append(fields, presentationViewFieldBinding{
			Field: field.ID, ValueType: field.ValueType, Format: field.Format,
			Nullable: field.Nullable, ReadOnly: field.ReadOnly, Components: append([]string(nil), field.Components...),
			Validation: field.Validation, EnumValues: append([]presentationEnumValue{}, field.EnumValues...),
			Codec: presentationCodec(field), PreviewValue: presentationPreviewValue(field),
		})
	}
	return presentationViewAdapterMetadata{
		PageKey: projection.PageKey, DefinitionHash: projection.DefinitionHash,
		Components: components, Fields: fields,
	}
}

func buildPresentationBusinessAdapter(moduleName string, projection *presentationProjection) (presentationBusinessAdapterMetadata, error) {
	result := presentationBusinessAdapterMetadata{PageKey: projection.PageKey, DefinitionHash: projection.DefinitionHash}
	catalog, err := spec.DefaultPresentationCatalog()
	if err != nil {
		return presentationBusinessAdapterMetadata{}, fmt.Errorf("load presentation catalog for generated business adapter: %w", err)
	}
	queryBindings, err := buildPresentationQueryBindings(projection)
	if err != nil {
		return presentationBusinessAdapterMetadata{}, err
	}
	for _, dataSource := range projection.DataSources {
		localID, err := localPresentationReference(moduleName, dataSource.ID)
		if err != nil {
			return presentationBusinessAdapterMetadata{}, err
		}
		operation, err := presentationDataSourceOperation(catalog, localID)
		if err != nil {
			return presentationBusinessAdapterMetadata{}, err
		}
		result.DataSources = append(result.DataSources, presentationBusinessDataSourceBinding{
			ID: dataSource.ID, Operation: operation,
			RequiredPermissions: append([]string(nil), dataSource.RequiredPermissions...),
			PageSizeOptions:     append([]int(nil), dataSource.PageSizeOptions...), MaxPageSize: dataSource.MaxPageSize,
			MaxSortFields: dataSource.MaxSortFields, QueryBindings: append([]presentationBusinessQueryBinding{}, queryBindings...),
		})
	}
	for _, action := range projection.Actions {
		localID, err := localPresentationReference(moduleName, action.ID)
		if err != nil {
			return presentationBusinessAdapterMetadata{}, err
		}
		operation, err := presentationActionOperation(catalog, localID)
		if err != nil {
			return presentationBusinessAdapterMetadata{}, err
		}
		result.Actions = append(result.Actions, presentationBusinessActionBinding{
			ID: action.ID, Operation: operation,
			RequiredPermissions: append([]string(nil), action.RequiredPermissions...),
			Placements:          append([]string(nil), action.Placements...), Destructive: action.Destructive,
		})
	}
	return result, nil
}

func presentationDataSourceOperation(catalog *spec.PresentationCatalog, localID string) (string, error) {
	for _, dataSource := range catalog.Spec.DataSources {
		if dataSource.ID == localID {
			return dataSource.APIOperation, nil
		}
	}
	return "", fmt.Errorf("generated presentation data source %q is not registered by the Foundation catalog", localID)
}

func presentationActionOperation(catalog *spec.PresentationCatalog, localID string) (string, error) {
	for _, action := range catalog.Spec.Actions {
		if action.ID == localID {
			return action.APIOperation, nil
		}
	}
	return "", fmt.Errorf("generated presentation action %q is not registered by the Foundation catalog", localID)
}

func buildPresentationQueryBindings(projection *presentationProjection) ([]presentationBusinessQueryBinding, error) {
	fieldByID := make(map[string]presentationCapabilityField, len(projection.Fields))
	for _, field := range projection.Fields {
		fieldByID[field.ID] = field
	}
	bindings := make([]presentationBusinessQueryBinding, 0, len(projection.DefaultPresentation.Search.Fields))
	parameters := make(map[string]string, len(projection.DefaultPresentation.Search.Fields))
	for _, searchField := range projection.DefaultPresentation.Search.Fields {
		field, exists := fieldByID[searchField.Field]
		if !exists {
			return nil, fmt.Errorf("generated presentation search field %q is not compiled", searchField.Field)
		}
		binding := presentationBusinessQueryBinding{Field: field.ID}
		switch {
		case field.Searchable:
			binding.Parameter = "q"
			binding.Kind = "keyword"
		case field.Filterable:
			binding.Parameter = field.ID
			binding.Kind = "filter"
		default:
			return nil, fmt.Errorf("generated presentation search field %q is neither searchable nor filterable", field.ID)
		}
		if previous, duplicate := parameters[binding.Parameter]; duplicate {
			return nil, fmt.Errorf(
				"generated presentation search fields %q and %q both bind query parameter %q",
				previous, binding.Field, binding.Parameter,
			)
		}
		parameters[binding.Parameter] = binding.Field
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func localPresentationReference(moduleName, qualified string) (string, error) {
	prefix := moduleName + "."
	local := strings.TrimPrefix(qualified, prefix)
	if local == qualified || local == "" || strings.Contains(local, ".") {
		return "", fmt.Errorf("generated presentation reference %q is not qualified by module %s", qualified, moduleName)
	}
	return local, nil
}

func presentationCodec(field presentationCapabilityField) string {
	if field.Format != "" && field.Format != "plain" {
		return field.Format
	}
	switch field.ValueType {
	case "enum", "boolean", "date", "date-time", "number":
		return field.ValueType
	default:
		return "string"
	}
}

func presentationPreviewValue(field presentationCapabilityField) any {
	if len(field.EnumValues) > 0 {
		return field.EnumValues[0].Value
	}
	if field.Format == "email" {
		return "preview@example.test"
	}
	switch field.ValueType {
	case "boolean":
		return true
	case "number":
		return 1
	case "date":
		return "2026-01-01"
	case "date-time":
		return "2026-01-01T00:00:00Z"
	default:
		return "PREVIEW_" + strings.ToUpper(spec.SnakeCase(field.ID))
	}
}

func lowerFirst(value string) string {
	if value == "" {
		return "presentation"
	}
	return strings.ToLower(value[:1]) + value[1:]
}
