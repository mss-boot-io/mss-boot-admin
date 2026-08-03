package spec

import (
	"fmt"
	"strings"
)

// RenderModuleTemplate returns a semantically valid starter AdminModule.
func RenderModuleTemplate(name, displayName string) ([]byte, error) {
	name = normalizeIdentifier(name)
	if !moduleNamePattern.MatchString(name) {
		return nil, fmt.Errorf("module name %q must be lower-case kebab-case", name)
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = titleWords(name)
	}
	module := &Module{
		APIVersion: ModuleAPIVersion,
		Kind:       ModuleKind,
		Metadata: ModuleMetadata{
			Name:        name,
			DisplayName: displayName,
			Description: "Manage " + displayName + " records through a generated vertical module.",
		},
		Spec: ModuleSpec{
			Entity: EntitySpec{
				GoName: PascalCase(name),
				Table:  "biz_" + SnakeCase(simplePlural(name)),
				IDType: "uuid",
				Fields: []FieldSpec{
					{
						Name:        "name",
						DisplayName: "Name",
						Type:        "string",
						Required:    true,
						Searchable:  true,
						Sortable:    true,
						Validation: ValidationSpec{
							MinLength: intPointer(1),
							MaxLength: intPointer(200),
						},
						UI: FieldUISpec{Component: "input"},
					},
					{
						Name:        "status",
						DisplayName: "Status",
						Type:        "enum",
						Required:    true,
						Filterable:  true,
						Default:     "enabled",
						EnumValues: []EnumValue{
							{Value: "disabled", Label: "Disabled", LabelEn: "Disabled", Color: "default"},
							{Value: "enabled", Label: "Enabled", LabelEn: "Enabled", Color: "green"},
						},
						UI: FieldUISpec{Component: "select"},
					},
				},
			},
			API: APISpec{
				BasePath:   "/" + simplePlural(name),
				Version:    "v1",
				Operations: []string{"list", "get", "create", "update", "delete"},
			},
			Permissions: []Permission{
				{Action: "create", DisplayName: "Create " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "delete", DisplayName: "Delete " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "list", DisplayName: "List " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "read", DisplayName: "Read " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "update", DisplayName: "Update " + displayName, DefaultRoles: []string{"admin"}},
			},
			Ownership: OwnershipSpec{Mode: "none"},
			Menu: MenuSpec{
				Path:          "/" + simplePlural(name),
				DisplayName:   displayName,
				DisplayNameEn: displayName,
				Icon:          "table",
			},
			UI: UISpec{
				List:   true,
				Form:   true,
				Detail: true,
			},
			Tests: TestSpec{
				Unit:             true,
				API:              true,
				E2E:              true,
				PermissionMatrix: true,
			},
		},
	}
	module.Normalize()
	if issues := module.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return module.YAML()
}

func intPointer(value int) *int {
	return &value
}
