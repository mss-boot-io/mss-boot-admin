package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

// ProjectionStatus states how one specification surface is handled by the
// current generator checkpoint.
type ProjectionStatus string

const (
	ProjectionImplemented    ProjectionStatus = "implemented"
	ProjectionValidationOnly ProjectionStatus = "validation-only"
	ProjectionDeferred       ProjectionStatus = "deferred"
	ProjectionUnsupported    ProjectionStatus = "unsupported"
)

// Projection is one explicit field-to-output-kind decision. Deferred entries
// keep the backend checkpoint honest: they are visible in every plan and keep
// Plan.Complete false until their output kinds are implemented.
type Projection struct {
	Path        string           `json:"path"`
	Status      ProjectionStatus `json:"status"`
	OutputKinds []string         `json:"outputKinds,omitempty"`
	Detail      string           `json:"detail"`
}

// ProjectionError rejects declarations that would affect backend semantics
// but cannot be implemented by this generator checkpoint.
type ProjectionError struct {
	Projections []Projection
}

func (e *ProjectionError) Error() string {
	if e == nil || len(e.Projections) == 0 {
		return "module projection is unsupported"
	}
	parts := make([]string, 0, len(e.Projections))
	for _, projection := range e.Projections {
		parts = append(parts, projection.Path+": "+projection.Detail)
	}
	return "module projection preflight failed: " + strings.Join(parts, "; ")
}

func buildProjectionReport(module *spec.Module) ([]Projection, error) {
	projections := make([]Projection, 0, 32+len(module.Spec.Entity.Fields)*2)
	add := func(path string, status ProjectionStatus, detail string, kinds ...string) {
		kinds = append([]string(nil), kinds...)
		sort.Strings(kinds)
		projections = append(projections, Projection{
			Path:        path,
			Status:      status,
			OutputKinds: kinds,
			Detail:      detail,
		})
	}

	add("metadata", ProjectionImplemented, "module identity is emitted into generated backend metadata", "backend-descriptor", "generated-source")
	if len(module.Metadata.Labels) > 0 {
		add("metadata.labels", ProjectionValidationOnly, "labels remain source-contract metadata and do not change backend behavior", "normalized-spec")
	}
	add("spec.entity.goName", ProjectionImplemented, "entity name drives generated Go contracts", "api", "dto", "events", "migration", "model", "service", "tests")
	add("spec.entity.table", ProjectionImplemented, "table name drives explicit DDL and runtime queries", "migration", "model", "service")
	if module.Spec.Entity.IDType != "uuid" {
		add("spec.entity.idType", ProjectionUnsupported, "this backend checkpoint supports only uuid identifiers", "migration", "model", "service")
	} else {
		add("spec.entity.idType", ProjectionImplemented, "uuid identifiers use the framework model boundary", "migration", "model")
	}
	if module.Spec.Entity.Timestamps == nil || !*module.Spec.Entity.Timestamps {
		add("spec.entity.timestamps", ProjectionUnsupported, "this backend checkpoint requires timestamps", "migration", "model")
	} else {
		add("spec.entity.timestamps", ProjectionImplemented, "createdAt and updatedAt are persisted and exposed", "api", "migration", "model")
	}
	if module.Spec.Entity.SoftDelete == nil || !*module.Spec.Entity.SoftDelete {
		add("spec.entity.softDelete", ProjectionUnsupported, "this backend checkpoint requires soft deletion", "migration", "model", "service")
	} else {
		add("spec.entity.softDelete", ProjectionImplemented, "delete operations preserve rows through deleted_at", "migration", "model", "service")
	}

	for index, field := range module.Spec.Entity.Fields {
		path := fmt.Sprintf("spec.entity.fields[%d]", index)
		if field.Type != "string" && field.Type != "enum" && field.Type != "bool" {
			add(path+".type", ProjectionUnsupported, "this backend checkpoint supports string, enum, and bool fields only", "api", "dto", "migration", "model", "service")
			continue
		}
		if field.Nullable {
			add(path+".nullable", ProjectionUnsupported, "nullable business fields are not implemented by this backend checkpoint", "dto", "migration", "model")
		}
		if field.Relation != nil {
			add(path+".relation", ProjectionUnsupported, "relations require a separately generated referential-integrity contract", "api", "migration", "service")
		}
		if field.Validation.Format != "" && field.Validation.Format != "email" {
			add(path+".validation.format", ProjectionUnsupported, "only the email string format is implemented by this backend checkpoint", "dto", "tests")
		}
		if field.Searchable && field.Type != "string" {
			add(path+".searchable", ProjectionUnsupported, "substring search is implemented only for string fields", "dto", "service", "tests")
		}
		if field.Validation.Minimum != nil || field.Validation.Maximum != nil || field.Validation.Precision != nil || field.Validation.Scale != nil {
			add(path+".validation", ProjectionUnsupported, "numeric validation cannot apply to the supported string, enum, or bool field set", "dto", "tests")
		}
		if field.Default != nil {
			switch field.Type {
			case "bool":
				if _, ok := field.Default.(bool); !ok {
					add(path+".default", ProjectionUnsupported, "bool defaults must be boolean values", "dto", "migration")
				}
			case "string", "enum":
				if _, ok := field.Default.(string); !ok {
					add(path+".default", ProjectionUnsupported, "string and enum defaults must be strings", "dto", "migration")
				}
			}
		}
		add(path, ProjectionImplemented, "field shape, constraints, query behavior, export value, and DDL are generated", "api", "dto", "export", "migration", "model", "service", "tests")
		if field.Type == "enum" {
			add(path+".enumValues[*].value", ProjectionImplemented, "enum values are enforced by DTO validation and database check constraints", "dto", "migration", "tests")
			add(path+".enumValues[*].presentation", ProjectionImplemented, "enum labels and colors are projected into synchronized locale entries, filters, forms, and table tags", "frontend", "frontend-locales", "frontend-tests")
		}
		if field.UI.Component != "" || field.UI.Width != nil || field.UI.Placeholder != "" || field.UI.Help != "" || field.UI.Hidden {
			add(path+".ui", ProjectionImplemented, "field presentation drives generated columns, validation-aware controls, responsive widths, and localized help", "frontend", "frontend-locales")
		}
	}

	indexNames := map[string]string{
		"idx_" + module.Spec.Entity.Table + "_deleted_at": "generated soft-delete index",
	}
	for index, field := range module.Spec.Entity.Fields {
		name := ""
		if field.Unique {
			name = "ux_" + module.Spec.Entity.Table + "_" + field.Column
		} else if field.Index {
			name = "idx_" + module.Spec.Entity.Table + "_" + field.Column
		}
		if name != "" {
			if owner, exists := indexNames[name]; exists {
				add(fmt.Sprintf("spec.entity.fields[%d]", index), ProjectionUnsupported, "generated index name collides with "+owner, "migration")
			}
			indexNames[name] = fmt.Sprintf("field %s", field.Name)
		}
	}
	for index, databaseIndex := range module.Spec.Entity.Indexes {
		if owner, exists := indexNames[databaseIndex.Name]; exists {
			add(fmt.Sprintf("spec.entity.indexes[%d].name", index), ProjectionUnsupported, "index name collides with "+owner, "migration")
		}
		indexNames[databaseIndex.Name] = "declared index"
	}
	add("spec.entity.indexes", ProjectionImplemented, "named field groups are emitted as explicit ordered database indexes", "migration", "tests")
	add("spec.api.basePath", ProjectionImplemented, "the canonical resource path drives authorized routes and OpenAPI annotations", "api", "openapi", "tests")
	if module.Spec.API.Version != "v1" {
		add("spec.api.version", ProjectionUnsupported, "this backend checkpoint exposes only v1 routes", "api", "openapi")
	} else {
		add("spec.api.version", ProjectionImplemented, "v1 is recorded in backend operation contracts", "api", "openapi")
	}
	for index, operation := range module.Spec.API.Operations {
		if operation == "import" {
			add(fmt.Sprintf("spec.api.operations[%d]", index), ProjectionUnsupported, "import is not implemented by this backend checkpoint", "api", "service")
			continue
		}
		add(fmt.Sprintf("spec.api.operations[%d]", index), ProjectionImplemented, "operation has an authorized route, service method, OpenAPI contract, and test", "api", "openapi", "service", "tests")
	}

	for index, permission := range module.Spec.Permissions {
		permissionPath := fmt.Sprintf("spec.permissions[%d]", index)
		add(permissionPath+".action", ProjectionImplemented, "the derived permission code is enforced by the required injected Admin backend authorizer", "api", "authorization-policy", "tests")
		add(permissionPath+".displayName", ProjectionImplemented, "permission display metadata is emitted to the runtime descriptor and persisted as auditable Admin authorization metadata", "authorization-policy", "backend-descriptor", "migration")
		add(permissionPath+".displayName", ProjectionImplemented, "permission presentation is emitted to synchronized locale metadata while exact component paths drive action visibility", "frontend", "frontend-locales", "frontend-tests")
		if permission.Description != "" {
			add(permissionPath+".description", ProjectionImplemented, "permission descriptions are emitted to the backend runtime descriptor", "backend-descriptor")
			add(permissionPath+".description", ProjectionImplemented, "permission descriptions remain synchronized with generated frontend permission metadata", "frontend")
		}
		if len(permission.DefaultRoles) > 0 {
			add(permissionPath+".defaultRoles", ProjectionImplemented, "default roles are resolved or provisioned and receive exact COMPONENT/API policies in one additive transaction", "authorization-policy", "authorization-tests", "migration")
		}
	}
	if module.Spec.Ownership.Mode != "none" {
		add("spec.ownership", ProjectionUnsupported, "non-trivial ownership requires the authorization checkpoint and must fail closed", "authorization-policy", "service", "tests")
	} else {
		add("spec.ownership", ProjectionImplemented, "none is enforced as role-only authorization with no row-ownership fallback or filter", "authorization-policy", "service", "tests")
		add("spec.ownership.adminBypass", ProjectionImplemented, "Admin root bypass is explicit while non-root roles remain bound to exact persisted policies", "authorization-policy", "tests")
	}
	add("spec.menu", ProjectionImplemented, "menu path, parent, icon, order, visibility, permission metadata, and default-role policies are persisted transactionally", "authorization-policy", "menu-migration", "tests")
	add("spec.menu", ProjectionImplemented, "the generated Umi route and synchronized locales bind the persisted menu path to the typed module page", "frontend-route", "frontend-locales", "frontend-tests")
	add("spec.ui", ProjectionImplemented, "list, create/edit form, detail, responsive layout, export, loading, empty, error, forbidden, conflict, and destructive confirmation states are generated as declared", "frontend", "frontend-tests")
	if module.Spec.Workflow != nil {
		add("spec.workflow", ProjectionUnsupported, "workflow generation is outside this backend checkpoint", "api", "migration", "service", "tests")
	}
	for index, event := range module.Spec.Events {
		if event.When != "created" && event.When != "updated" {
			add(fmt.Sprintf("spec.events[%d]", index), ProjectionUnsupported, "only created and updated typed events are implemented by this backend checkpoint", "events", "service")
			continue
		}
		add(fmt.Sprintf("spec.events[%d]", index), ProjectionImplemented, "typed event is collected only after the authoritative transaction commits", "events", "service", "tests")
	}
	add("spec.tests.unit", ProjectionImplemented, "generated unit and service contracts are emitted", "tests")
	add("spec.tests.api", ProjectionImplemented, "generated API and OpenAPI contracts are emitted", "tests")
	if module.Spec.Tests.E2E {
		if module.Spec.Generation.Frontend == nil || !*module.Spec.Generation.Frontend || !module.Spec.UI.List {
			add("spec.tests.e2e", ProjectionUnsupported, "browser E2E generation requires the generated frontend list page", "browser-e2e")
		} else {
			add("spec.tests.e2e", ProjectionImplemented, "an executable Playwright flow is generated for the declared list, create, detail, edit, export, and delete UI surfaces; actual browser-run evidence remains separately reportable", "browser-e2e")
		}
	}
	if module.Spec.Tests.PermissionMatrix {
		add("spec.tests.permissionMatrix", ProjectionImplemented, "generated tests cover every declared default-role allow and deny plus missing identity", "authorization-tests")
	}
	if module.Spec.Tests.OwnershipIsolation {
		add("spec.tests.ownershipIsolation", ProjectionUnsupported, "ownership isolation was requested without a supported ownership implementation", "authorization-tests")
	}
	if module.Spec.Tests.MigrationUpgrade != nil && *module.Spec.Tests.MigrationUpgrade {
		add("spec.tests.migrationUpgrade", ProjectionImplemented, "fresh, upgrade, repeat, and interrupted forward migration cases are emitted", "migration-tests")
	}

	if module.Spec.Generation.MigrationID == "" {
		add("spec.generation.migrationID", ProjectionUnsupported, "backend generation requires an explicit complete migration ID", "migration")
	} else {
		add("spec.generation.migrationID", ProjectionImplemented, "complete decimal ID is registered without truncation", "migration", "tests")
	}
	if module.Spec.Generation.AuthorizationMigrationID == "" {
		add("spec.generation.authorizationMigrationID", ProjectionUnsupported, "backend authorization generation requires an explicit additive migration ID", "migration")
	} else {
		add("spec.generation.authorizationMigrationID", ProjectionImplemented, "the additive authorization seed is registered under its own complete decimal ID", "authorization-policy", "migration", "tests")
	}
	if module.Spec.Generation.Backend == nil || !*module.Spec.Generation.Backend {
		add("spec.generation.backend", ProjectionUnsupported, "this command is producing the backend checkpoint and backend generation must be enabled", "backend")
	} else {
		add("spec.generation.backend", ProjectionImplemented, "backend generation is enabled", "backend")
	}
	if module.Spec.Generation.Frontend != nil && *module.Spec.Generation.Frontend {
		add("spec.generation.frontend", ProjectionImplemented, "typed client, permission contracts, React page, route registry, locale registries, and focused tests are generated", "frontend", "frontend-locales", "frontend-route", "frontend-tests")
		add("spec.generation.frontendTargets", ProjectionImplemented, "each declared frontend profile is generated only when that target is selected", "frontend")
	}
	if module.Spec.Generation.Docs != nil && *module.Spec.Generation.Docs {
		add("spec.generation.docs", ProjectionImplemented, "source-linked module contracts, migrations, permissions, generated outputs, and executable validation commands are emitted as deterministic documentation", "docs")
	}
	if module.Spec.Generation.Tests != nil && *module.Spec.Generation.Tests {
		add("spec.generation.tests", ProjectionImplemented, "backend contract tests are generated", "tests")
	}

	sort.SliceStable(projections, func(i, j int) bool {
		if projections[i].Path == projections[j].Path {
			if projections[i].Status == projections[j].Status {
				return strings.Join(projections[i].OutputKinds, ",") < strings.Join(projections[j].OutputKinds, ",")
			}
			return projections[i].Status < projections[j].Status
		}
		return projections[i].Path < projections[j].Path
	})
	unsupported := make([]Projection, 0)
	for _, projection := range projections {
		if projection.Status == ProjectionUnsupported {
			unsupported = append(unsupported, projection)
		}
	}
	if len(unsupported) > 0 {
		return projections, &ProjectionError{Projections: unsupported}
	}
	return projections, nil
}
