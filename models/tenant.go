package models

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/8 13:42:44
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/8 13:42:44
 */

var (
	data map[string]*Tenant
	mux  sync.RWMutex
)

type AdminUser struct {
	Username string `gorm:"-" json:"username"`
	Password string `gorm:"-" json:"password"`
	Email    string `gorm:"-" json:"email"`
}

type Tenant struct {
	actions.ModelGorm
	Name      string          `gorm:"column:name;type:varchar(255);not null;comment:租户名称" json:"name"`
	Remark    string          `gorm:"column:remark;type:varchar(255);not null;comment:备注" json:"remark"`
	Default   bool            `gorm:"column:default;type:tinyint(1);default:0;comment:是否是默认租户;->" json:"default"`
	Domains   []*TenantDomain `gorm:"foreignKey:TenantID;references:ID" json:"domains"`
	Status    enum.Status     `gorm:"column:status;type:varchar(10);not null;default:enabled;comment:状态" json:"status"`
	Expire    *time.Time      `gorm:"column:expire;type:datetime;comment:过期时间" json:"expire"`
	AdminUser `gorm:"-" json:",inline"`
}

func (*Tenant) TableName() string {
	return "mss_boot_tenants"
}

type TenantDomain struct {
	actions.ModelGorm
	TenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index;comment:租户ID" json:"tenantId"`
	Name     string `gorm:"column:name;type:varchar(255);not null;index;comment:名称" json:"name"`
	Domain   string `gorm:"column:domain;type:varchar(255);not null;index;comment:域名" json:"domain"`
}

func (*TenantDomain) TableName() string {
	return "mss_boot_tenant_domains"
}

func (t *Tenant) BeforeSave(_ *gorm.DB) error {
	if len(t.Domains) == 0 || t.Domains[0].Domain == "" {
		return fmt.Errorf("tenant domain is empty")
	}
	return nil
}

func (t *Tenant) AfterSave(tx *gorm.DB) error {
	return InitTenant(tx)
}

func (t *Tenant) AfterCreate(tx *gorm.DB) error {
	err := InitTenant(tx)
	if err != nil {
		return err
	}
	// todo create tenant data
	return t.Migrate(center.GetTenant().GetDB(nil, nil))
}

func (t *Tenant) AfterDelete(tx *gorm.DB) error {
	//todo delete tenant data
	return InitTenant(tx)
}

func (t *Tenant) GetID() any {
	return t.ID
}

func InitTenant(tx *gorm.DB) error {
	list := make([]*Tenant, 0)
	err := tx.Model(&Tenant{}).Preload("Domains").
		Where("status = ?", enum.Enabled).Find(&list).Error
	if err != nil {
		return err
	}
	mux.Lock()
	defer mux.Unlock()
	data = make(map[string]*Tenant)
	for i := range list {
		for j := range list[i].Domains {
			data[list[i].Domains[j].Domain] = list[i]
		}
	}
	return nil
}

func (t *Tenant) GetTenant(ctx *gin.Context) (center.TenantImp, error) {
	u, err := url.Parse(ctx.GetHeader("Referer"))
	if err != nil {
		return nil, err
	}
	tenant, ok := data[u.Host]
	if !ok || tenant == nil {
		return nil, fmt.Errorf("not found tenant for domain %s", u.Host)
	}
	if tenant.Expire == nil || tenant.Expire.Before(time.Now()) {
		return nil, fmt.Errorf("tenant %s is expired", tenant.Name)
	}
	return tenant, nil
}

func (t *Tenant) GetDB(ctx *gin.Context, table schema.Tabler) *gorm.DB {
	if ctx == nil {
		return gormdb.DB
	}
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		return gormdb.DB.WithContext(ctx).Scopes(t.Scope(ctx, table))
	}
	return gormdb.DB.WithContext(ctx).Scopes(t.Scope(ctx, table), verify.(*User).Scope(ctx, table))
}

func (t *Tenant) GetMigrateDB(ctx *gin.Context, tx *gorm.DB, table schema.Tabler) *gorm.DB {
	if ctx == nil {
		return tx
	}
	return tx.WithContext(ctx).Scopes(t.Scope(ctx, table))
}

func (t *Tenant) Scope(ctx *gin.Context, table schema.Tabler) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if ctx == nil || table == nil || !pkg.SupportMultiTenant(table) {
			return db
		}
		tenant, err := t.GetTenant(ctx)
		query := fmt.Sprintf("`%s`.`tenant_id` = ?", table.TableName())
		if err != nil {
			slog.Error("get tenant error", "error", err)
			_ = db.AddError(err)
			return db
		}
		db = db.Where(query, tenant.GetID())
		if pkg.SupportCreator(table) {
			verify := middleware.GetVerify(ctx)
			if verify == nil {
				return db
			}
			db = db.Where(pkg.GetCreatorField(), verify.GetUserID())
		}
		return db
	}
}

// TenantIDScope get tenant id scope
func TenantIDScope(ctx *gin.Context) (any, error) {
	tenant, err := center.Default.GetTenant().GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	return tenant.GetID(), nil
}
