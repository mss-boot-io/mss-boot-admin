package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 23:09:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 23:09:49
 */

type ArrayString []string

func (a *ArrayString) Scan(val any) error {
	s := val.([]uint8)
	ss := strings.Split(string(s), "|")
	*a = ss
	return nil
}

func (a *ArrayString) Value() (driver.Value, error) {
	return strings.Join(*a, "|"), nil

}

type Metadata map[string]string

func (m *Metadata) Scan(val any) error {
	s := val.([]uint8)
	return json.Unmarshal(s, m)
}

func (m *Metadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// ModelGormTenant model gorm support multi tenant
type ModelGormTenant struct {
	actions.ModelGorm
	// TenantID tenant id
	TenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index;comment:租户ID" json:"tenantID"`
}

func (e *ModelGormTenant) BeforeCreate(tx *gorm.DB) (err error) {
	_, err = e.PrepareID(nil)
	if e.TenantID != "" {
		return nil
	}
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return fmt.Errorf("not gin context")
	}
	tenant, err := center.Default.GetTenant().GetTenant(ctx)
	if err != nil {
		return err
	}
	// tenantID Can only be assigned at creation time
	e.TenantID = tenant.GetID().(string)
	return err
}

func (e *ModelGormTenant) BeforeDelete(tx *gorm.DB) error {
	if e.TenantID != "" {
		return nil
	}
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return fmt.Errorf("not gin context")
	}
	tenant, err := center.Default.GetTenant().GetTenant(ctx)
	if err != nil {
		return err
	}
	tx = tx.Where("tenant_id = ?", tenant.GetID())
	return nil
}
