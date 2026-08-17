package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

const (
	MaxLanguageDefinitions     = 1000
	MaxLanguageDefinitionsSize = 256 * 1024
	MaxLanguageDefineGroup     = 64
	MaxLanguageDefineKey       = 256
	MaxLanguageDefineValue     = 4096
)

var ErrLanguageInvalid = errors.New("language is invalid")

type LanguageDefine struct {
	ID    string `json:"id"`
	Group string `json:"group" gorm:"column:group;comment:分组;type:varchar(20);not null" binding:"required"`
	Key   string `json:"key" gorm:"column:key;comment:键;type:varchar(20);not null" binding:"required"`
	Value string `json:"value" gorm:"column:value;comment:值;type:varchar(100);not null" binding:"required"`
}

type Language struct {
	ModelGormTenant
	Name    string           `json:"name" gorm:"column:name;comment:名称;type:varchar(255);not null;uniqueIndex" binding:"required"`
	Remark  string           `json:"remark" gorm:"column:remark;comment:备注;type:varchar(255);not null"`
	Status  enum.Status      `json:"status" gorm:"column:status;comment:状态;size:10"`
	Defines *LanguageDefines `json:"defines,omitempty" gorm:"column:defines;comment:定义;type:json"`
}

func (*Language) TableName() string {
	return "mss_boot_languages"
}

func (l *Language) BeforeCreate(tx *gorm.DB) error {
	if err := l.ModelGormTenant.BeforeCreate(tx); err != nil {
		return err
	}
	if l.Status == enum.Unknown {
		l.Status = enum.Enabled
	}
	return l.NormalizeAndValidate()
}

func (l *Language) BeforeUpdate(_ *gorm.DB) error {
	return l.NormalizeAndValidate()
}

func (l *Language) NormalizeAndValidate() error {
	return l.normalizeAndValidate(true)
}

// ValidateStored verifies that an existing row is already canonical and
// bounded. Unlike NormalizeAndValidate it never repairs response data in
// memory, so a corrupt record cannot yield unstable, ephemeral identifiers.
func (l *Language) ValidateStored() error {
	return l.normalizeAndValidate(false)
}

func (l *Language) normalizeAndValidate(normalize bool) error {
	name := strings.TrimSpace(l.Name)
	if name == "" || utf8.RuneCountInString(name) > 255 {
		return fmt.Errorf("%w: name is required", ErrLanguageInvalid)
	}
	// x/text also accepts the underscore-separated Unicode locale identifier
	// extension. The public contract is deliberately strict BCP 47, so reject
	// that compatibility spelling before canonicalization.
	if strings.ContainsRune(name, '_') {
		return fmt.Errorf("%w: name must be a valid BCP 47 locale tag", ErrLanguageInvalid)
	}
	tag, err := language.Parse(name)
	if err != nil || tag == language.Und {
		return fmt.Errorf("%w: name must be a valid BCP 47 locale tag", ErrLanguageInvalid)
	}
	canonicalName := tag.String()
	if normalize {
		l.Name = canonicalName
	} else if l.Name != name {
		// Pre-existing, parseable tags may use non-canonical casing. Reads do
		// not rewrite persisted data; the next explicit write canonicalizes the
		// value without allowing hidden surrounding whitespace.
		return fmt.Errorf("%w: stored name contains surrounding whitespace", ErrLanguageInvalid)
	}
	if !normalize && (strings.TrimSpace(l.ID) == "" || len(l.ID) > 64) {
		return fmt.Errorf("%w: stored ID is invalid", ErrLanguageInvalid)
	}
	if utf8.RuneCountInString(l.Remark) > 255 {
		return fmt.Errorf("%w: remark is too long", ErrLanguageInvalid)
	}
	if l.Status != enum.Enabled && l.Status != enum.Disabled {
		return fmt.Errorf("%w: status must be enabled or disabled", ErrLanguageInvalid)
	}
	if l.Defines == nil {
		return nil
	}
	if len(*l.Defines) > MaxLanguageDefinitions {
		return fmt.Errorf("%w: too many definitions", ErrLanguageInvalid)
	}
	definitionIDs := make(map[string]struct{}, len(*l.Defines))
	definitionKeys := make(map[string]struct{}, len(*l.Defines))
	for index, definition := range *l.Defines {
		if definition == nil {
			return fmt.Errorf("%w: definition %d is null", ErrLanguageInvalid, index)
		}
		definitionID := strings.TrimSpace(definition.ID)
		group := strings.TrimSpace(definition.Group)
		key := strings.TrimSpace(definition.Key)
		if normalize {
			definition.ID = definitionID
			definition.Group = group
			definition.Key = key
			if definition.ID == "" {
				definition.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
			}
			definitionID = definition.ID
		} else if definitionID == "" ||
			definition.ID != definitionID ||
			definition.Group != group ||
			definition.Key != key {
			return fmt.Errorf("%w: stored definition %d is not canonical", ErrLanguageInvalid, index)
		}
		if utf8.RuneCountInString(definitionID) > 64 ||
			utf8.RuneCountInString(group) == 0 ||
			utf8.RuneCountInString(group) > MaxLanguageDefineGroup ||
			utf8.RuneCountInString(key) == 0 ||
			utf8.RuneCountInString(key) > MaxLanguageDefineKey ||
			utf8.RuneCountInString(definition.Value) > MaxLanguageDefineValue {
			return fmt.Errorf("%w: definition %d exceeds field limits", ErrLanguageInvalid, index)
		}
		if _, exists := definitionIDs[definitionID]; exists {
			return fmt.Errorf("%w: definition IDs must be unique", ErrLanguageInvalid)
		}
		definitionIDs[definitionID] = struct{}{}
		compoundKey := group + "\x00" + key
		if _, exists := definitionKeys[compoundKey]; exists {
			return fmt.Errorf("%w: definition group and key must be unique", ErrLanguageInvalid)
		}
		definitionKeys[compoundKey] = struct{}{}
	}
	payload, err := json.Marshal(l.Defines)
	if err != nil {
		return fmt.Errorf("%w: definitions cannot be encoded", ErrLanguageInvalid)
	}
	if len(payload) > MaxLanguageDefinitionsSize {
		return fmt.Errorf("%w: definitions exceed the encoded size limit", ErrLanguageInvalid)
	}
	return nil
}

type LanguageDefines []*LanguageDefine

func (l *LanguageDefines) Scan(val any) error {
	if val == nil {
		*l = nil
		return nil
	}
	var payload []byte
	switch value := val.(type) {
	case []byte:
		payload = value
	case string:
		payload = []byte(value)
	default:
		return fmt.Errorf("scan language definitions: unsupported value %T", val)
	}
	if len(payload) == 0 {
		*l = nil
		return nil
	}
	return json.Unmarshal(payload, l)
}

func (l *LanguageDefines) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	for i := range *l {
		if (*l)[i].ID == "" {
			(*l)[i].ID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
	}
	return json.Marshal(*l)
}
