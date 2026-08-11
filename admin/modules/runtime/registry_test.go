package runtime

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testModuleRecord struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100;not null"`
}

func TestRegistryRejectsInvalidAndDuplicateModules(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	if err := Register(Descriptor{}); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("Register(empty) error = %v", err)
	}
	descriptor := Descriptor{
		Name:        "supplier",
		DisplayName: "供应商",
		Model:       new(testModuleRecord),
	}
	if err := Register(descriptor); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := Register(descriptor); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate Register() error = %v", err)
	}
}

func TestAllReturnsStableCopies(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	MustRegister(Descriptor{
		Name:        "zeta",
		DisplayName: "Zeta",
		Model:       new(testModuleRecord),
		Permissions: []Permission{{Code: "zeta:read", DisplayName: "Read", DefaultRoles: []string{"reader"}}},
	})
	MustRegister(Descriptor{
		Name:        "alpha",
		DisplayName: "Alpha",
		Model:       new(testModuleRecord),
	})

	all := All()
	if len(all) != 2 || all[0].Name != "alpha" || all[1].Name != "zeta" {
		t.Fatalf("All() = %#v", all)
	}
	all[1].Permissions[0].Code = "mutated"
	all[1].Permissions[0].DefaultRoles[0] = "mutated"
	stored, ok := Get("zeta")
	if !ok {
		t.Fatal("Get(zeta) not found")
	}
	if stored.Permissions[0].Code != "zeta:read" {
		t.Fatalf("registry exposed mutable permission slice: %#v", stored.Permissions)
	}
	if got := stored.Permissions[0].DefaultRoles[0]; got != "reader" {
		t.Fatalf("registry exposed mutable default-role slice: %#v", stored.Permissions)
	}
}

func TestMigrateDoesNotInferDDLFromRegisteredModels(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	MustRegister(Descriptor{
		Name:        "record",
		DisplayName: "Record",
		Model:       new(testModuleRecord),
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if db.Migrator().HasTable(&testModuleRecord{}) {
		t.Fatal("Migrate() inferred production DDL from descriptor model")
	}
}

func TestMigrateUsesCustomMigration(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	called := false
	MustRegister(Descriptor{
		Name:        "custom",
		DisplayName: "Custom",
		Migrate: func(db *gorm.DB) error {
			called = true
			return db.AutoMigrate(new(testModuleRecord))
		},
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if !called {
		t.Fatal("custom migration was not called")
	}
}
