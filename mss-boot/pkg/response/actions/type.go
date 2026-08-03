package actions

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/4/19 01:00:40
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/4/19 01:00:40
 */

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ModelProvider model provider
type ModelProvider string

const (
	// ModelProviderMgm mgm model provider
	ModelProviderMgm ModelProvider = "mgm"
	// ModelProviderGorm gorm model provider
	ModelProviderGorm ModelProvider = "gorm"
	ModelProviderK8S  ModelProvider = "k8s"
)

func (p ModelProvider) String() string {
	return string(p)
}

// Model gorm and mgm model
type Model interface {
	mgm.Model
	schema.Tabler
}

// ModelGorm model gorm
type ModelGorm struct {
	// ID primary key
	ID string `gorm:"primaryKey;column:id;type:varchar(64);not null;comment:ID" json:"id" bson:"_id,omitempty" form:"id" query:"id"`
	// CreatedAt create time
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	// UpdatedAt update time
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
	// DeletedAt delete time soft delete
	DeletedAt gorm.DeletedAt `gorm:"index" bson:"-" json:"-"`
}

func (e *ModelGorm) BeforeCreate(_ *gorm.DB) (err error) {
	_, err = e.PrepareID(nil)
	return err
}

// PrepareID prepare id
func (e *ModelGorm) PrepareID(_ any) (any, error) {
	if e.ID == "" {
		e.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return e.ID, nil
}

// GetID get id
func (e *ModelGorm) GetID() any {
	return e.ID
}

// SetID set id
func (e *ModelGorm) SetID(id any) {
	e.ID = cast.ToString(id)
}

// TableName return table name
func (e *ModelGorm) TableName() string {
	return "mss-boot-base"
}
