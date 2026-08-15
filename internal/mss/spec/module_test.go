package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExampleSupplierModule(t *testing.T) {
	root := findRepositoryRoot(t)
	module, err := LoadModule(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	if module.Metadata.Name != "supplier" {
		t.Fatalf("module name = %q", module.Metadata.Name)
	}
	if module.Spec.Entity.GoName != "Supplier" {
		t.Fatalf("entity Go name = %q", module.Spec.Entity.GoName)
	}
	if module.PermissionCode("read") != "supplier:read" {
		t.Fatalf("permission code = %q", module.PermissionCode("read"))
	}
	if field, ok := module.Field("creditLevel"); !ok || field.Column != "credit_level" {
		t.Fatalf("creditLevel field = %#v, exists=%t", field, ok)
	}
	if field, ok := module.Field("contactName"); !ok || !field.Required || field.Nullable {
		t.Fatalf("contactName required contract = %#v, exists=%t", field, ok)
	}
	financeCanList := false
	for _, permission := range module.Spec.Permissions {
		if permission.Action != "list" {
			continue
		}
		for _, role := range permission.DefaultRoles {
			financeCanList = financeCanList || role == "finance"
		}
	}
	if !financeCanList {
		t.Fatal("supplier list permission omitted the finance role")
	}
	if got, want := module.Spec.Generation.MigrationID, "20260810160000"; got != want {
		t.Fatalf("migration ID = %q, want %q", got, want)
	}
	if got, want := module.Spec.Generation.AuthorizationMigrationID, "20260811120000"; got != want {
		t.Fatalf("authorization migration ID = %q, want %q", got, want)
	}
	if !module.SupportsFrontendTarget(FrontendTargetAntDV5) || !module.SupportsFrontendTarget(FrontendTargetAntDV6) {
		t.Fatalf("supplier frontend targets = %#v, want v5 and v6", module.Spec.Generation.FrontendTargets)
	}
	if got, want := module.Spec.Menu.ParentDisplayName, "采购管理"; got != want {
		t.Fatalf("parent display name = %q, want %q", got, want)
	}
	if got, want := module.Spec.Menu.ParentDisplayNameEn, "Procurement"; got != want {
		t.Fatalf("English parent display name = %q, want %q", got, want)
	}
	if issues := module.Validate(); len(issues) != 0 {
		t.Fatalf("Validate() issues = %#v", issues)
	}
}

func TestSupplierSourceSpecMatchesFeatureAccessContract(t *testing.T) {
	root := findRepositoryRoot(t)
	module, err := LoadModule(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}
	contact, ok := module.Field("contactName")
	if !ok || !contact.Required || contact.Nullable {
		t.Fatalf("primary contact contract = %#v, exists=%t", contact, ok)
	}
	for _, permission := range module.Spec.Permissions {
		if permission.Action != "list" {
			continue
		}
		for _, role := range permission.DefaultRoles {
			if role == "finance" {
				return
			}
		}
	}
	t.Fatal("finance role cannot list/search suppliers")
}

func TestModuleValidationRejectsUnsafeOrAmbiguousEvents(t *testing.T) {
	for _, test := range []struct {
		name   string
		events []EventSpec
		code   string
	}{
		{name: "invalid name", events: []EventSpec{{Name: "supplier\"created", When: "created"}}, code: "invalid-event-name"},
		{name: "duplicate trigger", events: []EventSpec{{Name: "supplier.created", When: "created"}, {Name: "supplier.created-again", When: "created"}}, code: "duplicate-event-trigger"},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := validModule()
			module.Spec.Events = test.events
			found := false
			for _, issue := range module.Validate() {
				found = found || issue.Code == test.code
			}
			if !found {
				t.Fatalf("Validate() omitted issue %q", test.code)
			}
		})
	}
}

func TestModuleNormalizeDerivesStableDefaults(t *testing.T) {
	module := validModule()
	module.Spec.Entity.GoName = ""
	module.Spec.Entity.IDType = ""
	module.Spec.API.BasePath = ""
	module.Spec.API.Version = ""
	module.Spec.API.Operations = nil
	module.Spec.Entity.Fields[0].Column = ""
	module.Spec.Entity.Fields[0].GoName = ""
	module.Normalize()

	if got, want := module.Spec.Entity.GoName, "PurchaseOrder"; got != want {
		t.Fatalf("GoName = %q, want %q", got, want)
	}
	if got, want := module.Spec.API.BasePath, "/purchase-orders"; got != want {
		t.Fatalf("BasePath = %q, want %q", got, want)
	}
	field := module.Spec.Entity.Fields[0]
	if field.Column != "order_code" || field.GoName != "OrderCode" {
		t.Fatalf("normalized field = %#v", field)
	}
	if issues := module.Validate(); len(issues) != 0 {
		t.Fatalf("Validate() issues = %#v", issues)
	}
}

func TestModuleValidationReturnsAllImportantIssues(t *testing.T) {
	module := validModule()
	module.Metadata.Name = "Bad_Name"
	module.Metadata.DisplayName = ""
	module.Spec.Entity.Fields = append(module.Spec.Entity.Fields, module.Spec.Entity.Fields[0])
	module.Spec.API.Operations = append(module.Spec.API.Operations, "publish")
	module.Spec.Ownership = OwnershipSpec{Mode: "creator", Field: "missingOwner"}
	module.Spec.Menu.Path = "not/absolute"
	module.Normalize()

	issues := module.Validate()
	if len(issues) < 6 {
		t.Fatalf("Validate() returned %d issues, want multiple independent diagnostics: %#v", len(issues), issues)
	}
	codes := make(map[string]bool, len(issues))
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	for _, code := range []string{
		"invalid-module-name",
		"required",
		"duplicate-field",
		"unsupported-operation",
		"unknown-field",
		"invalid-menu-path",
	} {
		if !codes[code] {
			t.Fatalf("missing validation code %q in %#v", code, issues)
		}
	}
}

func TestLoadModuleReportsInvalidRegex(t *testing.T) {
	module := validModule()
	module.Spec.Entity.Fields[0].Validation.Pattern = "["
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "module.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = LoadModule(path)
	if err == nil || !strings.Contains(err.Error(), "invalid-pattern") {
		t.Fatalf("LoadModule() error = %v, want invalid-pattern", err)
	}
}

func TestLoadModuleRejectsUnknownFields(t *testing.T) {
	module := validModule()
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	data = append(data, []byte("unknownContract: true\n")...)
	path := filepath.Join(t.TempDir(), "module.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = LoadModule(path)
	if err == nil || !strings.Contains(err.Error(), "field unknownContract not found") {
		t.Fatalf("LoadModule() error = %v, want strict unknown-field failure", err)
	}
}

func TestLoadModuleRejectsDuplicateMappingKeys(t *testing.T) {
	module := validModule()
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	data = append(data, []byte("apiVersion: mss.io/v1alpha1\n")...)
	path := filepath.Join(t.TempDir(), "module.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = LoadModule(path)
	if err == nil || !strings.Contains(err.Error(), "mapping key \"apiVersion\" already defined") {
		t.Fatalf("LoadModule() error = %v, want duplicate-key failure", err)
	}
}

func TestLoadModuleRejectsMultipleYAMLDocuments(t *testing.T) {
	module := validModule()
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	data = append(data, []byte("---\napiVersion: mss.io/v1alpha1\nkind: AdminModule\n")...)
	path := filepath.Join(t.TempDir(), "module.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = LoadModule(path)
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents are not allowed") {
		t.Fatalf("LoadModule() error = %v, want multi-document failure", err)
	}
}

func TestModuleValidationRejectsMalformedMigrationID(t *testing.T) {
	module := validModule()
	module.Spec.Generation.MigrationID = "020260810160000"
	issues := module.Validate()
	for _, issue := range issues {
		if issue.Code == "invalid-migration-id" {
			return
		}
	}
	t.Fatalf("Validate() issues = %#v, want invalid-migration-id", issues)
}

func TestModuleValidationRejectsMissingOrDuplicateAuthorizationMigrationID(t *testing.T) {
	for _, test := range []struct {
		name string
		id   string
		code string
	}{
		{name: "missing", id: "", code: "required"},
		{name: "malformed", id: "020260811120000", code: "invalid-migration-id"},
		{name: "duplicates entity", id: "20260810160002", code: "duplicate-migration-id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := validModule()
			module.Spec.Generation.AuthorizationMigrationID = test.id
			for _, issue := range module.Validate() {
				if issue.Path == "spec.generation.authorizationMigrationID" && issue.Code == test.code {
					return
				}
			}
			t.Fatalf("Validate() issues = %#v, want %s", module.Validate(), test.code)
		})
	}
}

func TestModuleValidationRequiresMigrationIDForBackendGeneration(t *testing.T) {
	module := validModule()
	module.Spec.Generation.MigrationID = ""
	issues := module.Validate()
	for _, issue := range issues {
		if issue.Path == "spec.generation.migrationID" && issue.Code == "required" {
			return
		}
	}
	t.Fatalf("Validate() issues = %#v, want required migration ID", issues)
}

func TestModuleFrontendTargetsDefaultToV5AndRejectInvalidDeclarations(t *testing.T) {
	module := validModule()
	if got := module.Spec.Generation.FrontendTargets; len(got) != 1 || got[0] != FrontendTargetAntDV5 {
		t.Fatalf("default frontend targets = %#v, want [%s]", got, FrontendTargetAntDV5)
	}
	if !module.SupportsFrontendTarget(FrontendTargetAntDV5) || module.SupportsFrontendTarget(FrontendTargetAntDV6) {
		t.Fatalf("default target support = %#v", module.Spec.Generation.FrontendTargets)
	}

	module.Spec.Generation.FrontendTargets = []string{FrontendTargetAntDV6, FrontendTargetAntDV6, "unknown"}
	issues := module.Validate()
	wants := map[string]bool{
		"duplicate-frontend-target":   false,
		"unsupported-frontend-target": false,
	}
	for _, issue := range issues {
		if _, exists := wants[issue.Code]; exists {
			wants[issue.Code] = true
		}
	}
	for code, found := range wants {
		if !found {
			t.Fatalf("Validate() omitted %s from %#v", code, issues)
		}
	}
}

func TestModuleAntDV6ProfileFailsClosedOutsideQualifiedSurface(t *testing.T) {
	qualified := validModule()
	qualified.Spec.Generation.FrontendTargets = []string{FrontendTargetAntDV6}
	qualified.Spec.Entity.Fields[0].Unique = true
	qualified.Spec.API.Operations = append(qualified.Spec.API.Operations, "export")
	qualified.Spec.Permissions = append(qualified.Spec.Permissions, Permission{
		Action:      "export",
		DisplayName: "导出",
	})
	qualified.Spec.UI.Export = true
	if issues := qualified.Validate(); len(issues) != 0 {
		t.Fatalf("qualified v6 module issues = %#v", issues)
	}

	unsupported := validModule()
	unsupported.Spec.Generation.FrontendTargets = []string{FrontendTargetAntDV6}
	unsupported.Spec.Entity.Fields[0].Type = "int"
	unsupported.Spec.Entity.Fields[0].Form = boolPointer(false)
	unsupported.Spec.UI.BatchDelete = true
	issues := unsupported.Validate()
	wants := map[string]bool{
		"antd-v6-field-type-unsupported":      false,
		"antd-v6-operation-required":          false,
		"antd-v6-required-field-not-editable": false,
		"antd-v6-ui-required":                 false,
		"antd-v6-ui-unsupported":              false,
	}
	for _, issue := range issues {
		if _, exists := wants[issue.Code]; exists {
			wants[issue.Code] = true
		}
	}
	for code, found := range wants {
		if !found {
			t.Fatalf("Validate() omitted %s from %#v", code, issues)
		}
	}
}

func TestIdentifierConversions(t *testing.T) {
	tests := []struct {
		input  string
		pascal string
		snake  string
	}{
		{input: "purchase-order", pascal: "PurchaseOrder", snake: "purchase_order"},
		{input: "contactEmail", pascal: "ContactEmail", snake: "contact_email"},
		{input: "APIKey", pascal: "APIKey", snake: "api_key"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := PascalCase(tt.input); got != tt.pascal {
				t.Fatalf("PascalCase(%q) = %q, want %q", tt.input, got, tt.pascal)
			}
			if got := SnakeCase(tt.input); got != tt.snake {
				t.Fatalf("SnakeCase(%q) = %q, want %q", tt.input, got, tt.snake)
			}
		})
	}
}

func validModule() *Module {
	module := &Module{
		APIVersion: ModuleAPIVersion,
		Kind:       ModuleKind,
		Metadata: ModuleMetadata{
			Name:        "purchase-order",
			DisplayName: "采购订单",
		},
		Spec: ModuleSpec{
			Entity: EntitySpec{
				GoName: "PurchaseOrder",
				Table:  "biz_purchase_orders",
				IDType: "uuid",
				Fields: []FieldSpec{
					{
						Name:        "orderCode",
						Column:      "order_code",
						GoName:      "OrderCode",
						DisplayName: "订单编码",
						Type:        "string",
						Required:    true,
						Searchable:  true,
					},
				},
			},
			API: APISpec{
				BasePath:   "/purchase-orders",
				Version:    "v1",
				Operations: []string{"list", "get", "create", "update", "delete"},
			},
			Permissions: []Permission{
				{Action: "list", DisplayName: "列表"},
				{Action: "read", DisplayName: "详情"},
				{Action: "create", DisplayName: "创建"},
				{Action: "update", DisplayName: "更新"},
				{Action: "delete", DisplayName: "删除"},
			},
			Ownership: OwnershipSpec{Mode: "none"},
			Menu: MenuSpec{
				Path:        "/purchase-orders",
				DisplayName: "采购订单",
			},
			UI: UISpec{List: true, Form: true, Detail: true},
			Tests: TestSpec{
				Unit:             true,
				API:              true,
				E2E:              true,
				PermissionMatrix: true,
			},
			Generation: GenerationSpec{
				MigrationID:              "20260810160002",
				AuthorizationMigrationID: "20260810160003",
			},
		},
	}
	module.Normalize()
	return module
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".mss", "project.yaml")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("repository root not found")
		}
		current = parent
	}
}
