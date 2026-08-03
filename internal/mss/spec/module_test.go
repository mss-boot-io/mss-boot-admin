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
	if issues := module.Validate(); len(issues) != 0 {
		t.Fatalf("Validate() issues = %#v", issues)
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
