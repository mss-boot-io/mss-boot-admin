package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/1 11:57:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/1 11:57:51
 */

type OptionItem struct {
	ID    string         `json:"id"`
	Key   string         `json:"key"`
	Label string         `json:"label"`
	Value string         `json:"value"`
	Color string         `json:"color"`
	Sort  int            `json:"sort"`
	Icon  string         `json:"icon,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

type OptionItems []*OptionItem

const (
	MaxOptionItems             = 1000
	MaxOptionItemsEncodedSize  = 256 * 1024
	MaxOptionCategoryLength    = 50
	MaxOptionNameLength        = 255
	MaxOptionDisplayNameLength = 255
	MaxOptionDescriptionLength = 4096
	MaxOptionRemarkLength      = 255
	MaxOptionItemKeyLength     = 128
	MaxOptionItemLabelLength   = 255
	MaxOptionItemValueLength   = 1024
	MaxOptionItemColorLength   = 64
	MaxOptionItemIconLength    = 512
	MaxOptionItemIDLength      = 64
	MaxOptionItemSort          = 1_000_000
	MaxOptionExtraDepth        = 8
	MaxOptionExtraEntries      = 128
)

var ErrOptionInvalid = errors.New("option is invalid")

func (o *OptionItems) Value() (driver.Value, error) {
	if o == nil {
		return nil, nil
	}
	for i := range *o {
		if (*o)[i] != nil && strings.TrimSpace((*o)[i].ID) == "" {
			(*o)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(o)
}

func (o *OptionItems) Scan(val any) error {
	if val == nil {
		*o = nil
		return nil
	}
	var data []byte
	switch value := val.(type) {
	case []byte:
		data = value
	case string:
		data = []byte(value)
	default:
		return fmt.Errorf("%w: unsupported option items database type %T", ErrOptionInvalid, val)
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*o = nil
		return nil
	}
	if err := json.Unmarshal(data, o); err != nil {
		return fmt.Errorf("%w: decode option items: %v", ErrOptionInvalid, err)
	}
	return nil
}

type Option struct {
	ModelGormTenant
	// Category 选项分类
	Category string `json:"category" gorm:"column:category;type:varchar(50);not null;index:idx_category;comment:选项分类"`
	// DisplayName 显示名称
	DisplayName string `json:"displayName" gorm:"column:display_name;type:varchar(255);comment:显示名称"`
	// Description 描述
	Description string `json:"description" gorm:"column:description;type:text;comment:描述"`
	// Name 选项名称
	Name string `json:"name" gorm:"column:name;type:varchar(255);not null;unique_index:idx_name;comment:选项名称"`
	// Remark 备注
	Remark string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:备注"`
	// Items 选项内容
	Items *OptionItems `json:"items" gorm:"column:items;type:json;comment:选项内容"`
	// Status 状态
	Status enum.Status `json:"status" gorm:"column:status;comment:状态;size:10"`
	// Version 版本号
	Version int `json:"version" gorm:"column:version;type:int;default:1;comment:版本号"`
	// BuiltIn 是否内置
	BuiltIn bool `json:"builtIn" gorm:"column:built_in;type:boolean;default:false;comment:是否内置"`
}

// NormalizeAndValidate canonicalizes the client-owned option surface and
// assigns item identifiers when needed. Record identity, version and built-in
// status remain service-owned.
func (o *Option) NormalizeAndValidate() error {
	if o == nil {
		return fmt.Errorf("%w: option is nil", ErrOptionInvalid)
	}
	o.Category = strings.TrimSpace(o.Category)
	o.Name = strings.TrimSpace(o.Name)
	o.DisplayName = strings.TrimSpace(o.DisplayName)
	o.Description = strings.TrimSpace(o.Description)
	o.Remark = strings.TrimSpace(o.Remark)
	if o.Category == "" {
		o.Category = "system"
	}
	if o.Status == "" {
		o.Status = enum.Enabled
	}
	if o.Version <= 0 {
		o.Version = 1
	}
	if o.Items == nil {
		empty := OptionItems{}
		o.Items = &empty
	}
	for _, item := range *o.Items {
		if item == nil {
			continue
		}
		item.ID = strings.TrimSpace(item.ID)
		item.Key = strings.TrimSpace(item.Key)
		item.Label = strings.TrimSpace(item.Label)
		item.Value = strings.TrimSpace(item.Value)
		item.Color = strings.TrimSpace(item.Color)
		item.Icon = strings.TrimSpace(item.Icon)
		if item.ID == "" {
			item.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return o.validate(false)
}

// ValidateStored checks persisted data without repairing or mutating it. API
// reads fail closed when a stored dictionary violates the bounded contract.
func (o *Option) ValidateStored() error {
	return o.validate(true)
}

func (o *Option) validate(requireID bool) error {
	if o == nil {
		return fmt.Errorf("%w: option is nil", ErrOptionInvalid)
	}
	if requireID && strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("%w: option ID is empty", ErrOptionInvalid)
	}
	if o.Category == "" || o.Category != strings.TrimSpace(o.Category) || utf8.RuneCountInString(o.Category) > MaxOptionCategoryLength {
		return fmt.Errorf("%w: category is invalid", ErrOptionInvalid)
	}
	if o.Name == "" || o.Name != strings.TrimSpace(o.Name) || utf8.RuneCountInString(o.Name) > MaxOptionNameLength {
		return fmt.Errorf("%w: name is invalid", ErrOptionInvalid)
	}
	if o.DisplayName != strings.TrimSpace(o.DisplayName) || utf8.RuneCountInString(o.DisplayName) > MaxOptionDisplayNameLength {
		return fmt.Errorf("%w: display name is invalid", ErrOptionInvalid)
	}
	if o.Description != strings.TrimSpace(o.Description) || utf8.RuneCountInString(o.Description) > MaxOptionDescriptionLength {
		return fmt.Errorf("%w: description is invalid", ErrOptionInvalid)
	}
	if o.Remark != strings.TrimSpace(o.Remark) || utf8.RuneCountInString(o.Remark) > MaxOptionRemarkLength {
		return fmt.Errorf("%w: remark is invalid", ErrOptionInvalid)
	}
	if o.Status != enum.Enabled && o.Status != enum.Disabled {
		return fmt.Errorf("%w: status is invalid", ErrOptionInvalid)
	}
	if o.Version <= 0 {
		return fmt.Errorf("%w: version is invalid", ErrOptionInvalid)
	}
	if o.Items == nil {
		return fmt.Errorf("%w: items are missing", ErrOptionInvalid)
	}
	if len(*o.Items) > MaxOptionItems {
		return fmt.Errorf("%w: too many items", ErrOptionInvalid)
	}
	ids := make(map[string]struct{}, len(*o.Items))
	keys := make(map[string]struct{}, len(*o.Items))
	values := make(map[string]struct{}, len(*o.Items))
	for index, item := range *o.Items {
		if item == nil {
			return fmt.Errorf("%w: item %d is nil", ErrOptionInvalid, index)
		}
		if item.ID == "" || item.ID != strings.TrimSpace(item.ID) || utf8.RuneCountInString(item.ID) > MaxOptionItemIDLength {
			return fmt.Errorf("%w: item %d ID is invalid", ErrOptionInvalid, index)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("%w: duplicate item ID", ErrOptionInvalid)
		}
		ids[item.ID] = struct{}{}
		if item.Key == "" || item.Key != strings.TrimSpace(item.Key) || utf8.RuneCountInString(item.Key) > MaxOptionItemKeyLength {
			return fmt.Errorf("%w: item %d key is invalid", ErrOptionInvalid, index)
		}
		if _, exists := keys[item.Key]; exists {
			return fmt.Errorf("%w: duplicate item key", ErrOptionInvalid)
		}
		keys[item.Key] = struct{}{}
		if item.Label == "" || item.Label != strings.TrimSpace(item.Label) || utf8.RuneCountInString(item.Label) > MaxOptionItemLabelLength {
			return fmt.Errorf("%w: item %d label is invalid", ErrOptionInvalid, index)
		}
		if item.Value == "" || item.Value != strings.TrimSpace(item.Value) || utf8.RuneCountInString(item.Value) > MaxOptionItemValueLength {
			return fmt.Errorf("%w: item %d value is invalid", ErrOptionInvalid, index)
		}
		if _, exists := values[item.Value]; exists {
			return fmt.Errorf("%w: duplicate item value", ErrOptionInvalid)
		}
		values[item.Value] = struct{}{}
		if item.Color != strings.TrimSpace(item.Color) || utf8.RuneCountInString(item.Color) > MaxOptionItemColorLength {
			return fmt.Errorf("%w: item %d color is invalid", ErrOptionInvalid, index)
		}
		if item.Icon != strings.TrimSpace(item.Icon) || utf8.RuneCountInString(item.Icon) > MaxOptionItemIconLength {
			return fmt.Errorf("%w: item %d icon is invalid", ErrOptionInvalid, index)
		}
		if item.Sort < -MaxOptionItemSort || item.Sort > MaxOptionItemSort {
			return fmt.Errorf("%w: item %d sort is invalid", ErrOptionInvalid, index)
		}
		if err := validateOptionExtra(item.Extra); err != nil {
			return fmt.Errorf("%w: item %d extra: %v", ErrOptionInvalid, index, err)
		}
	}
	payload, err := json.Marshal(o.Items)
	if err != nil {
		return fmt.Errorf("%w: encode items: %v", ErrOptionInvalid, err)
	}
	if len(payload) > MaxOptionItemsEncodedSize {
		return fmt.Errorf("%w: items payload is too large", ErrOptionInvalid)
	}
	return nil
}

func validateOptionExtra(extra map[string]any) error {
	if extra == nil {
		return nil
	}
	payload, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return err
	}
	return validateOptionJSONValue(normalized, 0)
}

func validateOptionJSONValue(value any, depth int) error {
	if depth > MaxOptionExtraDepth {
		return errors.New("nested value is too deep")
	}
	switch typed := value.(type) {
	case nil, bool, string:
		return nil
	case json.Number:
		number, err := typed.Float64()
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return errors.New("number is invalid")
		}
		return nil
	case []any:
		if len(typed) > MaxOptionExtraEntries {
			return errors.New("array has too many entries")
		}
		for _, item := range typed {
			if err := validateOptionJSONValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > MaxOptionExtraEntries {
			return errors.New("object has too many entries")
		}
		for key, item := range typed {
			if key == "" || utf8.RuneCountInString(key) > MaxOptionItemKeyLength || key == "__proto__" || key == "constructor" || key == "prototype" {
				return errors.New("object key is invalid")
			}
			if err := validateOptionJSONValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
}

func (*Option) TableName() string {
	return "mss_boot_options"
}
