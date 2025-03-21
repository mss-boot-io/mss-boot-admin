package models

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
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
	Token     string    `gorm:"type:text;comment:token" json:"token"`
	ExpiredAt time.Time `gorm:"index;comment:过期时间" json:"expiredAt"`
	Revoked   bool      `gorm:"index;comment:是否撤销" json:"revoked"`
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
	auth.TimeoutFunc = func(_ interface{}) time.Duration {
		return validityPeriod
	}
	userAuthToken := &UserAuthToken{
		UserID: verify.GetUserID(),
	}
	userAuthToken.ID = pkg.SimpleID()
	verify.SetRefreshTokenDisable(true)
	verify.SetPersonAccessToken(userAuthToken.ID)
	var err error
	userAuthToken.Token, userAuthToken.ExpiredAt, err = auth.TokenGenerator(verify)
	if err != nil {
		return nil, err
	}
	err = center.GetDB(ctx, &UserAuthToken{}).Create(userAuthToken).Error
	if err != nil {
		return nil, err
	}
	return userAuthToken, nil
}
