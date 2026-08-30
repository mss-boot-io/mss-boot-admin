package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	identifierPattern      = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
	fieldIdentifierPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	enumValuePattern       = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	definitionHashPattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	allowedValueTypes      = stringSet("string", "integer", "number", "boolean", "enum", "date", "date-time", "json")
	allowedFieldFormats    = stringSet("plain", "email", "identifier", "date", "date-time")
	allowedDensity         = stringSet("compact", "middle", "large")
	allowedDirections      = stringSet("asc", "desc")
	allowedOperators       = stringSet("eq", "neq", "in", "not-in", "exists", "not-exists", "gt", "gte", "lt", "lte")
)

const maxSafeJSONInteger = int64(1<<53 - 1)

func ParseDocument(raw []byte) (*Document, []Issue) {
	issues := make([]Issue, 0)
	if len(raw) == 0 {
		return nil, []Issue{issue("empty-document", "$", "presentation document is empty")}
	}
	if len(raw) > MaxDocumentBytes {
		return nil, []Issue{issue("document-too-large", "$", fmt.Sprintf("presentation document exceeds %d bytes", MaxDocumentBytes))}
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return nil, []Issue{issue("invalid-json", "$", err.Error())}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	profile := &Profile{}
	if err := decoder.Decode(profile); err != nil {
		return nil, []Issue{issue("invalid-document", "$", err.Error())}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, []Issue{issue("invalid-json", "$", err.Error())}
	}

	var generic any
	genericDecoder := json.NewDecoder(bytes.NewReader(raw))
	genericDecoder.UseNumber()
	if err := genericDecoder.Decode(&generic); err != nil {
		return nil, []Issue{issue("invalid-json", "$", err.Error())}
	}
	inspectUnexpectedNulls(generic, "$", false, &issues)
	validateProfileShape(profile, &issues)
	if len(issues) > 0 {
		sortIssues(issues)
		return nil, issues
	}
	canonical, err := canonicalJSONBytes(raw)
	if err != nil {
		return nil, []Issue{issue("invalid-json", "$", err.Error())}
	}
	return &Document{Profile: profile, Canonical: canonical, Digest: sha256Digest(canonical)}, nil
}

func inspectUnexpectedNulls(value any, path string, allowNull bool, issues *[]Issue) {
	switch current := value.(type) {
	case nil:
		if !allowNull {
			*issues = append(*issues, issue("unexpected-null", path, "null is not allowed here"))
		}
	case []any:
		for index := range current {
			inspectUnexpectedNulls(current[index], fmt.Sprintf("%s[%d]", path, index), allowNull, issues)
		}
	case map[string]any:
		for key, nested := range current {
			inspectUnexpectedNulls(nested, path+"."+key, key == "value", issues)
		}
	}
}

func validateProfileShape(profile *Profile, issues *[]Issue) {
	if profile.APIVersion != APIVersion {
		addIssue(issues, "invalid-api-version", "apiVersion", "apiVersion must be "+APIVersion)
	}
	if profile.Kind != Kind {
		addIssue(issues, "invalid-kind", "kind", "kind must be "+Kind)
	}
	validateIdentifier(profile.Metadata.Name, "metadata.name", issues)
	validateIdentifier(profile.Metadata.PageKey, "metadata.pageKey", issues)
	if !definitionHashPattern.MatchString(profile.Metadata.DefinitionHash) {
		addIssue(issues, "invalid-definition-hash", "metadata.definitionHash", "definition hash must use sha256 and 64 lowercase hexadecimal characters")
	}
	if profile.Metadata.Description != nil && utf8.RuneCountInString(*profile.Metadata.Description) > 500 {
		addIssue(issues, "description-too-long", "metadata.description", "description exceeds 500 characters")
	}
	validateScope(profile.Metadata.Scope, issues)
	validateProfileSpec(&profile.Spec, issues)
}

func validateScope(scope Scope, issues *[]Issue) {
	switch scope.Kind {
	case ScopeApplication:
		if scope.Subject != nil {
			addIssue(issues, "unexpected-scope-subject", "metadata.scope.subject", "application scope cannot contain a subject")
		}
	case ScopeRole:
		if scope.Subject == nil {
			addIssue(issues, "missing-scope-subject", "metadata.scope.subject", "role scope requires a subject")
			break
		}
		validateOpaqueScopeSubject(*scope.Subject, issues)
	case ScopeUser:
		if scope.Subject == nil {
			addIssue(issues, "invalid-scope-subject", "metadata.scope.subject", "role and user scope subjects must contain 1 to 160 characters")
			break
		}
		validateOpaqueScopeSubject(*scope.Subject, issues)
	default:
		addIssue(issues, "invalid-scope-kind", "metadata.scope.kind", "scope kind must be application, role, or user")
	}
}

func validateOpaqueScopeSubject(subject string, issues *[]Issue) {
	length := utf8.RuneCountInString(subject)
	if length < 1 || length > 160 {
		addIssue(issues, "invalid-scope-subject", "metadata.scope.subject", "role and user scope subjects must contain 1 to 160 characters")
	}
}

func validateProfileSpec(spec *ProfileSpec, issues *[]Issue) {
	if spec.Title == nil && spec.DataSource == nil && spec.List == nil && spec.Search == nil &&
		spec.Form == nil && spec.Detail == nil && spec.Actions == nil {
		addIssue(issues, "empty-spec", "spec", "presentation spec needs at least one property")
	}
	validateLocalizedText(spec.Title, "spec.title", issues)
	if spec.DataSource != nil {
		validateIdentifier(*spec.DataSource, "spec.dataSource", issues)
	}
	if spec.List != nil {
		validateListPatch(spec.List, issues)
	}
	if spec.Search != nil {
		if spec.Search.Fields == nil && spec.Search.CollapsedByDefault == nil {
			addIssue(issues, "empty-search", "spec.search", "search needs at least one property")
		}
		if spec.Search.Fields != nil {
			validateFieldPatches(*spec.Search.Fields, "spec.search.fields", issues)
		}
	}
	if spec.Form != nil {
		validateFormLike(spec.Form.Fields, spec.Form.Columns, "spec.form", issues)
	}
	if spec.Detail != nil {
		validateFormLike(spec.Detail.Fields, spec.Detail.Columns, "spec.detail", issues)
	}
	if spec.Actions != nil {
		if len(*spec.Actions) > 64 {
			addIssue(issues, "too-many-actions", "spec.actions", "actions exceed the limit of 64")
		}
		for index := range *spec.Actions {
			validateActionPatch(&(*spec.Actions)[index], fmt.Sprintf("spec.actions[%d]", index), issues)
		}
	}
}

func validateListPatch(patch *ListPatch, issues *[]Issue) {
	if patch.Columns == nil && patch.Density == nil && patch.PageSize == nil && patch.DefaultSort == nil {
		addIssue(issues, "empty-list", "spec.list", "list needs at least one property")
	}
	if patch.Columns != nil {
		validateFieldPatches(*patch.Columns, "spec.list.columns", issues)
	}
	if patch.Density != nil {
		if _, ok := allowedDensity[*patch.Density]; !ok {
			addIssue(issues, "invalid-density", "spec.list.density", "density must be compact, middle, or large")
		}
	}
	if patch.PageSize != nil && (*patch.PageSize < 1 || *patch.PageSize > 200) {
		addIssue(issues, "invalid-page-size", "spec.list.pageSize", "page size must be 1 to 200")
	}
	if patch.DefaultSort != nil {
		validateSorts(*patch.DefaultSort, "spec.list.defaultSort", issues)
	}
}

func validateFormLike(fields *[]FieldPatch, columns *int, path string, issues *[]Issue) {
	if fields == nil && columns == nil {
		addIssue(issues, "empty-layout", path, "layout needs at least one property")
	}
	if fields != nil {
		validateFieldPatches(*fields, path+".fields", issues)
	}
	if columns != nil && (*columns < 1 || *columns > 4) {
		addIssue(issues, "invalid-layout-columns", path+".columns", "layout columns must be 1 to 4")
	}
}

func validateFieldPatches(fields []FieldPatch, path string, issues *[]Issue) {
	if len(fields) > 100 {
		addIssue(issues, "too-many-fields", path, "field collection exceeds the limit of 100")
	}
	for index := range fields {
		currentPath := fmt.Sprintf("%s[%d]", path, index)
		field := &fields[index]
		validateIdentifier(field.Field, currentPath+".field", issues)
		validateLocalizedText(field.Label, currentPath+".label", issues)
		validateLocalizedText(field.Placeholder, currentPath+".placeholder", issues)
		validateLocalizedText(field.Help, currentPath+".help", issues)
		if field.Component != nil {
			validateIdentifier(*field.Component, currentPath+".component", issues)
		}
		if field.Order != nil && (*field.Order < 0 || *field.Order > 10000) {
			addIssue(issues, "invalid-field-order", currentPath+".order", "field order must be 0 to 10000")
		}
		if field.Width != nil && (*field.Width < 60 || *field.Width > 1200) {
			addIssue(issues, "invalid-field-width", currentPath+".width", "field width must be 60 to 1200")
		}
		if field.Span != nil && (*field.Span < 1 || *field.Span > 24) {
			addIssue(issues, "invalid-field-span", currentPath+".span", "field span must be 1 to 24")
		}
		validateConditionShape(field.VisibleWhen, currentPath+".visibleWhen", issues, 0)
	}
}

func validateActionPatch(action *ActionPatch, path string, issues *[]Issue) {
	validateIdentifier(action.Action, path+".action", issues)
	validateLocalizedText(action.Label, path+".label", issues)
	validateLocalizedText(action.Confirm, path+".confirm", issues)
	if action.Placement != nil && !validPlacement(*action.Placement) {
		addIssue(issues, "invalid-action-placement", path+".placement", "action placement is not supported")
	}
	if action.Order != nil && (*action.Order < 0 || *action.Order > 10000) {
		addIssue(issues, "invalid-action-order", path+".order", "action order must be 0 to 10000")
	}
	validateConditionShape(action.VisibleWhen, path+".visibleWhen", issues, 0)
}

func validateSorts(sorts []Sort, path string, issues *[]Issue) {
	if len(sorts) > 3 {
		addIssue(issues, "too-many-sort-fields", path, "default sort exceeds the limit of 3")
	}
	seen := make(map[string]struct{}, len(sorts))
	for index := range sorts {
		currentPath := fmt.Sprintf("%s[%d]", path, index)
		validateIdentifier(sorts[index].Field, currentPath+".field", issues)
		if _, ok := allowedDirections[sorts[index].Direction]; !ok {
			addIssue(issues, "invalid-sort-direction", currentPath+".direction", "sort direction must be asc or desc")
		}
		key := sorts[index].Field + "\x00" + sorts[index].Direction
		if _, duplicate := seen[key]; duplicate {
			addIssue(issues, "duplicate-sort", currentPath, "default sort contains a duplicate entry")
		}
		seen[key] = struct{}{}
	}
}

func validateLocalizedText(value *LocalizedText, path string, issues *[]Issue) {
	if value == nil {
		return
	}
	if value.ZhCN == nil && value.EnUS == nil {
		addIssue(issues, "empty-localized-text", path, "localized text requires zh-CN or en-US")
	}
	for locale, text := range map[string]*string{"zh-CN": value.ZhCN, "en-US": value.EnUS} {
		if text == nil {
			continue
		}
		length := utf8.RuneCountInString(*text)
		if length < 1 || length > 200 {
			addIssue(issues, "invalid-localized-text", path+"."+locale, "localized text must contain 1 to 200 characters")
		}
	}
}

func validateConditionShape(condition *Condition, path string, issues *[]Issue, depth int) {
	if condition == nil {
		return
	}
	if depth > 8 {
		addIssue(issues, "condition-depth", path, "condition depth exceeds 8")
		return
	}
	variants := 0
	predicate := condition.Field != nil || condition.Operator != nil || condition.Value != nil
	if predicate {
		variants++
	}
	if condition.All != nil {
		variants++
	}
	if condition.Any != nil {
		variants++
	}
	if condition.Not != nil {
		variants++
	}
	if variants != 1 {
		addIssue(issues, "invalid-condition-shape", path, "condition must contain exactly one predicate, all, any, or not variant")
		return
	}
	if condition.All != nil {
		validateConditionGroup(*condition.All, path+".all", issues, depth)
		return
	}
	if condition.Any != nil {
		validateConditionGroup(*condition.Any, path+".any", issues, depth)
		return
	}
	if condition.Not != nil {
		validateConditionShape(condition.Not, path+".not", issues, depth+1)
		return
	}
	if condition.Field == nil {
		addIssue(issues, "missing-condition-field", path+".field", "predicate field is required")
	} else {
		validateIdentifier(*condition.Field, path+".field", issues)
	}
	if condition.Operator == nil {
		addIssue(issues, "missing-condition-operator", path+".operator", "predicate operator is required")
		return
	}
	if _, ok := allowedOperators[*condition.Operator]; !ok {
		addIssue(issues, "unknown-condition-operator", path+".operator", "condition operator is not supported")
		return
	}
	presence := *condition.Operator == "exists" || *condition.Operator == "not-exists"
	if presence && condition.Value != nil {
		addIssue(issues, "unexpected-condition-value", path+".value", "presence operators take no value")
	}
	if !presence && condition.Value == nil {
		addIssue(issues, "missing-condition-value", path+".value", "comparison operator requires a value")
	}
	if condition.Value != nil {
		validateConditionValue(*condition.Value, path+".value", issues)
	}
}

func validateConditionGroup(group []Condition, path string, issues *[]Issue, depth int) {
	if len(group) < 1 || len(group) > 20 {
		addIssue(issues, "condition-size", path, "condition group must contain 1 to 20 items")
	}
	for index := range group {
		validateConditionShape(&group[index], fmt.Sprintf("%s[%d]", path, index), issues, depth+1)
	}
}

func validateConditionValue(raw json.RawMessage, path string, issues *[]Issue) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		addIssue(issues, "invalid-condition-value", path, err.Error())
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		addIssue(issues, "invalid-condition-value", path, err.Error())
		return
	}
	if values, ok := value.([]any); ok {
		if len(values) < 1 || len(values) > 50 {
			addIssue(issues, "condition-value-size", path, "condition value array must contain 1 to 50 items")
		}
		seen := make(map[string]struct{}, len(values))
		for index := range values {
			if !isScalar(values[index]) {
				addIssue(issues, "invalid-condition-value", fmt.Sprintf("%s[%d]", path, index), "condition values must be scalar")
				continue
			}
			canonical, _ := canonicalJSON(values[index])
			key := string(canonical)
			if _, duplicate := seen[key]; duplicate {
				addIssue(issues, "duplicate-condition-value", fmt.Sprintf("%s[%d]", path, index), "condition value array contains a duplicate")
			}
			seen[key] = struct{}{}
		}
		return
	}
	if !isScalar(value) {
		addIssue(issues, "invalid-condition-value", path, "condition value must be a scalar or scalar array")
	}
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, json.Number:
		return true
	default:
		return false
	}
}

func ValidateProfile(capability *CapabilityDefinition, profile *Profile) []Issue {
	issues := make([]Issue, 0)
	if profile == nil {
		return []Issue{issue("missing-profile", "$", "presentation profile is required")}
	}
	if capability == nil {
		return []Issue{issue("unknown-page", "metadata.pageKey", "page is not registered by the server")}
	}
	if profile.Metadata.PageKey != capability.PageKey {
		addIssue(&issues, "page-key-mismatch", "metadata.pageKey", "profile page key does not match the selected capability")
	}
	if profile.Metadata.DefinitionHash != capability.DefinitionHash {
		addIssue(&issues, "definition-drift", "metadata.definitionHash", "profile definition hash does not match the current capability")
	}
	if capabilityUsesLimitedTablePresentation(capability) {
		validateLimitedPresentationConditions(&profile.Spec, &issues)
	}
	fieldDefinitions := make(map[string]CapabilityField, len(capability.Fields))
	for _, field := range capability.Fields {
		fieldDefinitions[field.ID] = field
	}
	componentIDs := make(map[string]struct{}, len(capability.Components))
	for _, component := range capability.Components {
		componentIDs[component.ID] = struct{}{}
	}
	if profile.Spec.DataSource != nil && !containsDataSource(capability.DataSources, *profile.Spec.DataSource) {
		addIssue(&issues, "unknown-data-source", "spec.dataSource", "profile references an unknown data source")
	}
	effectiveDataSourceID := capability.DefaultPresentation.DataSource
	if profile.Spec.DataSource != nil {
		effectiveDataSourceID = *profile.Spec.DataSource
	}
	effectiveDataSource, hasEffectiveDataSource := findDataSource(capability.DataSources, effectiveDataSourceID)
	if profile.Spec.List != nil {
		if profile.Spec.List.Columns != nil {
			validateSemanticFields(*profile.Spec.List.Columns, SurfaceList, "spec.list.columns", fieldDefinitions, componentIDs, capability.DefinitionVersion == DefinitionVersionV2, &issues)
		}
		if profile.Spec.List.DefaultSort != nil {
			validateSemanticSort(*profile.Spec.List.DefaultSort, "spec.list.defaultSort", fieldDefinitions, &issues)
			if capability.DefinitionVersion == DefinitionVersionV2 && hasEffectiveDataSource && len(*profile.Spec.List.DefaultSort) > effectiveDataSource.MaxSortFields {
				addIssue(&issues, "too-many-data-source-sort-fields", "spec.list.defaultSort", "default sort exceeds the compiled data source limit")
			}
		}
		if capability.DefinitionVersion == DefinitionVersionV2 && profile.Spec.List.PageSize != nil && hasEffectiveDataSource {
			validateCapabilityPageSize(*profile.Spec.List.PageSize, effectiveDataSource, "spec.list.pageSize", &issues)
		}
	}
	if profile.Spec.Search != nil && profile.Spec.Search.Fields != nil {
		validateSemanticFields(*profile.Spec.Search.Fields, SurfaceSearch, "spec.search.fields", fieldDefinitions, componentIDs, capability.DefinitionVersion == DefinitionVersionV2, &issues)
	}
	if profile.Spec.Form != nil && profile.Spec.Form.Fields != nil {
		validateSemanticFields(*profile.Spec.Form.Fields, SurfaceForm, "spec.form.fields", fieldDefinitions, componentIDs, capability.DefinitionVersion == DefinitionVersionV2, &issues)
	}
	if profile.Spec.Detail != nil && profile.Spec.Detail.Fields != nil {
		validateSemanticFields(*profile.Spec.Detail.Fields, SurfaceDetail, "spec.detail.fields", fieldDefinitions, componentIDs, capability.DefinitionVersion == DefinitionVersionV2, &issues)
	}
	if profile.Spec.Actions != nil {
		validateSemanticActions(*profile.Spec.Actions, capability.Actions, fieldDefinitions, &issues)
	}
	sortIssues(issues)
	return issues
}

// capabilityUsesLimitedTablePresentation identifies the bounded Foundation
// table contract. These capabilities intentionally expose no configurable
// form, detail, or action surface; their handwritten consumers do not have a
// row-value context in which list/search conditions could be evaluated.
func capabilityUsesLimitedTablePresentation(capability *CapabilityDefinition) bool {
	return len(capability.Actions) == 0 &&
		len(capability.DefaultPresentation.Form.Fields) == 0 &&
		len(capability.DefaultPresentation.Detail.Fields) == 0 &&
		len(capability.DefaultPresentation.Actions) == 0
}

func validateLimitedPresentationConditions(spec *ProfileSpec, issues *[]Issue) {
	collections := []struct {
		path   string
		fields *[]FieldPatch
	}{
		{path: "spec.list.columns", fields: fieldPatches(spec.List, func(patch *ListPatch) *[]FieldPatch { return patch.Columns })},
		{path: "spec.search.fields", fields: fieldPatches(spec.Search, func(patch *SearchPatch) *[]FieldPatch { return patch.Fields })},
		{path: "spec.form.fields", fields: fieldPatches(spec.Form, func(patch *FormPatch) *[]FieldPatch { return patch.Fields })},
		{path: "spec.detail.fields", fields: fieldPatches(spec.Detail, func(patch *DetailPatch) *[]FieldPatch { return patch.Fields })},
	}
	for _, collection := range collections {
		if collection.fields == nil {
			continue
		}
		for index, field := range *collection.fields {
			if field.VisibleWhen != nil {
				addIssue(issues, "unsupported-limited-condition", fmt.Sprintf("%s[%d].visibleWhen", collection.path, index), "visibility conditions are not supported by limited table pages")
			}
		}
	}
	if spec.Actions != nil {
		for index, action := range *spec.Actions {
			if action.VisibleWhen != nil {
				addIssue(issues, "unsupported-limited-condition", fmt.Sprintf("spec.actions[%d].visibleWhen", index), "visibility conditions are not supported by limited table pages")
			}
		}
	}
}

func fieldPatches[T any](patch *T, selectFields func(*T) *[]FieldPatch) *[]FieldPatch {
	if patch == nil {
		return nil
	}
	return selectFields(patch)
}

func validateSemanticFields(fields []FieldPatch, surface Surface, path string, definitions map[string]CapabilityField, components map[string]struct{}, strictSurfaceComponents bool, issues *[]Issue) {
	seen := make(map[string]struct{}, len(fields))
	for index := range fields {
		currentPath := fmt.Sprintf("%s[%d]", path, index)
		field := &fields[index]
		if _, duplicate := seen[field.Field]; duplicate {
			addIssue(issues, "duplicate-field", currentPath+".field", "field is configured more than once")
		}
		seen[field.Field] = struct{}{}
		definition, ok := definitions[field.Field]
		if !ok {
			addIssue(issues, "unknown-field", currentPath+".field", "profile references an unknown field")
			continue
		}
		if !containsSurface(definition.Surfaces, surface) {
			addIssue(issues, "unsupported-field-surface", currentPath+".field", "field is not registered for this surface")
		}
		if field.Component != nil {
			if _, ok = components[*field.Component]; !ok {
				addIssue(issues, "unknown-component", currentPath+".component", "profile references an unknown component")
			} else if !capabilityFieldSupportsComponent(definition, surface, *field.Component, strictSurfaceComponents) {
				addIssue(issues, "unsupported-field-component", currentPath+".component", "component is not allowed for this field")
			}
		}
		if surface == SurfaceForm && definition.Required && field.Hidden != nil && *field.Hidden {
			addIssue(issues, "required-form-field-hidden", currentPath+".hidden", "required form field cannot be hidden")
		}
		if surface == SurfaceForm && definition.Required && field.VisibleWhen != nil {
			addIssue(issues, "required-form-field-conditional", currentPath+".visibleWhen", "required form field cannot be conditionally hidden")
		}
		validateSemanticCondition(field.VisibleWhen, currentPath+".visibleWhen", definitions, issues)
	}
}

func validateSemanticSort(sorts []Sort, path string, fields map[string]CapabilityField, issues *[]Issue) {
	for index := range sorts {
		definition, ok := fields[sorts[index].Field]
		if !ok {
			addIssue(issues, "unknown-sort-field", fmt.Sprintf("%s[%d].field", path, index), "sort references an unknown field")
		} else if !definition.Sortable {
			addIssue(issues, "unsupported-sort-field", fmt.Sprintf("%s[%d].field", path, index), "field is not sortable")
		}
	}
}

func validateSemanticActions(actions []ActionPatch, definitions []CapabilityAction, fields map[string]CapabilityField, issues *[]Issue) {
	byID := make(map[string]CapabilityAction, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}
	seen := make(map[string]struct{}, len(actions))
	for index := range actions {
		path := fmt.Sprintf("spec.actions[%d]", index)
		action := &actions[index]
		if _, duplicate := seen[action.Action]; duplicate {
			addIssue(issues, "duplicate-action", path+".action", "action is configured more than once")
		}
		seen[action.Action] = struct{}{}
		definition, ok := byID[action.Action]
		if !ok {
			addIssue(issues, "unknown-action", path+".action", "profile references an unknown action")
		} else if action.Placement != nil && !containsPlacement(definition.Placements, *action.Placement) {
			addIssue(issues, "unsupported-action-placement", path+".placement", "action cannot use this placement")
		}
		validateSemanticCondition(action.VisibleWhen, path+".visibleWhen", fields, issues)
	}
}

func validateSemanticCondition(condition *Condition, path string, fields map[string]CapabilityField, issues *[]Issue) {
	if condition == nil {
		return
	}
	if condition.All != nil {
		for index := range *condition.All {
			validateSemanticCondition(&(*condition.All)[index], fmt.Sprintf("%s.all[%d]", path, index), fields, issues)
		}
		return
	}
	if condition.Any != nil {
		for index := range *condition.Any {
			validateSemanticCondition(&(*condition.Any)[index], fmt.Sprintf("%s.any[%d]", path, index), fields, issues)
		}
		return
	}
	if condition.Not != nil {
		validateSemanticCondition(condition.Not, path+".not", fields, issues)
		return
	}
	if condition.Field != nil {
		if _, ok := fields[*condition.Field]; !ok {
			addIssue(issues, "unknown-condition-field", path+".field", "condition references an unknown field")
		}
	}
}

func ValidateCapability(capability *CapabilityDefinition) []Issue {
	issues := make([]Issue, 0)
	if capability == nil {
		return []Issue{issue("missing-capability", "$", "capability definition is required")}
	}
	validateIdentifier(capability.PageKey, "pageKey", &issues)
	if IsProtectedPageKey(capability.PageKey) {
		addIssue(&issues, "protected-page", "pageKey", "protected core page cannot be presentation-configurable")
	}
	if strings.TrimSpace(capability.DefinitionVersion) == "" {
		addIssue(&issues, "missing-definition-version", "definitionVersion", "definition version is required")
	} else if capability.DefinitionVersion != DefinitionVersionV1 && capability.DefinitionVersion != DefinitionVersionV2 {
		addIssue(&issues, "unsupported-definition-version", "definitionVersion", "definition version must be 1 or 2")
	}
	supportedDefinitionVersion := capability.DefinitionVersion == DefinitionVersionV1 || capability.DefinitionVersion == DefinitionVersionV2
	if !definitionHashPattern.MatchString(capability.DefinitionHash) {
		addIssue(&issues, "invalid-definition-hash", "definitionHash", "definition hash is invalid")
	} else if supportedDefinitionVersion {
		computed, err := ComputeDefinitionHash(capability)
		if err != nil {
			addIssue(&issues, "definition-hash-computation", "definitionHash", err.Error())
		} else if computed != capability.DefinitionHash {
			addIssue(&issues, "definition-hash-mismatch", "definitionHash", "definition hash does not match the canonical compatibility contract")
		}
	}
	validateCapabilityIDs(capability, &issues)
	if capabilityUsesLimitedTablePresentation(capability) {
		validateLimitedCompletePresentationConditions(&capability.DefaultPresentation, &issues)
	}
	validateCompletePresentation(capability, &issues)
	sortIssues(issues)
	return issues
}

func validateLimitedCompletePresentationConditions(presentation *CompletePresentation, issues *[]Issue) {
	collections := []struct {
		path   string
		fields []CompleteField
	}{
		{path: "defaultPresentation.list.columns", fields: presentation.List.Columns},
		{path: "defaultPresentation.search.fields", fields: presentation.Search.Fields},
		{path: "defaultPresentation.form.fields", fields: presentation.Form.Fields},
		{path: "defaultPresentation.detail.fields", fields: presentation.Detail.Fields},
	}
	for _, collection := range collections {
		for index, field := range collection.fields {
			if field.VisibleWhen != nil {
				addIssue(issues, "unsupported-limited-condition", fmt.Sprintf("%s[%d].visibleWhen", collection.path, index), "visibility conditions are not supported by limited table pages")
			}
		}
	}
	for index, action := range presentation.Actions {
		if action.VisibleWhen != nil {
			addIssue(issues, "unsupported-limited-condition", fmt.Sprintf("defaultPresentation.actions[%d].visibleWhen", index), "visibility conditions are not supported by limited table pages")
		}
	}
}

func validateCapabilityIDs(capability *CapabilityDefinition, issues *[]Issue) {
	componentIDs := make(map[string]struct{}, len(capability.Components))
	validateUniqueIdentifiers("components", len(capability.Components), func(index int) string { return capability.Components[index].ID }, componentIDs, issues)
	fieldIDs := make(map[string]struct{}, len(capability.Fields))
	if capability.DefinitionVersion == DefinitionVersionV2 {
		validateUniqueFieldIdentifiers("fields", len(capability.Fields), func(index int) string { return capability.Fields[index].ID }, fieldIDs, issues)
	} else {
		validateUniqueVersionOneFieldIdentifiers("fields", len(capability.Fields), func(index int) string { return capability.Fields[index].ID }, fieldIDs, issues)
	}
	dataSourceIDs := make(map[string]struct{}, len(capability.DataSources))
	validateUniqueIdentifiers("dataSources", len(capability.DataSources), func(index int) string { return capability.DataSources[index].ID }, dataSourceIDs, issues)
	actionIDs := make(map[string]struct{}, len(capability.Actions))
	validateUniqueIdentifiers("actions", len(capability.Actions), func(index int) string { return capability.Actions[index].ID }, actionIDs, issues)
	for index := range capability.Fields {
		field := &capability.Fields[index]
		validateLocalizedText(&field.Label, fmt.Sprintf("fields[%d].label", index), issues)
		if _, ok := allowedValueTypes[field.ValueType]; !ok {
			addIssue(issues, "invalid-field-value-type", fmt.Sprintf("fields[%d].valueType", index), "field value type is not supported")
		}
		if field.Required && field.Nullable {
			addIssue(issues, "conflicting-field-nullability", fmt.Sprintf("fields[%d]", index), "required and nullable cannot both be true")
		}
		if field.Validation.MinLength != nil && (*field.Validation.MinLength < 0 || (capability.DefinitionVersion == DefinitionVersionV2 && int64(*field.Validation.MinLength) > maxSafeJSONInteger)) {
			addIssue(issues, "invalid-field-min-length", fmt.Sprintf("fields[%d].validation.minLength", index), "minimum length cannot be negative")
		}
		if field.Validation.MaxLength != nil && (*field.Validation.MaxLength < 0 || (capability.DefinitionVersion == DefinitionVersionV2 && int64(*field.Validation.MaxLength) > maxSafeJSONInteger)) {
			addIssue(issues, "invalid-field-max-length", fmt.Sprintf("fields[%d].validation.maxLength", index), "maximum length cannot be negative")
		}
		if field.Validation.MinLength != nil && field.Validation.MaxLength != nil && *field.Validation.MinLength > *field.Validation.MaxLength {
			addIssue(issues, "invalid-field-length-range", fmt.Sprintf("fields[%d].validation", index), "minimum length cannot exceed maximum length")
		}
		if field.Validation.Pattern != "" {
			if _, err := regexp.Compile(field.Validation.Pattern); err != nil {
				addIssue(issues, "invalid-field-pattern", fmt.Sprintf("fields[%d].validation.pattern", index), "field pattern is invalid")
			}
			if capability.DefinitionVersion == DefinitionVersionV2 && !isPortableCapabilityPattern(field.Validation.Pattern) {
				addIssue(issues, "non-portable-field-pattern", fmt.Sprintf("fields[%d].validation.pattern", index), "field pattern must use the portable Go and ECMAScript subset")
			}
		}
		_, formatAllowed := allowedFieldFormats[field.Format]
		if capability.DefinitionVersion == DefinitionVersionV2 && !formatAllowed {
			addIssue(issues, "unsupported-field-format", fmt.Sprintf("fields[%d].format", index), "field format is not supported")
		} else if field.Format != "" && !formatAllowed {
			addIssue(issues, "unsupported-field-format", fmt.Sprintf("fields[%d].format", index), "field format is not supported")
		}
		validateCapabilityNumericFacts(field, index, issues)
		enumValues := make(map[string]struct{}, len(field.EnumValues))
		for valueIndex := range field.EnumValues {
			valuePath := fmt.Sprintf("fields[%d].enumValues[%d]", index, valueIndex)
			value := &field.EnumValues[valueIndex]
			if len(value.Value) < 1 || len(value.Value) > 120 || !enumValuePattern.MatchString(value.Value) {
				addIssue(issues, "invalid-enum-value", valuePath+".value", "enum value must be a stable lower-case token")
			}
			validateLocalizedText(&value.Label, valuePath+".label", issues)
			if _, duplicate := enumValues[value.Value]; duplicate {
				addIssue(issues, "duplicate-enum-value", valuePath+".value", "enum value is duplicated")
			}
			enumValues[value.Value] = struct{}{}
		}
		if capability.DefinitionVersion == DefinitionVersionV2 && field.ValueType == "enum" && len(field.EnumValues) == 0 {
			addIssue(issues, "missing-enum-values", fmt.Sprintf("fields[%d].enumValues", index), "enum field needs values")
		} else if field.ValueType != "enum" && len(field.EnumValues) > 0 {
			addIssue(issues, "unexpected-enum-values", fmt.Sprintf("fields[%d].enumValues", index), "enum values require enum value type")
		}
		if len(field.Surfaces) == 0 {
			addIssue(issues, "missing-field-surface", fmt.Sprintf("fields[%d].surfaces", index), "field needs at least one surface")
		}
		seenSurfaces := make(map[Surface]struct{}, len(field.Surfaces))
		for surfaceIndex, surface := range field.Surfaces {
			if !validSurface(surface) {
				addIssue(issues, "invalid-field-surface", fmt.Sprintf("fields[%d].surfaces[%d]", index, surfaceIndex), "surface is not supported")
			}
			if _, duplicate := seenSurfaces[surface]; duplicate {
				addIssue(issues, "duplicate-field-surface", fmt.Sprintf("fields[%d].surfaces[%d]", index, surfaceIndex), "surface is duplicated")
			}
			seenSurfaces[surface] = struct{}{}
		}
		if len(field.Components) == 0 {
			addIssue(issues, "missing-field-component", fmt.Sprintf("fields[%d].components", index), "field needs at least one component")
		}
		seenComponents := make(map[string]struct{}, len(field.Components))
		for componentIndex, component := range field.Components {
			if _, ok := componentIDs[component]; !ok {
				addIssue(issues, "unknown-field-component", fmt.Sprintf("fields[%d].components[%d]", index, componentIndex), "field references an unknown component")
			}
			if _, duplicate := seenComponents[component]; duplicate {
				addIssue(issues, "duplicate-field-component", fmt.Sprintf("fields[%d].components[%d]", index, componentIndex), "field component is duplicated")
			}
			seenComponents[component] = struct{}{}
		}
		if capability.DefinitionVersion == DefinitionVersionV2 {
			validateSurfaceComponents(field, index, componentIDs, issues)
		}
	}
	for index := range capability.DataSources {
		validatePermissions(capability.DataSources[index].RequiredPermissions, fmt.Sprintf("dataSources[%d].requiredPermissions", index), issues)
		if capability.DefinitionVersion == DefinitionVersionV2 {
			validateDataSourceLimits(&capability.DataSources[index], index, issues)
		}
	}
	for index := range capability.Actions {
		validatePermissions(capability.Actions[index].RequiredPermissions, fmt.Sprintf("actions[%d].requiredPermissions", index), issues)
		if len(capability.Actions[index].Placements) == 0 {
			addIssue(issues, "missing-action-placement", fmt.Sprintf("actions[%d].placements", index), "action needs at least one placement")
		}
		seen := make(map[ActionPlacement]struct{}, len(capability.Actions[index].Placements))
		for placementIndex, placement := range capability.Actions[index].Placements {
			if !validPlacement(placement) {
				addIssue(issues, "invalid-action-placement", fmt.Sprintf("actions[%d].placements[%d]", index, placementIndex), "action placement is not supported")
			}
			if _, duplicate := seen[placement]; duplicate {
				addIssue(issues, "duplicate-action-placement", fmt.Sprintf("actions[%d].placements[%d]", index, placementIndex), "action placement is duplicated")
			}
			seen[placement] = struct{}{}
		}
	}
}

func validateCompletePresentation(capability *CapabilityDefinition, issues *[]Issue) {
	defaultValue := &capability.DefaultPresentation
	validateLocalizedText(&defaultValue.Title, "defaultPresentation.title", issues)
	dataSource, hasDataSource := findDataSource(capability.DataSources, defaultValue.DataSource)
	if !hasDataSource {
		addIssue(issues, "unknown-data-source", "defaultPresentation.dataSource", "default presentation references an unknown data source")
	}
	fieldDefinitions := make(map[string]CapabilityField, len(capability.Fields))
	for _, field := range capability.Fields {
		fieldDefinitions[field.ID] = field
	}
	components := make(map[string]struct{}, len(capability.Components))
	for _, component := range capability.Components {
		components[component.ID] = struct{}{}
	}
	collections := []struct {
		surface Surface
		path    string
		fields  []CompleteField
	}{
		{SurfaceList, "defaultPresentation.list.columns", defaultValue.List.Columns},
		{SurfaceSearch, "defaultPresentation.search.fields", defaultValue.Search.Fields},
		{SurfaceForm, "defaultPresentation.form.fields", defaultValue.Form.Fields},
		{SurfaceDetail, "defaultPresentation.detail.fields", defaultValue.Detail.Fields},
	}
	for _, collection := range collections {
		validateCompleteFields(collection.fields, collection.surface, collection.path, fieldDefinitions, components, capability.DefinitionVersion == DefinitionVersionV2, issues)
	}
	for _, field := range capability.Fields {
		for _, surface := range field.Surfaces {
			if !completeCollectionHasField(defaultValue, surface, field.ID) {
				addIssue(issues, "missing-default-field", "defaultPresentation."+string(surface), "registered field is missing from its default surface")
			}
		}
	}
	visibleList := false
	for _, field := range defaultValue.List.Columns {
		visibleList = visibleList || !field.Hidden
	}
	if !visibleList {
		addIssue(issues, "empty-list-presentation", "defaultPresentation.list.columns", "at least one list column must be visible")
	}
	if _, ok := allowedDensity[defaultValue.List.Density]; !ok {
		addIssue(issues, "invalid-density", "defaultPresentation.list.density", "default density is invalid")
	}
	if defaultValue.List.PageSize < 1 || defaultValue.List.PageSize > 200 {
		addIssue(issues, "invalid-page-size", "defaultPresentation.list.pageSize", "default page size must be 1 to 200")
	}
	if hasDataSource && capability.DefinitionVersion == DefinitionVersionV2 {
		validateCapabilityPageSize(defaultValue.List.PageSize, dataSource, "defaultPresentation.list.pageSize", issues)
		if len(defaultValue.List.DefaultSort) > dataSource.MaxSortFields {
			addIssue(issues, "too-many-data-source-sort-fields", "defaultPresentation.list.defaultSort", "default sort exceeds the compiled data source limit")
		}
	}
	validateSorts(defaultValue.List.DefaultSort, "defaultPresentation.list.defaultSort", issues)
	validateSemanticSort(defaultValue.List.DefaultSort, "defaultPresentation.list.defaultSort", fieldDefinitions, issues)
	if defaultValue.Form.Columns < 1 || defaultValue.Form.Columns > 4 {
		addIssue(issues, "invalid-layout-columns", "defaultPresentation.form.columns", "default form columns must be 1 to 4")
	}
	if defaultValue.Detail.Columns < 1 || defaultValue.Detail.Columns > 4 {
		addIssue(issues, "invalid-layout-columns", "defaultPresentation.detail.columns", "default detail columns must be 1 to 4")
	}
	actionDefinitions := make(map[string]CapabilityAction, len(capability.Actions))
	for _, action := range capability.Actions {
		actionDefinitions[action.ID] = action
	}
	seenActions := make(map[string]struct{}, len(defaultValue.Actions))
	for index := range defaultValue.Actions {
		path := fmt.Sprintf("defaultPresentation.actions[%d]", index)
		action := &defaultValue.Actions[index]
		if _, duplicate := seenActions[action.Action]; duplicate {
			addIssue(issues, "duplicate-action", path+".action", "default action is duplicated")
		}
		seenActions[action.Action] = struct{}{}
		definition, ok := actionDefinitions[action.Action]
		if !ok {
			addIssue(issues, "unknown-action", path+".action", "default presentation references an unknown action")
		} else if !containsPlacement(definition.Placements, action.Placement) {
			addIssue(issues, "unsupported-action-placement", path+".placement", "default action placement is unsupported")
		}
		validateLocalizedText(action.Label, path+".label", issues)
		validateLocalizedText(action.Confirm, path+".confirm", issues)
		validateConditionShape(action.VisibleWhen, path+".visibleWhen", issues, 0)
		validateSemanticCondition(action.VisibleWhen, path+".visibleWhen", fieldDefinitions, issues)
	}
}

func validateCompleteFields(fields []CompleteField, surface Surface, path string, definitions map[string]CapabilityField, components map[string]struct{}, strictSurfaceComponents bool, issues *[]Issue) {
	seen := make(map[string]struct{}, len(fields))
	for index := range fields {
		currentPath := fmt.Sprintf("%s[%d]", path, index)
		field := &fields[index]
		if _, duplicate := seen[field.Field]; duplicate {
			addIssue(issues, "duplicate-field", currentPath+".field", "default field is duplicated")
		}
		seen[field.Field] = struct{}{}
		definition, ok := definitions[field.Field]
		if !ok {
			addIssue(issues, "unknown-field", currentPath+".field", "default presentation references an unknown field")
			continue
		}
		if !containsSurface(definition.Surfaces, surface) {
			addIssue(issues, "unsupported-field-surface", currentPath+".field", "default field is not registered for this surface")
		}
		if _, ok = components[field.Component]; !ok {
			addIssue(issues, "unknown-component", currentPath+".component", "default presentation references an unknown component")
		} else if !capabilityFieldSupportsComponent(definition, surface, field.Component, strictSurfaceComponents) {
			addIssue(issues, "unsupported-field-component", currentPath+".component", "default component is not allowed for the field")
		}
		if surface == SurfaceForm && definition.Required && field.Hidden {
			addIssue(issues, "required-form-field-hidden", currentPath+".hidden", "required form field cannot be hidden")
		}
		if surface == SurfaceForm && definition.Required && field.VisibleWhen != nil {
			addIssue(issues, "required-form-field-conditional", currentPath+".visibleWhen", "required form field cannot be conditionally hidden")
		}
		if field.Order < 0 || field.Order > 10000 {
			addIssue(issues, "invalid-field-order", currentPath+".order", "default field order must be 0 to 10000")
		}
		if field.Width != nil && (*field.Width < 60 || *field.Width > 1200) {
			addIssue(issues, "invalid-field-width", currentPath+".width", "default field width must be 60 to 1200")
		}
		if field.Span != nil && (*field.Span < 1 || *field.Span > 24) {
			addIssue(issues, "invalid-field-span", currentPath+".span", "default field span must be 1 to 24")
		}
		validateLocalizedText(field.Label, currentPath+".label", issues)
		validateLocalizedText(field.Placeholder, currentPath+".placeholder", issues)
		validateLocalizedText(field.Help, currentPath+".help", issues)
		validateConditionShape(field.VisibleWhen, currentPath+".visibleWhen", issues, 0)
		validateSemanticCondition(field.VisibleWhen, currentPath+".visibleWhen", definitions, issues)
	}
}

func ComputeDefinitionHash(capability *CapabilityDefinition) (string, error) {
	if capability == nil {
		return "", errors.New("capability is required")
	}
	var compatibility any
	switch capability.DefinitionVersion {
	case DefinitionVersionV1:
		type legacyField struct {
			ID         string        `json:"id"`
			Label      LocalizedText `json:"label"`
			ValueType  string        `json:"valueType"`
			Required   bool          `json:"required"`
			Sortable   bool          `json:"sortable"`
			Filterable bool          `json:"filterable"`
			Surfaces   []Surface     `json:"surfaces"`
			Components []string      `json:"components"`
		}
		type legacyDataSource struct {
			ID                  string   `json:"id"`
			RequiredPermissions []string `json:"requiredPermissions"`
		}
		type legacyAction struct {
			ID                  string            `json:"id"`
			RequiredPermissions []string          `json:"requiredPermissions"`
			Placements          []ActionPlacement `json:"placements"`
			Destructive         bool              `json:"destructive,omitempty"`
		}
		var fields []legacyField
		if capability.Fields != nil {
			fields = make([]legacyField, 0, len(capability.Fields))
			for _, field := range capability.Fields {
				fields = append(fields, legacyField{
					ID: field.ID, Label: field.Label, ValueType: field.ValueType,
					Required: field.Required, Sortable: field.Sortable, Filterable: field.Filterable,
					Surfaces: field.Surfaces, Components: field.Components,
				})
			}
		}
		var dataSources []legacyDataSource
		if capability.DataSources != nil {
			dataSources = make([]legacyDataSource, 0, len(capability.DataSources))
			for _, dataSource := range capability.DataSources {
				dataSources = append(dataSources, legacyDataSource{
					ID: dataSource.ID, RequiredPermissions: dataSource.RequiredPermissions,
				})
			}
		}
		var actions []legacyAction
		if capability.Actions != nil {
			actions = make([]legacyAction, 0, len(capability.Actions))
			for _, action := range capability.Actions {
				actions = append(actions, legacyAction{
					ID: action.ID, RequiredPermissions: action.RequiredPermissions,
					Placements: action.Placements, Destructive: action.Destructive,
				})
			}
		}
		compatibility = struct {
			PageKey           string                `json:"pageKey"`
			DefinitionVersion string                `json:"definitionVersion"`
			Components        []CapabilityComponent `json:"components"`
			Fields            []legacyField         `json:"fields"`
			DataSources       []legacyDataSource    `json:"dataSources"`
			Actions           []legacyAction        `json:"actions"`
		}{
			PageKey: capability.PageKey, DefinitionVersion: capability.DefinitionVersion,
			Components: capability.Components, Fields: fields,
			DataSources: dataSources, Actions: actions,
		}
	case DefinitionVersionV2:
		compatibility = struct {
			PageKey             string                 `json:"pageKey"`
			DefinitionVersion   string                 `json:"definitionVersion"`
			Components          []CapabilityComponent  `json:"components"`
			Fields              []CapabilityField      `json:"fields"`
			DataSources         []CapabilityDataSource `json:"dataSources"`
			Actions             []CapabilityAction     `json:"actions"`
			DefaultPresentation CompletePresentation   `json:"defaultPresentation"`
		}{
			PageKey: capability.PageKey, DefinitionVersion: capability.DefinitionVersion,
			Components: capability.Components, Fields: capability.Fields,
			DataSources: capability.DataSources, Actions: capability.Actions,
			DefaultPresentation: capability.DefaultPresentation,
		}
	default:
		return "", fmt.Errorf("unsupported definition version %q", capability.DefinitionVersion)
	}
	canonicalizer := canonicalJSON
	if capability.DefinitionVersion == DefinitionVersionV2 {
		canonicalizer = canonicalJSONV2
	}
	canonical, err := canonicalizer(compatibility)
	if err != nil {
		return "", err
	}
	return sha256Digest(canonical), nil
}

func IsProtectedPageKey(pageKey string) bool {
	namespace, _, _ := strings.Cut(pageKey, ".")
	switch namespace {
	case "account", "auth", "authentication", "authorization", "app-config", "application-config",
		"config", "configuration", "login", "presentation", "presentation-config", "recovery", "release", "system":
		return true
	}
	return false
}

func capabilityFieldSupportsComponent(field CapabilityField, surface Surface, component string, strict bool) bool {
	if !strict {
		return containsString(field.Components, component)
	}
	for _, mapping := range field.SurfaceComponents {
		if mapping.Surface == surface {
			return containsString(mapping.Components, component)
		}
	}
	return false
}

func isPortableCapabilityPattern(value string) bool {
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
					!isCapabilityPatternHex(value[index+1]) ||
					!isCapabilityPatternHex(value[index+2]) {
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

func isCapabilityPatternHex(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

func validateIdentifier(value, path string, issues *[]Issue) {
	if len(value) < 2 || len(value) > 120 || !identifierPattern.MatchString(value) {
		addIssue(issues, "invalid-identifier", path, "value must be a stable lowercase identifier")
	}
}

func validateUniqueIdentifiers(path string, length int, value func(int) string, seen map[string]struct{}, issues *[]Issue) {
	for index := 0; index < length; index++ {
		current := value(index)
		validateIdentifier(current, fmt.Sprintf("%s[%d].id", path, index), issues)
		if _, duplicate := seen[current]; duplicate {
			addIssue(issues, "duplicate-capability-id", fmt.Sprintf("%s[%d].id", path, index), "capability identifier is duplicated")
		}
		seen[current] = struct{}{}
	}
}

func validateUniqueFieldIdentifiers(path string, length int, value func(int) string, seen map[string]struct{}, issues *[]Issue) {
	for index := 0; index < length; index++ {
		current := value(index)
		currentPath := fmt.Sprintf("%s[%d].id", path, index)
		if len(current) < 1 || len(current) > 120 || !fieldIdentifierPattern.MatchString(current) {
			addIssue(issues, "invalid-field-identifier", currentPath, "field id must be a stable lower-camel identifier")
		}
		if _, duplicate := seen[current]; duplicate {
			addIssue(issues, "duplicate-capability-id", currentPath, "capability identifier is duplicated")
		}
		seen[current] = struct{}{}
	}
}

func validateUniqueVersionOneFieldIdentifiers(path string, length int, value func(int) string, seen map[string]struct{}, issues *[]Issue) {
	for index := 0; index < length; index++ {
		current := value(index)
		currentPath := fmt.Sprintf("%s[%d].id", path, index)
		if len(current) < 2 || len(current) > 120 || (!identifierPattern.MatchString(current) && !fieldIdentifierPattern.MatchString(current)) {
			addIssue(issues, "invalid-field-identifier", currentPath, "field id must be a stable version 1 identifier")
		}
		if _, duplicate := seen[current]; duplicate {
			addIssue(issues, "duplicate-capability-id", currentPath, "capability identifier is duplicated")
		}
		seen[current] = struct{}{}
	}
}

func validatePermissions(permissions []string, path string, issues *[]Issue) {
	if len(permissions) == 0 {
		addIssue(issues, "missing-permission", path, "at least one trusted permission is required")
	}
	seen := make(map[string]struct{}, len(permissions))
	for index, permission := range permissions {
		if strings.TrimSpace(permission) == "" || len(permission) > 255 {
			addIssue(issues, "invalid-permission", fmt.Sprintf("%s[%d]", path, index), "trusted permission must be non-empty and bounded")
		}
		if _, duplicate := seen[permission]; duplicate {
			addIssue(issues, "duplicate-permission", fmt.Sprintf("%s[%d]", path, index), "trusted permission is duplicated")
		}
		seen[permission] = struct{}{}
	}
}

func validateDataSourceLimits(dataSource *CapabilityDataSource, index int, issues *[]Issue) {
	path := fmt.Sprintf("dataSources[%d]", index)
	if dataSource.MaxPageSize < 1 || dataSource.MaxPageSize > 200 {
		addIssue(issues, "invalid-max-page-size", path+".maxPageSize", "maximum page size must be 1 to 200")
	}
	if len(dataSource.PageSizeOptions) == 0 {
		addIssue(issues, "missing-page-size-options", path+".pageSizeOptions", "version 2 data source needs page size options")
	}
	previous := 0
	seen := make(map[int]struct{}, len(dataSource.PageSizeOptions))
	for optionIndex, option := range dataSource.PageSizeOptions {
		optionPath := fmt.Sprintf("%s.pageSizeOptions[%d]", path, optionIndex)
		if option < 1 || option > dataSource.MaxPageSize {
			addIssue(issues, "invalid-page-size-option", optionPath, "page size option must be within the data source maximum")
		}
		if _, duplicate := seen[option]; duplicate {
			addIssue(issues, "duplicate-page-size-option", optionPath, "page size option is duplicated")
		}
		if optionIndex > 0 && option <= previous {
			addIssue(issues, "unsorted-page-size-options", optionPath, "page size options must be strictly increasing")
		}
		seen[option] = struct{}{}
		previous = option
	}
	if dataSource.MaxSortFields < 0 || dataSource.MaxSortFields > 3 {
		addIssue(issues, "invalid-max-sort-fields", path+".maxSortFields", "maximum sort fields must be 0 to 3")
	}
}

func validateCapabilityNumericFacts(field *CapabilityField, index int, issues *[]Issue) {
	path := fmt.Sprintf("fields[%d].validation", index)
	parseBound := func(value *string, valuePath string) (float64, bool) {
		if value == nil {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(*value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			addIssue(issues, "invalid-field-number-bound", valuePath, "numeric bound must be a finite decimal string")
			return 0, false
		}
		return parsed, true
	}
	minimum, hasMinimum := parseBound(field.Validation.Minimum, path+".minimum")
	maximum, hasMaximum := parseBound(field.Validation.Maximum, path+".maximum")
	if hasMinimum && hasMaximum && minimum > maximum {
		addIssue(issues, "invalid-field-number-range", path, "minimum cannot exceed maximum")
	}
	if field.Validation.Precision != nil && (*field.Validation.Precision < 1 || *field.Validation.Precision > 38) {
		addIssue(issues, "invalid-field-precision", path+".precision", "precision must be 1 to 38")
	}
	if field.Validation.Scale != nil {
		if *field.Validation.Scale < 0 || *field.Validation.Scale > 38 {
			addIssue(issues, "invalid-field-scale", path+".scale", "scale must be 0 to 38")
		}
		if field.Validation.Precision != nil && *field.Validation.Scale > *field.Validation.Precision {
			addIssue(issues, "invalid-field-scale", path+".scale", "scale cannot exceed precision")
		}
	}
}

func validateSurfaceComponents(field *CapabilityField, fieldIndex int, componentIDs map[string]struct{}, issues *[]Issue) {
	path := fmt.Sprintf("fields[%d].surfaceComponents", fieldIndex)
	if len(field.SurfaceComponents) == 0 {
		addIssue(issues, "missing-surface-components", path, "version 2 field needs component choices for every surface")
		return
	}
	declaredSurfaces := make(map[Surface]struct{}, len(field.Surfaces))
	for _, surface := range field.Surfaces {
		declaredSurfaces[surface] = struct{}{}
	}
	declaredComponents := make(map[string]struct{}, len(field.Components))
	for _, component := range field.Components {
		declaredComponents[component] = struct{}{}
	}
	seenSurfaces := make(map[Surface]struct{}, len(field.SurfaceComponents))
	usedComponents := make(map[string]struct{}, len(field.Components))
	for surfaceIndex, surfaceComponents := range field.SurfaceComponents {
		itemPath := fmt.Sprintf("%s[%d]", path, surfaceIndex)
		if !validSurface(surfaceComponents.Surface) {
			addIssue(issues, "invalid-field-surface", itemPath+".surface", "surface is not supported")
		}
		if _, ok := declaredSurfaces[surfaceComponents.Surface]; !ok {
			addIssue(issues, "unexpected-surface-components", itemPath+".surface", "surface components reference an undeclared field surface")
		}
		if _, duplicate := seenSurfaces[surfaceComponents.Surface]; duplicate {
			addIssue(issues, "duplicate-surface-components", itemPath+".surface", "surface component mapping is duplicated")
		}
		seenSurfaces[surfaceComponents.Surface] = struct{}{}
		if len(surfaceComponents.Components) == 0 {
			addIssue(issues, "missing-surface-component", itemPath+".components", "surface needs at least one component")
		}
		seenComponents := make(map[string]struct{}, len(surfaceComponents.Components))
		for componentIndex, component := range surfaceComponents.Components {
			componentPath := fmt.Sprintf("%s.components[%d]", itemPath, componentIndex)
			if _, ok := componentIDs[component]; !ok {
				addIssue(issues, "unknown-field-component", componentPath, "surface references an unknown component")
			}
			if _, ok := declaredComponents[component]; !ok {
				addIssue(issues, "surface-component-mismatch", componentPath, "surface component is missing from the field component inventory")
			}
			if _, duplicate := seenComponents[component]; duplicate {
				addIssue(issues, "duplicate-field-component", componentPath, "surface component is duplicated")
			}
			seenComponents[component] = struct{}{}
			usedComponents[component] = struct{}{}
		}
	}
	for surface := range declaredSurfaces {
		if _, ok := seenSurfaces[surface]; !ok {
			addIssue(issues, "missing-surface-components", path, "surface component mapping is missing for "+string(surface))
		}
	}
	for component := range declaredComponents {
		if _, ok := usedComponents[component]; !ok {
			addIssue(issues, "surface-component-mismatch", path, "field component is not available on any surface: "+component)
		}
	}
}

func completeCollectionHasField(value *CompletePresentation, surface Surface, field string) bool {
	var fields []CompleteField
	switch surface {
	case SurfaceList:
		fields = value.List.Columns
	case SurfaceSearch:
		fields = value.Search.Fields
	case SurfaceForm:
		fields = value.Form.Fields
	case SurfaceDetail:
		fields = value.Detail.Fields
	}
	for _, current := range fields {
		if current.Field == field {
			return true
		}
	}
	return false
}

func containsDataSource(values []CapabilityDataSource, id string) bool {
	_, ok := findDataSource(values, id)
	return ok
}

func findDataSource(values []CapabilityDataSource, id string) (CapabilityDataSource, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return CapabilityDataSource{}, false
}

func validateCapabilityPageSize(value int, dataSource CapabilityDataSource, path string, issues *[]Issue) {
	if dataSource.MaxPageSize > 0 && value > dataSource.MaxPageSize {
		addIssue(issues, "page-size-exceeds-data-source-limit", path, "page size exceeds the compiled data source maximum")
	}
	if len(dataSource.PageSizeOptions) > 0 {
		allowed := false
		for _, option := range dataSource.PageSizeOptions {
			if value == option {
				allowed = true
				break
			}
		}
		if !allowed {
			addIssue(issues, "unsupported-page-size", path, "page size is not allowed by the compiled data source")
		}
	}
}

func containsSurface(values []Surface, expected Surface) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsPlacement(values []ActionPlacement, expected ActionPlacement) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validSurface(value Surface) bool {
	return value == SurfaceList || value == SurfaceSearch || value == SurfaceForm || value == SurfaceDetail
}

func validPlacement(value ActionPlacement) bool {
	return value == PlacementToolbar || value == PlacementRow || value == PlacementForm || value == PlacementDetail
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func issue(code, path, message string) Issue {
	return Issue{Code: code, Path: path, Message: message}
}

func addIssue(issues *[]Issue, code, path, message string) {
	*issues = append(*issues, issue(code, path, message))
}

func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(left, right int) bool {
		if issues[left].Path != issues[right].Path {
			return issues[left].Path < issues[right].Path
		}
		if issues[left].Code != issues[right].Code {
			return issues[left].Code < issues[right].Code
		}
		return issues[left].Message < issues[right].Message
	})
}

// Keep imports used when Go versions change decoder EOF behavior.
var _ = io.EOF
