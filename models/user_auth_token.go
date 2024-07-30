package models

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/security"

	"github.com/mss-boot-io/mss-boot-admin/middleware"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/30 09:45:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/30 09:45:03
 */

type UserAuthToken struct {
	ModelGormTenant
	UserID    string    `gorm:"type:varchar(64);index;comment:用户ID" json:"userID"`
	Token     string    `gorm:"type:text;uniqueIndex;comment:token" json:"token"`
	ExpiredAt time.Time `gorm:"index;comment:过期时间" json:"expiredAt"`
	Revoked   bool      `gorm:"type:tinyint(1);index;comment:是否撤销" json:"revoked"`
}

func (*UserAuthToken) TableName() string {
	return "mss_boot_user_auth_token"
}

// GenerateUserAuthToken 生成用户令牌
func GenerateUserAuthToken(ctx *gin.Context, verify security.Verifier, validityPeriod time.Duration) (*UserAuthToken, error) {
	if validityPeriod <= 0 {
		validityPeriod = 100 * 12 * 30 * 24 * time.Hour
	}
	auth := *middleware.Auth
	auth.Timeout = validityPeriod
	verify.SetRefreshTokenDisable(true)
	token, expire, err := auth.TokenGenerator(verify)
	userAuthToken := &UserAuthToken{
		UserID:    verify.GetUserID(),
		Token:     token,
		ExpiredAt: expire,
	}
	err = center.GetDB(ctx, &UserAuthToken{}).Create(userAuthToken).Error
	if err != nil {
		return nil, err
	}
	return userAuthToken, nil
}
