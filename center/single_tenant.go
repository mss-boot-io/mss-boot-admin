package center

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type SingleTenant struct{}

func (s *SingleTenant) Scope(_ *gin.Context, _ schema.Tabler) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db
	}
}

func (s *SingleTenant) GetTenant(_ *gin.Context) (TenantImp, error) {
	return s, nil
}

func (s *SingleTenant) GetDB(_ *gin.Context, _ schema.Tabler) *gorm.DB {
	return gormdb.DB
}

func (s *SingleTenant) GetID() any {
	return "default"
}

func (s *SingleTenant) GetDefault() bool {
	return true
}
