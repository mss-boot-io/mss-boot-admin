package models

import (
	"errors"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

func TestOptionNormalizeOwnsDefaultsAndItemIdentifiers(t *testing.T) {
	items := OptionItems{{Key: " active ", Label: " Active ", Value: " enabled ", Color: " green "}}
	option := &Option{
		Category: " system ",
		Name:     " status ",
		Items:    &items,
	}
	if err := option.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate() error: %v", err)
	}
	if option.Category != "system" || option.Name != "status" || option.Status != enum.Enabled || option.Version != 1 {
		t.Fatalf("normalized option = %#v", option)
	}
	if items[0].ID == "" || items[0].Key != "active" || items[0].Value != "enabled" {
		t.Fatalf("normalized item = %#v", items[0])
	}
}

func TestOptionValidationUsesCharacterLimitsForUnicodeLabels(t *testing.T) {
	items := OptionItems{{Key: "状态", Label: strings.Repeat("中", MaxOptionItemLabelLength), Value: "启用"}}
	option := &Option{
		Category: "系统",
		Name:     "状态",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &items,
	}
	if err := option.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate() rejected a label within the character limit: %v", err)
	}
	items[0].Label += "中"
	if err := option.NormalizeAndValidate(); !errors.Is(err, ErrOptionInvalid) {
		t.Fatalf("NormalizeAndValidate() error = %v, want ErrOptionInvalid", err)
	}
}

func TestOptionStoredValidationDoesNotRepairCorruptItems(t *testing.T) {
	items := OptionItems{{Key: "active", Label: "Active", Value: "enabled"}}
	option := &Option{
		Category: "system",
		Name:     "status",
		Status:   enum.Enabled,
		Version:  1,
		Items:    &items,
	}
	option.ID = "option-1"
	if err := option.ValidateStored(); !errors.Is(err, ErrOptionInvalid) {
		t.Fatalf("ValidateStored() error = %v, want ErrOptionInvalid", err)
	}
	if items[0].ID != "" {
		t.Fatalf("ValidateStored() repaired corrupt item: %#v", items[0])
	}
}

func TestOptionValidationRejectsAmbiguousAndUnsafeItems(t *testing.T) {
	tests := []struct {
		name  string
		items OptionItems
	}{
		{
			name: "duplicate key",
			items: OptionItems{
				{ID: "one", Key: "same", Label: "One", Value: "1"},
				{ID: "two", Key: "same", Label: "Two", Value: "2"},
			},
		},
		{
			name: "duplicate value",
			items: OptionItems{
				{ID: "one", Key: "one", Label: "One", Value: "same"},
				{ID: "two", Key: "two", Label: "Two", Value: "same"},
			},
		},
		{
			name:  "null item",
			items: OptionItems{nil},
		},
		{
			name: "prototype key",
			items: OptionItems{
				{ID: "one", Key: "one", Label: "One", Value: "1", Extra: map[string]any{"__proto__": "unsafe"}},
			},
		},
		{
			name: "unsupported extra",
			items: OptionItems{
				{ID: "one", Key: "one", Label: "One", Value: "1", Extra: map[string]any{"channel": make(chan int)}},
			},
		},
		{
			name: "oversized value",
			items: OptionItems{
				{ID: "one", Key: "one", Label: "One", Value: strings.Repeat("x", MaxOptionItemValueLength+1)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := &Option{
				Category: "system",
				Name:     "status",
				Status:   enum.Enabled,
				Version:  1,
				Items:    &test.items,
			}
			if err := option.NormalizeAndValidate(); !errors.Is(err, ErrOptionInvalid) {
				t.Fatalf("NormalizeAndValidate() error = %v, want ErrOptionInvalid", err)
			}
		})
	}
}

func TestOptionItemsScanSupportsDatabaseRepresentations(t *testing.T) {
	for _, value := range []any{
		`[{"id":"one","key":"active","label":"Active","value":"enabled"}]`,
		[]byte(`[{"id":"one","key":"active","label":"Active","value":"enabled"}]`),
	} {
		var items OptionItems
		if err := items.Scan(value); err != nil {
			t.Fatalf("Scan(%T) error: %v", value, err)
		}
		if len(items) != 1 || items[0].Key != "active" {
			t.Fatalf("Scan(%T) = %#v", value, items)
		}
	}
	items := OptionItems{{ID: "stale"}}
	if err := items.Scan(nil); err != nil || items != nil {
		t.Fatalf("Scan(nil) = (%#v, %v), want (nil, nil)", items, err)
	}
	if err := items.Scan(42); !errors.Is(err, ErrOptionInvalid) {
		t.Fatalf("Scan(int) error = %v, want ErrOptionInvalid", err)
	}
}
