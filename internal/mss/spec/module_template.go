package spec

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
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
	migrationID, authorizationMigrationID := starterMigrationIDs(name)
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
						Unique:      true,
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
				Operations: []string{"list", "get", "create", "update", "delete", "export"},
			},
			Permissions: []Permission{
				{Action: "create", DisplayName: "Create " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "delete", DisplayName: "Delete " + displayName, DefaultRoles: []string{"admin"}},
				{Action: "export", DisplayName: "Export " + displayName, DefaultRoles: []string{"admin"}},
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
				Export: true,
			},
			Tests: TestSpec{
				Unit:             true,
				API:              true,
				E2E:              true,
				PermissionMatrix: true,
			},
			Generation: GenerationSpec{
				MigrationID:              migrationID,
				AuthorizationMigrationID: authorizationMigrationID,
			},
		},
	}
	module.Normalize()
	if issues := module.Validate(); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return module.YAML()
}

// starterMigrationIDs returns a deterministic, module-specific adjacent pair.
// The 14-digit range is intentionally outside the timestamp-shaped IDs used by
// the Foundation itself, while preserving stable decimal ordering and leaving
// collision detection to the generator's repository-wide preflight.
func starterMigrationIDs(name string) (string, string) {
	digest := sha256.Sum256([]byte("mss-admin-module:" + name))
	const pairCount uint64 = 30_000_000_000_000
	base := uint64(30_000_000_000_000) + 2*(binary.BigEndian.Uint64(digest[:8])%pairCount)
	return strconv.FormatUint(base, 10), strconv.FormatUint(base+1, 10)
}

func intPointer(value int) *int {
	return &value
}
