package models

import (
	"fmt"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"time"

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
	err := e.ModelGorm.BeforeCreate(nil)
	if err != nil {
		return err
	}
	e.Salt = security.GenerateRandomKey6()
	hash, err := security.SetPassword(e.Password, e.Salt)
	if err != nil {
		return err
	}
	e.RegistrationTime = time.Now()
	e.PasswordHash = hash
	return err
}

func (e *User) AfterFind(tx *gorm.DB) error {
	fmt.Println("AfterFind", e.ID, e.Username, e.Password, e.PasswordHash, e.Salt)
	e.Permissions = map[string][]string{
		"menu.role.serach": {"*"},
	}
	return nil
}

func (*User) TableName() string {
	return "mss_boot_users"
}

func (e *User) GetUserID() string {
	return e.ID
}

// GetUserByUsername get user by username
func GetUserByUsername(username string) (*User, error) {
	var user User
	err := gormdb.DB.Model(&user).First(&user, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type UserLogin struct {
	RoleID       string      `json:"roleId" gorm:"index"`
	Username     string      `json:"username" gorm:"size:100;uniqueIndex"`
	Email        string      `json:"email"`
	Password     string      `json:"password,omitempty" gorm:"-"`
	PasswordHash string      `json:"-" gorm:"size:255;comment:密码hash"`
	Salt         string      `json:"-" gorm:"size:255;comment:加盐"`
	Status       enum.Status `json:"status"`
}

func (e *UserLogin) TableName() string {
	return "mss_boot_users"
}

func (e *UserLogin) GetUserID() string {
	return e.Username
}

func (e *UserLogin) GetTenantID() string {
	return ""
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
func (e *UserLogin) Verify() (bool, security.Verifier, error) {
	user, err := GetUserByUsername(e.Username)
	if err != nil {
		return false, nil, err
	}
	verify, err := security.SetPassword(e.Password, user.Salt)
	if err != nil {
		return false, nil, err
	}
	return verify == user.PasswordHash, user, nil
}
