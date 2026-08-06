package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/11 13:48:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/11 13:48:02
 */

type UserOAuth2 struct {
	ModelGormTenant
	User                *User             `json:"user" gorm:"foreignKey:UserID;references:ID" swaggerignore:"true"`
	UserID              string            `json:"user_id" gorm:"size:64"`
	IdentityKey         *string           `json:"-" gorm:"column:identity_key;size:96;uniqueIndex:ux_user_oauth2_identity_key" swaggerignore:"true"`
	OpenID              string            `json:"openID" gorm:"size:64"`
	UnionID             string            `json:"unionID" gorm:"column:union_id;size:64"`
	Sub                 string            `json:"sub" gorm:"size:255;comment:主题"`
	Name                string            `json:"name" gorm:"size:255;comment:名称"`
	GivenName           string            `json:"given_name" gorm:"size:255;comment:名"`
	FamilyName          string            `json:"family_name" gorm:"size:255;comment:姓"`
	MiddleName          string            `json:"middle_name" gorm:"size:255;comment:中间名"`
	NickName            string            `json:"nickname" gorm:"size:255;comment:昵称"`
	PreferredUsername   string            `json:"preferred_username" gorm:"size:255;comment:首选用户名"`
	Profile             string            `json:"profile" gorm:"size:255;comment:个人资料"`
	Picture             string            `json:"picture" gorm:"size:255;comment:图片"`
	Website             string            `json:"website" gorm:"size:255;comment:网站"`
	Email               string            `json:"email" gorm:"size:255;comment:邮箱"`
	EmailVerified       bool              `json:"email_verified" gorm:"default:false;comment:邮箱是否验证"`
	Gender              string            `json:"gender" gorm:"size:255;comment:性别"`
	Birthdata           string            `json:"birthdata" gorm:"size:255;comment:出生日期"`
	Zoneinfo            string            `json:"zoneinfo" gorm:"size:255;comment:时区"`
	Locale              string            `json:"locale" gorm:"size:255;comment:语言"`
	PhoneNumber         string            `json:"phone_number" gorm:"size:255;comment:手机号"`
	PhoneNumberVerified bool              `json:"phone_number_verified" gorm:"default:false;comment:手机号是否验证"`
	Address             string            `json:"address" gorm:"size:255;comment:地址"`
	EmployeeNO          string            `json:"employee_no" gorm:"column:employee_no;size:255;comment:员工编号"`
	Provider            pkg.LoginProvider `json:"type" gorm:"size:20;comment:登录类型"`
}

func (*UserOAuth2) TableName() string {
	return "mss_boot_user_oauth2"
}

// UserOAuthIdentityKey returns the canonical, provider-scoped identity used
// for database uniqueness. Provider identifiers are opaque and therefore are
// trimmed but never case-folded.
func UserOAuthIdentityKey(provider pkg.LoginProvider, openID, unionID string) (string, error) {
	provider = pkg.LoginProvider(strings.ToLower(strings.TrimSpace(provider.String())))
	var identity string
	switch provider {
	case pkg.GithubLoginProvider:
		identity = strings.TrimSpace(openID)
	case pkg.LarkLoginProvider:
		identity = strings.TrimSpace(unionID)
	default:
		return "", fmt.Errorf("oauth identity provider %q is unsupported", provider)
	}
	if identity == "" {
		return "", fmt.Errorf("oauth %s identity is missing", provider)
	}
	if len(identity) > 64 {
		return "", fmt.Errorf("oauth %s identity exceeds 64 bytes", provider)
	}
	return provider.String() + ":" + identity, nil
}

// BeforeSave keeps the nullable storage column canonical for all new writes.
// NULL is reserved for soft-deleted legacy rows during migration; an active
// create or update without its provider-specific identifier fails closed.
func (e *UserOAuth2) BeforeSave(*gorm.DB) error {
	if e == nil {
		return errors.New("oauth identity is nil")
	}
	provider := pkg.LoginProvider(strings.ToLower(strings.TrimSpace(e.Provider.String())))
	key, err := UserOAuthIdentityKey(provider, e.OpenID, e.UnionID)
	if err != nil {
		return err
	}
	e.Provider = provider
	switch provider {
	case pkg.GithubLoginProvider:
		e.OpenID = strings.TrimSpace(e.OpenID)
	case pkg.LarkLoginProvider:
		e.UnionID = strings.TrimSpace(e.UnionID)
	}
	e.IdentityKey = &key
	return nil
}
