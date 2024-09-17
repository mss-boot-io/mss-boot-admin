package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	"golang.org/x/oauth2"

	"github.com/gin-gonic/gin"
	corePKG "github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/security"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
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
	Profile     string          `json:"profile" gorm:"column:profile;type:bytes"`
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

func (e *User) BeforeSave(*gorm.DB) error {
	//todo 判断密码强度
	return nil
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
func GetUserByUsername(ctx *gin.Context, username string) (*User, error) {
	var user User
	err := center.GetDB(ctx, &user).Preload("Role").First(&user, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail get user by email
func GetUserByEmail(ctx *gin.Context, email string) (*User, error) {
	var user User
	err := center.GetDB(ctx, &user).Preload("Role").First(&user, "email = ?", email).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type UserLogin struct {
	RoleID              string            `json:"roleID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Role                *Role             `json:"role" gorm:"foreignKey:RoleID;references:ID"`
	PostID              string            `json:"postID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Post                *Post             `json:"post" gorm:"foreignKey:PostID;references:ID"`
	DepartmentID        string            `json:"departmentID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Department          *Department       `json:"department" gorm:"foreignKey:DepartmentID;references:ID"`
	Username            string            `json:"username" gorm:"type:varchar(20);index"`
	Email               string            `json:"email" gorm:"type:varchar(100);index"`
	Password            string            `json:"password,omitempty" gorm:"-"`
	PasswordHash        string            `json:"-" gorm:"size:255;comment:密码hash" swaggerignore:"true"`
	PasswordStrength    string            `json:"passwordStrength" gorm:"size:20;comment:密码强度"`
	Salt                string            `json:"-" gorm:"size:255;comment:加盐" swaggerignore:"true"`
	Status              enum.Status       `json:"status" gorm:"size:10"`
	OAuth2              []*UserOAuth2     `json:"oauth2" gorm:"foreignKey:UserID;references:ID"`
	Provider            pkg.LoginProvider `json:"type" gorm:"-"`
	RefreshTokenDisable bool              `json:"-" gorm:"-"`
	PersonAccessToken   string            `json:"-" gorm:"-"`
	Captcha             string            `json:"captcha" gorm:"-"`
}

func (e *UserLogin) TableName() string {
	return "mss_boot_users"
}

func (e *UserLogin) GetUserID() string {
	return e.Username
}

func (e *UserLogin) GetTenantID() string {
	if e.Role == nil {
		return ""
	}
	return e.Role.TenantID
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

func (e *UserLogin) GetRefreshTokenDisable() bool {
	return e.RefreshTokenDisable
}

func (e *UserLogin) SetRefreshTokenDisable(support bool) {
	e.RefreshTokenDisable = support
}

func (e *UserLogin) GetPersonAccessToken() string {
	return e.PersonAccessToken
}

func (e *UserLogin) SetPersonAccessToken(token string) {
	e.PersonAccessToken = token
}

func (e *User) CheckToken(ctx context.Context, token string) error {
	userAuthToken := &UserAuthToken{}
	err := gormdb.DB.Model(&UserAuthToken{}).
		//Where("revoked = ?", false).
		Where("id = ?", token).
		First(userAuthToken).Error
	if err != nil {
		return err
	}
	if userAuthToken.ExpiredAt.Before(time.Now()) {
		return errors.New("token expired")
	}
	if userAuthToken.Revoked {
		return errors.New("token revoked")
	}
	err = gormdb.DB.Model(&User{}).
		Preload("Role").
		Where("id = ?", userAuthToken.UserID).
		First(e).Error
	if err != nil {
		return err
	}
	tenant := &Tenant{}
	err = gormdb.DB.Model(&Tenant{}).
		Where("id = ?", e.TenantID).
		Preload("Domains").
		First(tenant).Error
	if err != nil {
		return err
	}
	if len(tenant.Domains) == 0 {
		return errors.New("tenant domain not found")
	}
	ctx.(*gin.Context).Request.Header.Set("Referer", tenant.Domains[0].Domain)
	return nil
}

func (e *UserLogin) Root() bool {
	if e.Role == nil {
		return false
	}
	return e.Role.Root
}

var BeforeGithubVerify func(ctx context.Context, user *pkg.GithubUser, token string) error

// Verify verify password
func (e *UserLogin) Verify(ctx context.Context) (bool, security.Verifier, error) {
	c := ctx.(*gin.Context)
	defaultRole := &Role{Default: true}
	_ = center.GetDB(ctx.(*gin.Context), &Role{}).Where(*defaultRole).First(defaultRole).Error
	switch e.Provider {
	case pkg.GithubLoginProvider:
		// get user from github, then add user to db
		// github user
		clientID, _ := center.GetAppConfig().GetAppConfig(c, "security.githubClientId")
		clientSecret, _ := center.GetAppConfig().GetAppConfig(c, "security.githubClientSecret")
		redirectURL, _ := center.GetAppConfig().GetAppConfig(c, "security.githubRedirectUrl")
		scope, _ := center.GetAppConfig().GetAppConfig(c, "security.githubScope")
		scopes := strings.Split(scope, ",")
		allowGroup, _ := center.GetAppConfig().GetAppConfig(c, "security.githubAllowGroup")
		allowGroups := strings.Split(allowGroup, ",")
		if len(allowGroup) == 0 {
			allowGroups = nil
		}
		conf := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scopes,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		}
		githubUser, err := pkg.GetUserFromGithub(ctx, conf, e.Password)
		if err != nil {
			slog.Error("get user from github error", slog.Any("error", err))
			return false, nil, err
		}

		if len(allowGroups) > 0 {
			org, err := pkg.GetOrganizationsFromGithub(ctx, conf, e.Password)
			if err != nil {
				slog.Error("get organizations from github error", slog.Any("error", err))
				return false, nil, err
			}
			if !pkg.InArray(allowGroups, org, "", 0) {
				err = errors.New("user not in allow group")
				slog.Error(err.Error())
				return false, nil, err
			}
		}
		// custom access func
		if BeforeGithubVerify != nil {
			err = BeforeGithubVerify(ctx, githubUser, e.Password)
			if err != nil {
				return false, nil, err
			}
		}

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
				Provider:      pkg.GithubLoginProvider,
				User: &User{
					UserLogin: UserLogin{
						RoleID:   defaultRole.ID,
						Username: githubUser.Email,
						Email:    githubUser.Email,
						Password: e.Password,
						Provider: pkg.GithubLoginProvider,
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
	case pkg.LarkLoginProvider:
		client := http.Client{}
		req, err := http.NewRequest(http.MethodGet, "https://open.larksuite.com/open-apis/authen/v1/user_info", nil)
		if err != nil {
			slog.Error("new request error", slog.Any("error", err))
			return false, nil, err
		}
		req.Header.Add("Authorization", "Bearer "+e.Password)
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("do request error", slog.Any("error", err))
			return false, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = errors.New("get user from lark error")
			slog.Error(err.Error())
			return false, nil, err
		}
		result := &larkauthen.GetUserInfoResp{}
		err = json.NewDecoder(resp.Body).Decode(result)
		if err != nil {
			slog.Error("decode response error", slog.Any("error", err))
			return false, nil, err
		}
		userOAuth2 := &UserOAuth2{}
		err = center.GetDB(ctx.(*gin.Context), &UserOAuth2{}).
			Preload("User.Role").
			First(userOAuth2, "union_id = ?", result.Data.UnionId).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Error("get user from db error", slog.Any("error", err))
				return false, nil, err
			}
			err = nil
			email := ""
			if result.Data.EnterpriseEmail != nil {
				email = *result.Data.EnterpriseEmail
			}
			if email == "" && result.Data.Email != nil {
				email = *result.Data.Email
			}
			userOAuth2 = &UserOAuth2{
				UnionID:       *result.Data.UnionId,
				OpenID:        *result.Data.OpenId,
				Sub:           *result.Data.TenantKey,
				Name:          *result.Data.Name,
				Email:         email,
				Picture:       *result.Data.AvatarUrl,
				NickName:      *result.Data.Name,
				EmailVerified: email != "",
				Provider:      pkg.LarkLoginProvider,
				User: &User{
					UserLogin: UserLogin{
						RoleID:   defaultRole.ID,
						Username: *result.Data.UserId,
						Email:    email,
						Password: e.Password,
						Provider: pkg.LarkLoginProvider,
						Status:   enum.Enabled,
					},
					Name:   *result.Data.Name,
					Avatar: *result.Data.AvatarUrl,
				},
			}
			if result.Data.Mobile != nil {
				userOAuth2.PhoneNumber = *result.Data.Mobile
			}
			if result.Data.EmployeeNo != nil {
				userOAuth2.EmployeeNO = *result.Data.EmployeeNo
			}
			// register user
			err = center.GetDB(c, &User{}).Create(userOAuth2).Error
			if err != nil {
				slog.Error("create user error", slog.Any("error", err))
				return false, nil, err
			}
			userOAuth2.User.Role = defaultRole
		}
		return true, userOAuth2.User, nil
	case pkg.EmailLoginProvider:
		// verify captcha
		if e.Captcha == "" {
			return false, nil, nil
		}
		ok, err := center.Default.VerifyCode(c, e.Email, e.Captcha)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
		// get user from db
		user, err := GetUserByEmail(c, e.Email)
		if err != nil {
			return false, nil, err
		}
		return true, user, nil
	case pkg.EmailRegisterProvider:
		if e.RoleID != "" && e.Username != "" {
			// refresh token
			var user User
			err := center.GetDB(c, &User{}).Where("username = ?", e.Username).First(&user).Error
			if err != nil {
				return false, nil, err
			}
			return true, &user, nil
		}
		// verify captcha
		if e.Captcha == "" {
			return false, nil, nil
		}
		ok, err := center.Default.VerifyCode(c, e.Email, e.Captcha)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
		// fixme: 头像生成需要自己实现
		user := &User{}
		user.Username = e.Email
		user.Name = strings.Split(e.Email, "@")[0]
		user.Email = e.Email
		user.Password = e.Password
		user.Avatar = "https://avatars.githubusercontent.com/u/12806223?v=4"
		user.RoleID = defaultRole.ID
		user.Status = enum.Enabled                // register user
		user.Provider = pkg.EmailRegisterProvider // support email login
		err = center.GetDB(c, &User{}).Create(user).Error
		if err != nil {
			slog.Error("create user error", slog.Any("error", err))
			return false, nil, err
		}
		user.Role = defaultRole
		return true, user, nil
	}
	// username and password
	user, err := GetUserByUsername(ctx.(*gin.Context), e.Username)
	if err != nil {
		return false, nil, err
	}
	verify, err := security.SetPassword(e.Password, user.Salt)
	if err != nil {
		return false, nil, err
	}
	return verify == user.PasswordHash, user, nil
}

func (e *UserLogin) GetDepartmentUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	tx.Model(&User{}).
		Where("department_id = ?", e.DepartmentID).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) GetDepartmentAndChildrenUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	deptIDS := e.Department.GetAllChildrenID(tx)
	tx.Model(&User{}).
		Where("department_id in ?", deptIDS).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) GetCustomDepartmentUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	tx.Model(&User{}).Where("department_id in ?", e.Post.DeptIDSArr).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) GetPostUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	tx.Model(&User{}).Where("post_id = ?", e.PostID).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) GetPostAndChildrenUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	postIDS := e.Post.GetChildrenID(tx)
	tx.Model(&User{}).Where("post_id in ?", postIDS).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) GetPostAndAllChildrenUserID(tx *gorm.DB) []string {
	ids := make([]string, 0)
	postIDS := e.Post.GetAllChildrenID(tx)
	postIDS = append(postIDS, e.PostID)
	tx.Model(&User{}).Where("post_id in ?", postIDS).Pluck("id", &ids)
	return ids
}

func (e *UserLogin) getDataScopeCreator(ctx context.Context) []string {
	// get user from db
	if e.Post == nil {
		return nil
	}
	ids := make([]string, 0)
	tx := center.GetDB(ctx.(*gin.Context), &User{})
	switch e.Post.DataScope {
	case DataScopeAll:
		return nil
	case DataScopeCurrentDept:
		return e.GetDepartmentUserID(tx)
	case DataScopeCurrentAndChildrenDept:
		return e.GetDepartmentAndChildrenUserID(tx)
	case DataScopeCustomDept:
		return e.GetCustomDepartmentUserID(tx)
	case DataScopeSelf:
		return append(ids, e.GetUserID())
	case DataScopeSelfAndChildren:
		return e.GetPostAndChildrenUserID(tx)
	case DataScopeSelfAndAllChildren:
		return e.GetPostAndAllChildrenUserID(tx)
	}
	return nil
}

func (e *UserLogin) Scope(ctx *gin.Context, table schema.Tabler) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if !corePKG.SupportCreator(table) {
			return db
		}
		ids := e.getDataScopeCreator(ctx)
		if len(ids) == 0 {
			return db
		}
		db = db.Where("creator_id in ?", ids)
		return db
	}
}

func UserRegister(ctx *gin.Context, user *User) error {
	err := center.GetDB(ctx, user).Create(user).Error
	if err != nil {
		return err
	}
	return nil
}

// ********************* statistics *********************

func (e *User) AfterCreate(tx *gorm.DB) error {
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return nil
	}
	_ = center.Default.NowIncrease(ctx, e)
	return nil
}

func (e *User) AfterDelete(tx *gorm.DB) error {
	ctx, ok := tx.Statement.Context.(*gin.Context)
	if !ok {
		return nil
	}
	_ = center.Default.NowReduce(ctx, e)
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
