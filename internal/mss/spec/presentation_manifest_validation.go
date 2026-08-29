package spec

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const normalizedPresentationMaxSafeInteger = int64(1<<53 - 1)

var (
	normalizedPresentationIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
	normalizedPresentationDecimalPattern    = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
	normalizedPresentationEnumValuePattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// ValidateNormalizedPresentationManifest validates the final, fully-derived
// version 2 compatibility object. This is deliberately independent of the
// AdminModule JSON Schema: ParseModule does not execute that schema, while all
// generated projections and their definition hash trust this object.
func ValidateNormalizedPresentationManifest(manifest *NormalizedPresentationManifest) []Issue {
	issues := make([]Issue, 0)
	add := func(path, code, message string) {
		issues = append(issues, Issue{Path: path, Code: code, Message: message})
	}
	if manifest == nil {
		return []Issue{{Path: "normalizedPresentation", Code: "manifest-required", Message: "normalized presentation manifest is required"}}
	}

	if !moduleNamePattern.MatchString(manifest.ModuleName) || len(manifest.ModuleName) > 120 {
		add("normalizedPresentation.moduleName", "invalid-module-name", "module name must be lower-case kebab-case and not exceed 120 bytes")
	}
	if !presentationPageKeyPattern.MatchString(manifest.PageKey) || len(manifest.PageKey) > 120 {
		add("normalizedPresentation.pageKey", "invalid-page-key", "page key must be a dotted lower-case identifier not exceeding 120 bytes")
	} else if IsProtectedPresentationPageKey(manifest.PageKey) {
		add("normalizedPresentation.pageKey", "protected-page-key", "protected core page namespaces cannot be presentation-configurable")
	}
	if manifest.DefinitionVersion != PresentationDefinitionVersion {
		add("normalizedPresentation.definitionVersion", "unsupported-definition-version", "normalized presentation definitions must use version "+PresentationDefinitionVersion)
	}

	validateNormalizedCollection("normalizedPresentation.components", manifest.Components == nil, len(manifest.Components), true, add)
	validateNormalizedCollection("normalizedPresentation.fields", manifest.Fields == nil, len(manifest.Fields), true, add)
	validateNormalizedCollection("normalizedPresentation.dataSources", manifest.DataSources == nil, len(manifest.DataSources), true, add)
	validateNormalizedCollection("normalizedPresentation.actions", manifest.Actions == nil, len(manifest.Actions), false, add)

	componentIDs := make(map[string]bool, len(manifest.Components))
	for index, component := range manifest.Components {
		path := fmt.Sprintf("normalizedPresentation.components[%d].id", index)
		validateNormalizedIdentifier(path, component.ID, add)
		validateNormalizedUnique(path, component.ID, componentIDs, "duplicate-capability-id", "capability identifier must be unique", add)
	}

	fieldIDs := make(map[string]bool, len(manifest.Fields))
	fieldByID := make(map[string]NormalizedPresentationField, len(manifest.Fields))
	for index := range manifest.Fields {
		field := &manifest.Fields[index]
		path := fmt.Sprintf("normalizedPresentation.fields[%d]", index)
		if len(field.ID) < 1 || len(field.ID) > 120 || !camelNamePattern.MatchString(field.ID) {
			add(path+".id", "invalid-field-identifier", "field id must be a lower-camel identifier not exceeding 120 bytes")
		}
		validateNormalizedUnique(path+".id", field.ID, fieldIDs, "duplicate-capability-id", "capability identifier must be unique", add)
		fieldByID[field.ID] = *field
		validateNormalizedLocalizedText(path+".label", field.Label, add)
		validateNormalizedFieldFacts(field, path, componentIDs, add)
	}

	dataSourceIDs := make(map[string]bool, len(manifest.DataSources))
	dataSourceByID := make(map[string]NormalizedPresentationDataSource, len(manifest.DataSources))
	for index := range manifest.DataSources {
		dataSource := &manifest.DataSources[index]
		path := fmt.Sprintf("normalizedPresentation.dataSources[%d]", index)
		validateNormalizedIdentifier(path+".id", dataSource.ID, add)
		validateNormalizedUnique(path+".id", dataSource.ID, dataSourceIDs, "duplicate-capability-id", "capability identifier must be unique", add)
		dataSourceByID[dataSource.ID] = *dataSource
		validateNormalizedPermissions(path+".requiredPermissions", dataSource.RequiredPermissions, add)
		validateNormalizedDataSourceLimits(dataSource, path, add)
	}

	actionIDs := make(map[string]bool, len(manifest.Actions))
	actionByID := make(map[string]NormalizedPresentationAction, len(manifest.Actions))
	for index := range manifest.Actions {
		action := &manifest.Actions[index]
		path := fmt.Sprintf("normalizedPresentation.actions[%d]", index)
		validateNormalizedIdentifier(path+".id", action.ID, add)
		validateNormalizedUnique(path+".id", action.ID, actionIDs, "duplicate-capability-id", "capability identifier must be unique", add)
		actionByID[action.ID] = *action
		validateNormalizedPermissions(path+".requiredPermissions", action.RequiredPermissions, add)
		validateNormalizedCollection(path+".placements", action.Placements == nil, len(action.Placements), true, add)
		seenPlacements := map[string]bool{}
		for placementIndex, placement := range action.Placements {
			placementPath := fmt.Sprintf("%s.placements[%d]", path, placementIndex)
			if !contains([]string{"toolbar", "row", "form", "detail"}, placement) {
				add(placementPath, "invalid-action-placement", "action placement is not supported")
			}
			validateNormalizedUnique(placementPath, placement, seenPlacements, "duplicate-action-placement", "action placement must be unique", add)
		}
	}

	validateNormalizedCompletePresentation(&manifest.DefaultPresentation, fieldByID, componentIDs, dataSourceByID, actionByID, add)

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
	return issues
}

func validateNormalizedFieldFacts(
	field *NormalizedPresentationField,
	path string,
	componentIDs map[string]bool,
	add func(string, string, string),
) {
	if !contains([]string{"string", "integer", "number", "boolean", "enum", "date", "date-time", "json"}, field.ValueType) {
		add(path+".valueType", "invalid-field-value-type", "field value type is not supported")
	}
	if !contains([]string{"plain", "email", "identifier", "date", "date-time"}, field.Format) {
		add(path+".format", "unsupported-field-format", "field format is not supported")
	}
	if field.Required && field.Nullable {
		add(path, "conflicting-field-nullability", "required and nullable cannot both be true")
	}

	validateNormalizedValidation(&field.Validation, path+".validation", add)
	validateNormalizedCollection(path+".enumValues", field.EnumValues == nil, len(field.EnumValues), field.ValueType == "enum", add)
	seenEnumValues := map[string]bool{}
	for index, enumValue := range field.EnumValues {
		enumPath := fmt.Sprintf("%s.enumValues[%d]", path, index)
		if len(enumValue.Value) < 1 || len(enumValue.Value) > 120 || !normalizedPresentationEnumValuePattern.MatchString(enumValue.Value) {
			add(enumPath+".value", "invalid-enum-value", "enum value must be a lower-case token not exceeding 120 bytes")
		}
		validateNormalizedUnique(enumPath+".value", enumValue.Value, seenEnumValues, "duplicate-enum-value", "enum value must be unique", add)
		validateNormalizedLocalizedText(enumPath+".label", enumValue.Label, add)
	}
	if field.ValueType != "enum" && len(field.EnumValues) > 0 {
		add(path+".enumValues", "unexpected-enum-values", "enum values require enum value type")
	}

	validateNormalizedCollection(path+".surfaces", field.Surfaces == nil, len(field.Surfaces), true, add)
	seenSurfaces := map[string]bool{}
	for index, surface := range field.Surfaces {
		surfacePath := fmt.Sprintf("%s.surfaces[%d]", path, index)
		if !contains([]string{"list", "search", "form", "detail"}, surface) {
			add(surfacePath, "invalid-field-surface", "field surface is not supported")
		}
		validateNormalizedUnique(surfacePath, surface, seenSurfaces, "duplicate-field-surface", "field surface must be unique", add)
	}

	validateNormalizedCollection(path+".components", field.Components == nil, len(field.Components), true, add)
	seenComponents := map[string]bool{}
	for index, component := range field.Components {
		componentPath := fmt.Sprintf("%s.components[%d]", path, index)
		validateNormalizedIdentifier(componentPath, component, add)
		if !componentIDs[component] {
			add(componentPath, "unknown-field-component", "field references a component outside the capability inventory")
		}
		validateNormalizedUnique(componentPath, component, seenComponents, "duplicate-field-component", "field component must be unique", add)
	}

	validateNormalizedCollection(path+".surfaceComponents", field.SurfaceComponents == nil, len(field.SurfaceComponents), true, add)
	seenMappedSurfaces := map[string]bool{}
	usedComponents := map[string]bool{}
	for index, mapping := range field.SurfaceComponents {
		mappingPath := fmt.Sprintf("%s.surfaceComponents[%d]", path, index)
		if !contains([]string{"list", "search", "form", "detail"}, mapping.Surface) {
			add(mappingPath+".surface", "invalid-field-surface", "surface component mapping uses an unsupported surface")
		}
		if !seenSurfaces[mapping.Surface] {
			add(mappingPath+".surface", "unexpected-surface-components", "surface component mapping references an undeclared field surface")
		}
		validateNormalizedUnique(mappingPath+".surface", mapping.Surface, seenMappedSurfaces, "duplicate-surface-components", "surface component mapping must be unique", add)
		validateNormalizedCollection(mappingPath+".components", mapping.Components == nil, len(mapping.Components), true, add)
		seenMappingComponents := map[string]bool{}
		for componentIndex, component := range mapping.Components {
			componentPath := fmt.Sprintf("%s.components[%d]", mappingPath, componentIndex)
			validateNormalizedIdentifier(componentPath, component, add)
			if !componentIDs[component] {
				add(componentPath, "unknown-field-component", "surface references a component outside the capability inventory")
			}
			if !seenComponents[component] {
				add(componentPath, "surface-component-mismatch", "surface component is absent from the field component inventory")
			}
			validateNormalizedUnique(componentPath, component, seenMappingComponents, "duplicate-field-component", "surface component must be unique", add)
			usedComponents[component] = true
		}
	}
	for surface := range seenSurfaces {
		if !seenMappedSurfaces[surface] {
			add(path+".surfaceComponents", "missing-surface-components", "surface component mapping is missing for "+surface)
		}
	}
	for component := range seenComponents {
		if !usedComponents[component] {
			add(path+".surfaceComponents", "surface-component-mismatch", "field component is not available on any surface: "+component)
		}
	}
}

func validateNormalizedValidation(validation *NormalizedPresentationValidation, path string, add func(string, string, string)) {
	for name, value := range map[string]*int{"minLength": validation.MinLength, "maxLength": validation.MaxLength} {
		if value != nil && (*value < 0 || int64(*value) > normalizedPresentationMaxSafeInteger) {
			add(path+"."+name, "invalid-field-length", name+" must be a non-negative JSON safe integer")
		}
	}
	if validation.MinLength != nil && validation.MaxLength != nil && *validation.MinLength > *validation.MaxLength {
		add(path, "invalid-field-length-range", "minimum length cannot exceed maximum length")
	}
	minimum, hasMinimum := validateNormalizedDecimalBound(path+".minimum", validation.Minimum, add)
	maximum, hasMaximum := validateNormalizedDecimalBound(path+".maximum", validation.Maximum, add)
	if hasMinimum && hasMaximum && minimum > maximum {
		add(path, "invalid-field-number-range", "minimum cannot exceed maximum")
	}
	if validation.Precision != nil && (*validation.Precision < 1 || *validation.Precision > 38) {
		add(path+".precision", "invalid-field-precision", "precision must be between 1 and 38")
	}
	if validation.Scale != nil && (*validation.Scale < 0 || *validation.Scale > 38) {
		add(path+".scale", "invalid-field-scale", "scale must be between 0 and 38")
	}
	if validation.Precision != nil && validation.Scale != nil && *validation.Scale > *validation.Precision {
		add(path+".scale", "invalid-field-scale", "scale cannot exceed precision")
	}
	if validation.Pattern != "" {
		if _, err := regexp.Compile(validation.Pattern); err != nil {
			add(path+".pattern", "invalid-field-pattern", "field pattern is not a valid regular expression")
		}
		if !isPortablePresentationPattern(validation.Pattern) {
			add(path+".pattern", "non-portable-field-pattern", "field pattern must use the portable Go and ECMAScript subset")
		}
	}
}

func validateNormalizedDecimalBound(path string, value *string, add func(string, string, string)) (float64, bool) {
	if value == nil {
		return 0, false
	}
	if !normalizedPresentationDecimalPattern.MatchString(*value) {
		add(path, "invalid-field-number-bound", "numeric bound must be a finite decimal string")
		return 0, false
	}
	parsed, err := strconv.ParseFloat(*value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		add(path, "invalid-field-number-bound", "numeric bound must be a finite decimal string")
		return 0, false
	}
	return parsed, true
}

func validateNormalizedDataSourceLimits(dataSource *NormalizedPresentationDataSource, path string, add func(string, string, string)) {
	if dataSource.MaxPageSize < 1 || dataSource.MaxPageSize > 200 {
		add(path+".maxPageSize", "invalid-max-page-size", "maximum page size must be between 1 and 200")
	}
	if dataSource.MaxSortFields < 1 || dataSource.MaxSortFields > 3 {
		add(path+".maxSortFields", "invalid-max-sort-fields", "maximum sort fields must be between 1 and 3")
	}
	validateNormalizedCollection(path+".pageSizeOptions", dataSource.PageSizeOptions == nil, len(dataSource.PageSizeOptions), true, add)
	seen := map[int]bool{}
	previous := 0
	for index, option := range dataSource.PageSizeOptions {
		optionPath := fmt.Sprintf("%s.pageSizeOptions[%d]", path, index)
		if option < 1 || option > dataSource.MaxPageSize {
			add(optionPath, "invalid-page-size-option", "page size option must be within the data-source maximum")
		}
		if seen[option] {
			add(optionPath, "duplicate-page-size-option", "page size option must be unique")
		}
		if index > 0 && option <= previous {
			add(optionPath, "unsorted-page-size-options", "page size options must be strictly increasing")
		}
		seen[option] = true
		previous = option
	}
}

func validateNormalizedCompletePresentation(
	defaults *NormalizedCompletePresentation,
	fieldByID map[string]NormalizedPresentationField,
	componentIDs map[string]bool,
	dataSourceByID map[string]NormalizedPresentationDataSource,
	actionByID map[string]NormalizedPresentationAction,
	add func(string, string, string),
) {
	base := "normalizedPresentation.defaultPresentation"
	validateNormalizedLocalizedText(base+".title", defaults.Title, add)
	dataSource, hasDataSource := dataSourceByID[defaults.DataSource]
	if !hasDataSource {
		add(base+".dataSource", "unknown-data-source", "default presentation references an unknown data source")
	}

	collections := []struct {
		surface string
		path    string
		fields  []NormalizedCompleteField
	}{
		{surface: "list", path: base + ".list.columns", fields: defaults.List.Columns},
		{surface: "search", path: base + ".search.fields", fields: defaults.Search.Fields},
		{surface: "form", path: base + ".form.fields", fields: defaults.Form.Fields},
		{surface: "detail", path: base + ".detail.fields", fields: defaults.Detail.Fields},
	}
	fieldsBySurface := make(map[string]map[string]bool, len(collections))
	for _, collection := range collections {
		validateNormalizedCollection(collection.path, collection.fields == nil, len(collection.fields), true, add)
		fieldsBySurface[collection.surface] = validateNormalizedCompleteFields(collection.surface, collection.path, collection.fields, fieldByID, componentIDs, add)
	}
	for fieldID, field := range fieldByID {
		for _, surface := range field.Surfaces {
			if !fieldsBySurface[surface][fieldID] {
				add(base+"."+surface, "missing-default-field", "registered field is missing from its complete default surface: "+fieldID)
			}
		}
	}

	visibleList := false
	for _, field := range defaults.List.Columns {
		visibleList = visibleList || !field.Hidden
	}
	if !visibleList {
		add(base+".list.columns", "empty-list-presentation", "at least one list column must be visible")
	}
	if !contains([]string{"compact", "middle", "large"}, defaults.List.Density) {
		add(base+".list.density", "invalid-density", "default density is not supported")
	}
	if defaults.List.PageSize < 1 || defaults.List.PageSize > 200 {
		add(base+".list.pageSize", "invalid-page-size", "default page size must be between 1 and 200")
	}
	if hasDataSource {
		if defaults.List.PageSize > dataSource.MaxPageSize {
			add(base+".list.pageSize", "page-size-exceeds-data-source-limit", "default page size exceeds the data-source maximum")
		}
		if !containsInt(dataSource.PageSizeOptions, defaults.List.PageSize) {
			add(base+".list.pageSize", "unsupported-page-size", "default page size is not a compiled data-source option")
		}
	}
	validateNormalizedCollection(base+".list.defaultSort", defaults.List.DefaultSort == nil, len(defaults.List.DefaultSort), false, add)
	if len(defaults.List.DefaultSort) > 3 || (hasDataSource && len(defaults.List.DefaultSort) > dataSource.MaxSortFields) {
		add(base+".list.defaultSort", "too-many-sort-fields", "default sort exceeds the compiled limit")
	}
	seenSortFields := map[string]bool{}
	for index, sortValue := range defaults.List.DefaultSort {
		path := fmt.Sprintf("%s.list.defaultSort[%d]", base, index)
		if len(sortValue.Field) < 1 || len(sortValue.Field) > 120 || !camelNamePattern.MatchString(sortValue.Field) {
			add(path+".field", "invalid-field-identifier", "sort field must be a lower-camel identifier not exceeding 120 bytes")
		}
		validateNormalizedUnique(path+".field", sortValue.Field, seenSortFields, "duplicate-sort-field", "sort field must be unique", add)
		field, exists := fieldByID[sortValue.Field]
		if !exists {
			add(path+".field", "unknown-sort-field", "sort references an unknown field")
		} else if !field.Sortable || !contains(field.Surfaces, "list") {
			add(path+".field", "unsupported-sort-field", "field is not sortable on the list surface")
		}
		if !contains([]string{"asc", "desc"}, sortValue.Direction) {
			add(path+".direction", "invalid-sort-direction", "sort direction must be asc or desc")
		}
	}
	if defaults.Form.Columns < 1 || defaults.Form.Columns > 4 {
		add(base+".form.columns", "invalid-layout-columns", "form columns must be between 1 and 4")
	}
	if defaults.Detail.Columns < 1 || defaults.Detail.Columns > 4 {
		add(base+".detail.columns", "invalid-layout-columns", "detail columns must be between 1 and 4")
	}

	validateNormalizedCollection(base+".actions", defaults.Actions == nil, len(defaults.Actions), false, add)
	seenActions := map[string]bool{}
	for index, action := range defaults.Actions {
		path := fmt.Sprintf("%s.actions[%d]", base, index)
		validateNormalizedIdentifier(path+".action", action.Action, add)
		validateNormalizedUnique(path+".action", action.Action, seenActions, "duplicate-action", "default action must be unique", add)
		definition, exists := actionByID[action.Action]
		if !exists {
			add(path+".action", "unknown-action", "default presentation references an unknown action")
		} else {
			if !contains(definition.Placements, action.Placement) {
				add(path+".placement", "unsupported-action-placement", "default action placement is not supported")
			}
			if action.Confirm != nil && !definition.Destructive {
				add(path+".confirm", "confirmation-action-incompatible", "confirmation text requires a destructive action")
			}
		}
		if action.Order < 0 || action.Order > 10000 {
			add(path+".order", "invalid-action-order", "action order must be between 0 and 10000")
		}
		validateNormalizedLocalizedText(path+".label", action.Label, add)
		if action.Confirm != nil {
			validateNormalizedLocalizedText(path+".confirm", *action.Confirm, add)
		}
	}
	for actionID := range actionByID {
		if !seenActions[actionID] {
			add(base+".actions", "missing-default-action", "registered action is missing from complete defaults: "+actionID)
		}
	}
}

func validateNormalizedCompleteFields(
	surface string,
	path string,
	fields []NormalizedCompleteField,
	fieldByID map[string]NormalizedPresentationField,
	componentIDs map[string]bool,
	add func(string, string, string),
) map[string]bool {
	seen := make(map[string]bool, len(fields))
	for index, completeField := range fields {
		fieldPath := fmt.Sprintf("%s[%d]", path, index)
		if len(completeField.Field) < 1 || len(completeField.Field) > 120 || !camelNamePattern.MatchString(completeField.Field) {
			add(fieldPath+".field", "invalid-field-identifier", "default field must be a lower-camel identifier not exceeding 120 bytes")
		}
		validateNormalizedUnique(fieldPath+".field", completeField.Field, seen, "duplicate-field", "default field must be unique on one surface", add)
		definition, exists := fieldByID[completeField.Field]
		if !exists {
			add(fieldPath+".field", "unknown-field", "default presentation references an unknown field")
		} else {
			if !contains(definition.Surfaces, surface) {
				add(fieldPath+".field", "unsupported-field-surface", "default field is not registered for this surface")
			}
			if !normalizedFieldAllowsComponent(definition, surface, completeField.Component) {
				add(fieldPath+".component", "unsupported-field-component", "default component is not allowed for this field and surface")
			}
			if surface == "form" && definition.Required && completeField.Hidden {
				add(fieldPath+".hidden", "required-form-field-hidden", "required form field cannot be hidden")
			}
		}
		validateNormalizedIdentifier(fieldPath+".component", completeField.Component, add)
		if !componentIDs[completeField.Component] {
			add(fieldPath+".component", "unknown-component", "default presentation references a component outside the capability inventory")
		}
		if completeField.Order < 0 || completeField.Order > 10000 {
			add(fieldPath+".order", "invalid-field-order", "default field order must be between 0 and 10000")
		}
		if completeField.Width != nil {
			if surface != "list" {
				add(fieldPath+".width", "invalid-field-width-surface", "field width is supported only on the list surface")
			}
			if *completeField.Width < 60 || *completeField.Width > 1200 {
				add(fieldPath+".width", "invalid-field-width", "field width must be between 60 and 1200")
			}
		}
		if completeField.Span != nil {
			if surface != "form" && surface != "detail" {
				add(fieldPath+".span", "invalid-field-span-surface", "field span is supported only on form and detail surfaces")
			}
			if *completeField.Span < 1 || *completeField.Span > 24 {
				add(fieldPath+".span", "invalid-field-span", "field span must be between 1 and 24")
			}
		}
		validateNormalizedLocalizedText(fieldPath+".label", completeField.Label, add)
		if completeField.Placeholder != nil {
			validateNormalizedLocalizedText(fieldPath+".placeholder", *completeField.Placeholder, add)
		}
		if completeField.Help != nil {
			validateNormalizedLocalizedText(fieldPath+".help", *completeField.Help, add)
		}
	}
	return seen
}

func normalizedFieldAllowsComponent(field NormalizedPresentationField, surface, component string) bool {
	for _, mapping := range field.SurfaceComponents {
		if mapping.Surface == surface {
			return contains(mapping.Components, component)
		}
	}
	return false
}

func validateNormalizedPermissions(path string, permissions []string, add func(string, string, string)) {
	validateNormalizedCollection(path, permissions == nil, len(permissions), true, add)
	seen := map[string]bool{}
	for index, permission := range permissions {
		permissionPath := fmt.Sprintf("%s[%d]", path, index)
		if strings.TrimSpace(permission) == "" || len(permission) > 255 {
			add(permissionPath, "invalid-permission", "trusted permission must be non-empty and not exceed 255 bytes")
		}
		validateNormalizedUnique(permissionPath, permission, seen, "duplicate-permission", "trusted permission must be unique", add)
	}
}

func validateNormalizedLocalizedText(path string, text PresentationLocalizedText, add func(string, string, string)) {
	for locale, value := range map[string]string{"zh-CN": text.ZhCN, "en-US": text.EnUS} {
		localePath := path + "." + locale
		length := len([]rune(value))
		if length < 1 || length > 200 {
			add(localePath, "invalid-localized-text", "normalized localized text must contain 1 to 200 characters in every locale")
		}
		if !utf8.ValidString(value) {
			add(localePath, "invalid-localized-text", "localized text must be valid UTF-8")
		}
		if strings.TrimSpace(value) != value {
			add(localePath, "invalid-localized-text", "localized text must not contain leading or trailing whitespace")
		}
		for _, current := range value {
			if unicode.IsControl(current) {
				add(localePath, "invalid-localized-text", "localized text must not contain control characters")
				break
			}
		}
	}
}

func validateNormalizedIdentifier(path, value string, add func(string, string, string)) {
	if len(value) < 2 || len(value) > 120 || !normalizedPresentationIdentifierPattern.MatchString(value) {
		add(path, "invalid-identifier", "identifier must be stable lower-case syntax between 2 and 120 bytes")
	}
}

func validateNormalizedCollection(path string, isNil bool, length int, requireItems bool, add func(string, string, string)) {
	if isNil {
		add(path, "nil-collection", "normalized manifest collections must be non-nil")
	}
	if requireItems && length == 0 {
		add(path, "empty-collection", "normalized manifest collection must contain at least one item")
	}
}

func validateNormalizedUnique[T comparable](
	path string,
	value T,
	seen map[T]bool,
	code string,
	message string,
	add func(string, string, string),
) {
	if seen[value] {
		add(path, code, message)
	}
	seen[value] = true
}
