package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 22:02:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 22:02:39
 */

type User struct {
	actions.ModelGorm `json:",inline"`
	UserLogin         `json:",inline"`
	Name              string              `json:"name"`
	Avatar            string              `json:"avatar"`
	Email             string              `json:"email"`
	Job               string              `json:"job"`
	JobName           string              `json:"jobName" gorm:"-"`
	Organization      string              `json:"organization"`
	OrganizationName  string              `json:"organizationName" gorm:"-"`
	Location          string              `json:"location"`
	LocationName      string              `json:"locationName" gorm:"-"`
	Introduction      string              `json:"introduction"`
	PersonalWebsite   string              `json:"personalWebsite"`
	Verified          bool                `json:"verified"`
	PhoneNumber       string              `json:"phoneNumber"`
	AccountID         string              `json:"accountId"`
	RegistrationTime  time.Time           `json:"registrationTime"`
	Permissions       map[string][]string `json:"permissions" gorm:"-"`
}

func (e *User) BeforeCreate(_ *gorm.DB) error {
	_, err := e.PrepareID(nil)
	return err
}

func (*User) TableName() string {
	return "users"
}

type UserLogin struct {
	actions.ModelGorm
	TenantID     string `json:"tenantID"`
	RoleID       string `json:"roleId"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Password     string `json:"password" gorm:"-"`
	PasswordHash string `json:"-" gorm:"size:255;comment:密码hash"`
	Salt         string `json:"-" gorm:"size:255;comment:加盐"`
}

func (e *UserLogin) TableName() string {
	return "users"
}

func (e *UserLogin) GetUserID() string {
	return e.ID
}

func (e *UserLogin) GetTenantID() string {
	return e.TenantID
}

func (e *UserLogin) GetRoleID() string {
	return e.RoleID
}

func (e *UserLogin) GetEmail() string {
	return e.Email
}

func (e *UserLogin) GetUsername() string {
	return e.Username
}

// Verify verify password
func (e *UserLogin) Verify(password string) (bool, security.Verifier, error) {
	verify, err := security.SetPassword(password, e.Salt)
	if err != nil {
		return false, nil, err
	}
	return verify == e.PasswordHash, e, nil
}
