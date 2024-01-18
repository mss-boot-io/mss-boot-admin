package models

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/center"
	"log/slog"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 22:02:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 22:02:39
 */

type User struct {
	ModelGormTenant
	UserLogin   `json:",inline"`
	Name        string          `json:"name" gorm:"column:name;type:varchar(100)"`
	Avatar      string          `json:"avatar" gorm:"column:avatar;type:varchar(255)"`
	Signature   string          `json:"signature" gorm:"column:signature;type:varchar(255)"`
	Title       string          `json:"title" gorm:"column:title;type:varchar(100)"`
	Group       string          `json:"group" gorm:"column:group;type:varchar(255)"`
	Country     string          `json:"country" gorm:"column:country;type:varchar(20)"`
	Province    string          `json:"province" gorm:"column:province;type:varchar(20)"`
	City        string          `json:"city" gorm:"column:city;type:varchar(20)"`
	Address     string          `json:"address" gorm:"column:address;type:varchar(255)"`
	Phone       string          `json:"phone" gorm:"column:phone;type:varchar(20)"`
	Profile     string          `json:"profile" gorm:"column:profile;type:blob"`
	Tags        ArrayString     `json:"tags"  swaggertype:"array,string" gorm:"type:text"`
	Permissions map[string]bool `json:"permissions" gorm:"-"`
}

type Tag struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func (e *User) BeforeCreate(tx *gorm.DB) error {
	err := e.ModelGormTenant.BeforeCreate(tx)
	if err != nil {
		return err
	}
	e.Salt = security.GenerateRandomKey6()
	hash, err := security.SetPassword(e.Password, e.Salt)
	if err != nil {
		return err
	}
	e.PasswordHash = hash
	return err
}

func (*User) TableName() string {
	return "mss_boot_users"
}

func (e *User) GetUserID() string {
	return e.ID
}

// PasswordReset reset password
func PasswordReset(ctx context.Context, userID string, password string) error {
	user := &User{}
	err := gormdb.DB.WithContext(ctx).First(user, "id = ?", userID).Error
	if err != nil {
		return err
	}
	user.Salt = security.GenerateRandomKey6()
	hash, err := security.SetPassword(password, user.Salt)
	if err != nil {
		return err
	}
	err = gormdb.DB.Model(user).Updates(User{
		UserLogin: UserLogin{
			PasswordHash: hash,
			Salt:         user.Salt,
		},
	}).Error
	if err != nil {
		return err
	}
	return nil
}

// GetUserByUsername get user by username
func GetUserByUsername(username string) (*User, error) {
	var user User
	err := gormdb.DB.Model(&user).Preload("Role").First(&user, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type UserLogin struct {
	RoleID       string        `json:"roleID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Role         *Role         `json:"role" gorm:"foreignKey:RoleID;references:ID"`
	Username     string        `json:"username" gorm:"type:varchar(20);uniqueIndex"`
	Email        string        `json:"email" gorm:"type:varchar(100);uniqueIndex"`
	Password     string        `json:"password,omitempty" gorm:"-"`
	PasswordHash string        `json:"-" gorm:"size:255;comment:密码hash" swaggerignore:"true"`
	Salt         string        `json:"-" gorm:"size:255;comment:加盐" swaggerignore:"true"`
	Status       enum.Status   `json:"status" gorm:"size:10"`
	OAuth2       []*UserOAuth2 `json:"oauth2" gorm:"foreignKey:UserID;references:ID"`
	Provider     string        `json:"type" gorm:"-"`
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

func (e *UserLogin) Root() bool {
	if e.Role == nil {
		return false
	}
	return e.Role.Root
}

// Verify verify password
func (e *UserLogin) Verify(ctx context.Context) (bool, security.Verifier, error) {
	switch strings.ToLower(e.Provider) {
	case "github":
		// get user from github, then add user to db
		// github user
		conf, err := config.Cfg.OAuth2.GetOAuth2Config(ctx)
		if err != nil {
			slog.Error("get oauth2 config error", slog.Any("error", err))
			return false, nil, err
		}
		githubUser, err := pkg.GetUserFromGithub(ctx, conf, e.Password)
		if err != nil {
			slog.Error("get user from github error", slog.Any("error", err))
			return false, nil, err
		}

		if len(config.Cfg.OAuth2.AllowGroup) > 0 {
			org, err := pkg.GetOrganizationsFromGithub(ctx, conf, e.Password)
			if err != nil {
				slog.Error("get organizations from github error", slog.Any("error", err))
				return false, nil, err
			}
			if !pkg.InArray(config.Cfg.OAuth2.AllowGroup, org, "", 0) {
				err = errors.New("user not in allow group")
				slog.Error(err.Error())
				return false, nil, err
			}
		}
		defaultRole := &Role{Default: true}
		_ = center.GetDB(ctx.(*gin.Context), &Role{}).Where(defaultRole).First(defaultRole).Error
		// get user from db
		userOAuth2 := &UserOAuth2{}
		err = center.GetDB(ctx.(*gin.Context), &UserOAuth2{}).Preload("User.Role").First(userOAuth2, "open_id = ?", fmt.Sprintf("%d", githubUser.ID)).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Error("get user from db error", slog.Any("error", err))
				return false, nil, err
			}
			err = nil
			userOAuth2 = &UserOAuth2{
				OpenID:        fmt.Sprintf("%d", githubUser.ID),
				Sub:           "github",
				Name:          githubUser.Login,
				Email:         githubUser.Email,
				Profile:       githubUser.Blog,
				Picture:       githubUser.AvatarURL,
				NickName:      githubUser.Login,
				Website:       githubUser.HTMLURL,
				EmailVerified: true,
				Locale:        githubUser.Location,
				User: &User{
					UserLogin: UserLogin{
						RoleID:   defaultRole.ID,
						Username: githubUser.Email,
						Email:    githubUser.Email,
						Password: e.Password,
						Provider: "github",
						Status:   enum.Enabled,
					},
					Name:   githubUser.Login,
					Avatar: githubUser.AvatarURL,
					//Organization:    githubUser.Company,
					//Location:        githubUser.Location,
					//Introduction:    githubUser.Bio,
					Profile: githubUser.Blog,
					//Verified:        true,
					//AccountID:       fmt.Sprintf("%d", githubUser.ID),
				},
			}
			// register user
			err = center.GetDB(ctx.(*gin.Context), &User{}).Create(userOAuth2).Error
			if err != nil {
				slog.Error("create user error", slog.Any("error", err))
				return false, nil, err
			}
			userOAuth2.User.Role = defaultRole
		}
		return true, userOAuth2.User, nil
	}
	// username and password
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

// ********************* statistics *********************

func (e *User) AfterCreate(tx *gorm.DB) error {
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return nil
	}
	_ = center.Default.Increase(ctx, e)
	return nil
}

func (e *User) AfterDelete(tx *gorm.DB) error {
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return nil
	}
	_ = center.Default.Reduce(ctx, e)
	return nil
}

// StatisticsName statistics name
func (*User) StatisticsName() string {
	return "user-total"
}

// StatisticsType statistics type
func (*User) StatisticsType() string {
	return "user"
}

// StatisticsTime statistics time
func (*User) StatisticsTime() string {
	return pkg.NowFormatDay()
}

func (*User) StatisticsStep() int {
	return 100
}

// StatisticsCalibrate statistics calibrate
func (e *User) StatisticsCalibrate() (int, error) {
	var count int64
	err := gormdb.DB.Model(e).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// ********************* statistics *********************
