package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	identifierPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
	definitionHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	allowedValueTypes     = stringSet("string", "number", "boolean", "enum", "date", "date-time")
	allowedDensity        = stringSet("compact", "middle", "large")
	allowedDirections     = stringSet("asc", "desc")
	allowedOperators      = stringSet("eq", "neq", "in", "not-in", "exists", "not-exists", "gt", "gte", "lt", "lte")
)

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
		} else {
			validateIdentifier(*scope.Subject, "metadata.scope.subject", issues)
		}
	case ScopeUser:
		if scope.Subject == nil || utf8.RuneCountInString(*scope.Subject) < 1 || utf8.RuneCountInString(*scope.Subject) > 160 {
			addIssue(issues, "invalid-scope-subject", "metadata.scope.subject", "user scope subject must contain 1 to 160 characters")
		}
	default:
		addIssue(issues, "invalid-scope-kind", "metadata.scope.kind", "scope kind must be application, role, or user")
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
	if profile.Spec.List != nil {
		if profile.Spec.List.Columns != nil {
			validateSemanticFields(*profile.Spec.List.Columns, SurfaceList, "spec.list.columns", fieldDefinitions, componentIDs, &issues)
		}
		if profile.Spec.List.DefaultSort != nil {
			validateSemanticSort(*profile.Spec.List.DefaultSort, "spec.list.defaultSort", fieldDefinitions, &issues)
		}
	}
	if profile.Spec.Search != nil && profile.Spec.Search.Fields != nil {
		validateSemanticFields(*profile.Spec.Search.Fields, SurfaceSearch, "spec.search.fields", fieldDefinitions, componentIDs, &issues)
	}
	if profile.Spec.Form != nil && profile.Spec.Form.Fields != nil {
		validateSemanticFields(*profile.Spec.Form.Fields, SurfaceForm, "spec.form.fields", fieldDefinitions, componentIDs, &issues)
	}
	if profile.Spec.Detail != nil && profile.Spec.Detail.Fields != nil {
		validateSemanticFields(*profile.Spec.Detail.Fields, SurfaceDetail, "spec.detail.fields", fieldDefinitions, componentIDs, &issues)
	}
	if profile.Spec.Actions != nil {
		validateSemanticActions(*profile.Spec.Actions, capability.Actions, fieldDefinitions, &issues)
	}
	sortIssues(issues)
	return issues
}

func validateSemanticFields(fields []FieldPatch, surface Surface, path string, definitions map[string]CapabilityField, components map[string]struct{}, issues *[]Issue) {
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
			} else if !containsString(definition.Components, *field.Component) {
				addIssue(issues, "unsupported-field-component", currentPath+".component", "component is not allowed for this field")
			}
		}
		if surface == SurfaceForm && definition.Required && field.Hidden != nil && *field.Hidden {
			addIssue(issues, "required-form-field-hidden", currentPath+".hidden", "required form field cannot be hidden")
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
	}
	if !definitionHashPattern.MatchString(capability.DefinitionHash) {
		addIssue(&issues, "invalid-definition-hash", "definitionHash", "definition hash is invalid")
	} else if computed, err := ComputeDefinitionHash(capability); err != nil {
		addIssue(&issues, "definition-hash-computation", "definitionHash", err.Error())
	} else if computed != capability.DefinitionHash {
		addIssue(&issues, "definition-hash-mismatch", "definitionHash", "definition hash does not match the canonical compatibility contract")
	}
	validateCapabilityIDs(capability, &issues)
	validateCompletePresentation(capability, &issues)
	sortIssues(issues)
	return issues
}

func validateCapabilityIDs(capability *CapabilityDefinition, issues *[]Issue) {
	componentIDs := make(map[string]struct{}, len(capability.Components))
	validateUniqueIdentifiers("components", len(capability.Components), func(index int) string { return capability.Components[index].ID }, componentIDs, issues)
	fieldIDs := make(map[string]struct{}, len(capability.Fields))
	validateUniqueIdentifiers("fields", len(capability.Fields), func(index int) string { return capability.Fields[index].ID }, fieldIDs, issues)
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
	}
	for index := range capability.DataSources {
		validatePermissions(capability.DataSources[index].RequiredPermissions, fmt.Sprintf("dataSources[%d].requiredPermissions", index), issues)
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
	if !containsDataSource(capability.DataSources, defaultValue.DataSource) {
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
		validateCompleteFields(collection.fields, collection.surface, collection.path, fieldDefinitions, components, issues)
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

func validateCompleteFields(fields []CompleteField, surface Surface, path string, definitions map[string]CapabilityField, components map[string]struct{}, issues *[]Issue) {
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
		} else if !containsString(definition.Components, field.Component) {
			addIssue(issues, "unsupported-field-component", currentPath+".component", "default component is not allowed for the field")
		}
		if surface == SurfaceForm && definition.Required && field.Hidden {
			addIssue(issues, "required-form-field-hidden", currentPath+".hidden", "required form field cannot be hidden")
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
	compatibility := struct {
		PageKey           string                 `json:"pageKey"`
		DefinitionVersion string                 `json:"definitionVersion"`
		Components        []CapabilityComponent  `json:"components"`
		Fields            []CapabilityField      `json:"fields"`
		DataSources       []CapabilityDataSource `json:"dataSources"`
		Actions           []CapabilityAction     `json:"actions"`
	}{
		PageKey:           capability.PageKey,
		DefinitionVersion: capability.DefinitionVersion,
		Components:        capability.Components,
		Fields:            capability.Fields,
		DataSources:       capability.DataSources,
		Actions:           capability.Actions,
	}
	canonical, err := canonicalJSON(compatibility)
	if err != nil {
		return "", err
	}
	return sha256Digest(canonical), nil
}

func IsProtectedPageKey(pageKey string) bool {
	prefixes := []string{
		"auth.", "authentication.", "authorization.", "app-config.", "application-config.",
		"release.", "recovery.", "presentation-config.", "presentation.governance",
	}
	for _, prefix := range prefixes {
		if pageKey == strings.TrimSuffix(prefix, ".") || strings.HasPrefix(pageKey, prefix) {
			return true
		}
	}
	return false
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
	for _, value := range values {
		if value.ID == id {
			return true
		}
	}
	return false
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
