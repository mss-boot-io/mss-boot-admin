package center

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/pkg/config"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"github.com/mss-boot-io/mss-boot/virtual/model"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/8 09:46:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/8 09:46:12
 */

type Center interface {
	NoticeImp
	TenantImp
	UserImp
	VirtualModelImp
	ConfigImp
	server.Manager
}

type NoticeImp interface {
	List(ctx *gin.Context, userID string, page, size int) ([]NoticeImp, int, error)
	Unread(ctx *gin.Context, userID string) ([]NoticeImp, error)
	Read(ctx *gin.Context, userID string, ids []string) error
	Send(ctx *gin.Context, userID string, noticer NoticeImp) error
}

type TenantImp interface {
	Scope(ctx *gin.Context, table schema.Tabler) func(db *gorm.DB) *gorm.DB
	GetTenant(ctx *gin.Context) (TenantImp, error)
	GetDB(ctx *gin.Context, table schema.Tabler) *gorm.DB
	GetID() any
}

type VirtualModelImp interface {
	GetModels(ctx *gin.Context) ([]VirtualModelImp, error)
	Make() *model.Model
	GetKey() string
}

type UserImp interface {
	security.Verifier
}

type ConfigImp interface {
	config.Entity
	Init(...source.Option)
}