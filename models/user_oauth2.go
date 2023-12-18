package models

import "github.com/mss-boot-io/mss-boot/pkg/response/actions"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/11 13:48:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/11 13:48:02
 */

type UserOAuth2 struct {
	actions.ModelGorm
	UserID        string      `json:"user_id" gorm:"size:64"`
	Iss           string      `json:"iss" gorm:"size:255;comment:发行人"`
	Sub           string      `json:"sub" gorm:"size:255;comment:主题"`
	Aud           string      `json:"aud" gorm:"size:255;comment:受众"`
	Email         string      `json:"email" gorm:"size:255;comment:邮箱"`
	EmailVerified bool        `json:"email_verified;default:false;comment:邮箱是否验证"`
	Groups        ArrayString `json:"groups" gorm:"type:text;comment:用户组"`
	Name          string      `json:"name" gorm:"size:255;comment:名称"`
	Provider      string      `json:"provider" gorm:"size:50;comment:oauth2提供者"`
}

func (*UserOAuth2) TableName() string {
	return "mss_boot_user_oauth2"
}
