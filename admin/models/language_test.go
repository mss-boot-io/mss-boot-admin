package models

import (
	"errors"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLanguageCreateOwnsIdentifiersAndDefaultsStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&Language{}); err != nil {
		t.Fatalf("migrate languages: %v", err)
	}
	defines := LanguageDefines{{Group: "menu", Key: "welcome", Value: "Welcome"}}
	language := &Language{Name: " en-US ", Defines: &defines}
	if err := db.Create(language).Error; err != nil {
		t.Fatalf("create language: %v", err)
	}
	if language.ID == "" {
		t.Fatal("created language has an empty server-owned ID")
	}
	if language.Status != enum.Enabled {
		t.Fatalf("created language status = %q, want enabled", language.Status)
	}
	if language.Name != "en-US" {
		t.Fatalf("created language name = %q, want canonical whitespace", language.Name)
	}
	if (*language.Defines)[0].ID == "" {
		t.Fatal("created definition has an empty server-owned ID")
	}
}

func TestLanguageValidationAcceptsAndCanonicalizesBCP47Tags(t *testing.T) {
	language := &Language{Name: " es-419 ", Status: enum.Enabled}
	if err := language.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate() error: %v", err)
	}
	if language.Name != "es-419" {
		t.Fatalf("canonical language name = %q, want es-419", language.Name)
	}

	language.Name = "not_a_locale"
	if err := language.NormalizeAndValidate(); !errors.Is(err, ErrLanguageInvalid) {
		t.Fatalf("invalid locale error = %v, want ErrLanguageInvalid", err)
	}
}

func TestLanguageStoredValidationDoesNotRepairCorruptDefinitions(t *testing.T) {
	definitions := LanguageDefines{{Group: " menu ", Key: "welcome", Value: "Welcome"}}
	language := &Language{
		Name:    "en-US",
		Status:  enum.Enabled,
		Defines: &definitions,
	}
	language.ID = "language-1"
	if err := language.ValidateStored(); !errors.Is(err, ErrLanguageInvalid) {
		t.Fatalf("ValidateStored() error = %v, want ErrLanguageInvalid", err)
	}
	if definitions[0].ID != "" || definitions[0].Group != " menu " {
		t.Fatalf("ValidateStored() mutated corrupt definitions: %#v", definitions[0])
	}
}

func TestLanguageValidationRejectsAmbiguousOrUnboundedDefinitions(t *testing.T) {
	tests := []struct {
		name    string
		defines LanguageDefines
	}{
		{
			name: "null definition",
			defines: LanguageDefines{
				nil,
			},
		},
		{
			name: "duplicate translation key",
			defines: LanguageDefines{
				{ID: "one", Group: "menu", Key: "welcome", Value: "A"},
				{ID: "two", Group: "menu", Key: "welcome", Value: "B"},
			},
		},
		{
			name: "duplicate definition ID",
			defines: LanguageDefines{
				{ID: "same", Group: "menu", Key: "one", Value: "A"},
				{ID: "same", Group: "menu", Key: "two", Value: "B"},
			},
		},
		{
			name: "oversized value",
			defines: LanguageDefines{
				{ID: "one", Group: "menu", Key: "welcome", Value: strings.Repeat("界", MaxLanguageDefineValue+1)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			language := &Language{Name: "en-US", Status: enum.Enabled, Defines: &test.defines}
			if err := language.NormalizeAndValidate(); !errors.Is(err, ErrLanguageInvalid) {
				t.Fatalf("NormalizeAndValidate() error = %v, want ErrLanguageInvalid", err)
			}
		})
	}
}

func TestLanguageDefinitionsScanSupportsDatabaseRepresentations(t *testing.T) {
	for _, value := range []any{`[{"id":"one","group":"menu","key":"welcome","value":"Welcome"}]`, []byte(`[{"id":"one","group":"menu","key":"welcome","value":"Welcome"}]`)} {
		var definitions LanguageDefines
		if err := definitions.Scan(value); err != nil {
			t.Fatalf("Scan(%T) error: %v", value, err)
		}
		if len(definitions) != 1 || definitions[0].Key != "welcome" {
			t.Fatalf("Scan(%T) = %#v", value, definitions)
		}
	}
	definitions := LanguageDefines{{ID: "stale"}}
	if err := definitions.Scan(nil); err != nil || definitions != nil {
		t.Fatalf("Scan(nil) = (%#v, %v), want (nil, nil)", definitions, err)
	}
}
