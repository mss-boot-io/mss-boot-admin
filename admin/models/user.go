package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	corePKG "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/spf13/cast"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
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
	canonicalEmail, err := CanonicalizeOptionalEmail(e.Email)
	if err != nil {
		return err
	}
	e.Email = canonicalEmail
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
	db := gormdb.DB.WithContext(ctx)
	if db.Logger != nil {
		db = db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		user := &User{}
		if err := tx.First(user, "id = ?", userID).Error; err != nil {
			return err
		}
		user.Salt = security.GenerateRandomKey6()
		hash, err := security.SetPassword(password, user.Salt)
		if err != nil {
			return err
		}
		if err := tx.Model(user).Updates(map[string]any{
			"password_hash":           hash,
			"salt":                    user.Salt,
			"local_password_disabled": false,
		}).Error; err != nil {
			return err
		}

		// Password rotation is a credential revocation boundary. Keep session
		// and PAT revocation in the same database transaction so no committed
		// password can coexist with an old durable credential. Lookup re-probes
		// session rows on cache hits, so stale cache entries still fail closed.
		now := time.Now()
		if tx.Migrator().HasTable(&UserSession{}) {
			if err := tx.Model(&UserSession{}).
				Where("user_id = ? AND revoked = ?", userID, false).
				Updates(map[string]any{
					"revoked":       true,
					"revoked_at":    now,
					"revoked_by":    userID,
					"revoke_reason": SessionRevokeForceByUser,
				}).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable(&UserAuthToken{}) {
			if err := tx.Model(&UserAuthToken{}).
				Where("user_id = ? AND revoked = ?", userID, false).
				Update("revoked", true).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
	canonicalEmail, err := CanonicalEmailIdentity(email)
	if err != nil {
		return nil, err
	}
	var users []User
	database := center.GetDB(ctx, &User{})
	if database.Logger != nil {
		database = database.Session(&gorm.Session{Logger: database.Logger.LogMode(logger.Silent)})
	}
	err = database.
		Preload("Role").
		Where("email = ?", canonicalEmail).
		Limit(2).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if len(users) != 1 {
		return nil, ErrEmailIdentityAmbiguous
	}
	return &users[0], nil
}

// LoadCurrentUserPrincipal returns the minimal authorization identity from one
// indexed user-to-role query. JWT and session data are only snapshots: status,
// role membership, and root authority always come from this database result.
// Password hashes and profile fields are deliberately excluded from the
// projection so they cannot accidentally enter request context or a token.
func LoadCurrentUserPrincipal(ctx context.Context, db *gorm.DB, userID string) (*User, error) {
	userID = strings.TrimSpace(userID)
	if db == nil || userID == "" {
		return nil, errors.New("current user principal is unavailable")
	}
	type principalRow struct {
		UserID     string      `gorm:"column:user_id"`
		Username   string      `gorm:"column:username"`
		RoleID     string      `gorm:"column:role_id"`
		UserStatus enum.Status `gorm:"column:user_status"`
		RoleName   string      `gorm:"column:role_name"`
		RoleStatus enum.Status `gorm:"column:role_status"`
		RoleRoot   bool        `gorm:"column:role_root"`
	}

	var row principalRow
	err := db.WithContext(ctx).
		Table("mss_boot_users AS auth_user").
		Select(
			"auth_user.id AS user_id, auth_user.username, auth_user.role_id, auth_user.status AS user_status, "+
				"auth_role.name AS role_name, auth_role.status AS role_status, auth_role.root AS role_root",
		).
		Joins("JOIN mss_boot_roles AS auth_role ON auth_role.id = auth_user.role_id").
		Where(
			"auth_user.id = ? AND auth_user.deleted_at IS NULL AND auth_role.deleted_at IS NULL",
			userID,
		).
		Take(&row).Error
	if err != nil {
		return nil, fmt.Errorf("load current user principal: %w", err)
	}
	if row.UserID == "" || row.RoleID == "" ||
		row.UserStatus != enum.Enabled || row.RoleStatus != enum.Enabled {
		return nil, errors.New("current user principal is disabled")
	}

	role := &Role{
		Name:   row.RoleName,
		Root:   row.RoleRoot,
		Status: row.RoleStatus,
	}
	role.ID = row.RoleID
	user := &User{UserLogin: UserLogin{
		RoleID:   row.RoleID,
		Role:     role,
		Username: row.Username,
		Status:   row.UserStatus,
	}}
	user.ID = row.UserID
	return user, nil
}

type UserLogin struct {
	RoleID                string            `json:"roleID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Role                  *Role             `json:"role" gorm:"foreignKey:RoleID;references:ID"`
	PostID                string            `json:"postID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Post                  *Post             `json:"post" gorm:"foreignKey:PostID;references:ID"`
	DepartmentID          string            `json:"departmentID" gorm:"index;type:varchar(64)" swaggerignore:"true"`
	Department            *Department       `json:"department" gorm:"foreignKey:DepartmentID;references:ID"`
	Username              string            `json:"username" gorm:"type:varchar(20);index"`
	Email                 string            `json:"email" gorm:"type:varchar(100);index"`
	Password              string            `json:"password,omitempty" gorm:"-"`
	LocalPasswordDisabled bool              `json:"-" gorm:"not null;default:false;comment:disable local password login" swaggerignore:"true"`
	PasswordHash          string            `json:"-" gorm:"size:255;comment:密码hash" swaggerignore:"true"`
	PasswordStrength      string            `json:"passwordStrength" gorm:"size:20;comment:密码强度"`
	Salt                  string            `json:"-" gorm:"size:255;comment:加盐" swaggerignore:"true"`
	Status                enum.Status       `json:"status" gorm:"size:10"`
	OAuth2                []*UserOAuth2     `json:"oauth2" gorm:"foreignKey:UserID;references:ID"`
	Provider              pkg.LoginProvider `json:"type" gorm:"-"`
	RefreshTokenDisable   bool              `json:"-" gorm:"-"`
	PersonAccessToken     string            `json:"-" gorm:"-"`
	Captcha               string            `json:"captcha,omitempty" gorm:"-"`
}

func (e *UserLogin) TableName() string {
	return "mss_boot_users"
}

func (e *UserLogin) GetUserID() string {
	return e.Username
}

func (e *UserLogin) GetTenantID() string {
	return "default"
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
	tokenID := e.GetPersonAccessToken()
	if tokenID == "" || token == "" {
		return errors.New("token invalid")
	}
	userAuthToken := &UserAuthToken{}
	err := gormdb.DB.WithContext(ctx).Model(&UserAuthToken{}).
		Where("id = ?", tokenID).
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
	if !VerifyUserAuthToken(token, userAuthToken.TokenHash) {
		return errors.New("token invalid")
	}
	principal, err := LoadCurrentUserPrincipal(ctx, gormdb.DB, userAuthToken.UserID)
	if err != nil {
		return err
	}
	*e = *principal
	e.SetRefreshTokenDisable(true)
	e.SetPersonAccessToken(tokenID)
	return nil
}

func (e *UserLogin) Root() bool {
	if e.Role == nil {
		return false
	}
	return e.Role.Root
}

var BeforeGithubVerify func(ctx context.Context, user *pkg.GithubUser, token string) error

const (
	larkUserInfoURL          = "https://open.larksuite.com/open-apis/authen/v1/user_info"
	oauthUserInfoTimeout     = 10 * time.Second
	oauthUserInfoMaxBodySize = 1 << 20
)

func requirePublicRegistration(c *gin.Context) error {
	value, ok := userAppConfig(c, "security:registerEnabled")
	if !ok || !cast.ToBool(value) {
		return errors.New("public registration is disabled")
	}
	return nil
}

// provisioningRole fails closed unless one and only one active default role
// exists and that role is explicitly enabled and non-root. Existing OAuth
// identities never call this helper, so disabling registration does not break
// sign-in for already provisioned accounts.
func provisioningRole(c *gin.Context) (*Role, error) {
	var roles []Role
	err := center.GetDB(c, &Role{}).
		Where(map[string]any{"default": true}).
		Limit(2).
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("resolve provisioning role: %w", err)
	}
	if len(roles) != 1 {
		return nil, fmt.Errorf("provisioning role is ambiguous: found %d default roles", len(roles))
	}
	role := &roles[0]
	if role.ID == "" || role.Status != enum.Enabled || role.Root {
		return nil, errors.New("provisioning role must be enabled and non-root")
	}
	return role, nil
}

// Verify verify password
func (e *UserLogin) Verify(ctx context.Context) (bool, security.Verifier, error) {
	c := ctx.(*gin.Context)
	switch e.Provider {
	case pkg.GithubLoginProvider:
		// get user from db
		userOAuth2, err := e.GetUserGithubOAuth2(c)
		if err != nil {
			// Provider and extension-hook errors are untrusted and may contain
			// bearer credentials. Preserve the error for the caller, but never
			// attach its detail to application logs.
			slog.Error("github identity verification failed")
			return false, nil, err
		}
		if userOAuth2.ID == "" {
			// register
			if err := requirePublicRegistration(c); err != nil {
				return false, nil, err
			}
			defaultRole, err := provisioningRole(c)
			if err != nil {
				return false, nil, err
			}
			username := userOAuth2.Email
			if username == "" {
				username = userOAuth2.PreferredUsername
			}
			userOAuth2.User = &User{
				UserLogin: UserLogin{
					RoleID:                defaultRole.ID,
					Username:              username,
					Email:                 userOAuth2.Email,
					Password:              security.GenerateRandomKey20(),
					LocalPasswordDisabled: true,
					Provider:              pkg.GithubLoginProvider,
					Status:                enum.Enabled,
				},
				Name:    userOAuth2.Name,
				Avatar:  userOAuth2.Picture,
				Profile: userOAuth2.Profile,
			}
			if e.GetUserID() != "" {
				userOAuth2.User = nil
			}
			err = createOAuthIdentityWithoutEmailMerge(center.GetDB(c, &UserOAuth2{}), userOAuth2)
			if err != nil {
				slog.Error("github identity registration failed")
				return false, nil, err
			}
			userOAuth2.User.Role = defaultRole
		}
		return true, userOAuth2.User, nil
	case pkg.LarkLoginProvider:
		userOAuth2, err := e.GetUserLarkOAuth2(c)
		if err != nil {
			slog.Error("lark identity verification failed")
			return false, nil, err
		}
		if userOAuth2.ID == "" {
			// register
			if err := requirePublicRegistration(c); err != nil {
				return false, nil, err
			}
			defaultRole, err := provisioningRole(c)
			if err != nil {
				return false, nil, err
			}
			userOAuth2.User = &User{
				UserLogin: UserLogin{
					RoleID:                defaultRole.ID,
					Username:              userOAuth2.PreferredUsername,
					Email:                 userOAuth2.Email,
					Password:              security.GenerateRandomKey20(),
					LocalPasswordDisabled: true,
					Provider:              pkg.LarkLoginProvider,
					Status:                enum.Enabled,
				},
				Name:   userOAuth2.Name,
				Avatar: userOAuth2.Picture,
			}
			if e.GetUserID() != "" {
				userOAuth2.User = nil
			}
			err = createOAuthIdentityWithoutEmailMerge(center.GetDB(c, &UserOAuth2{}), userOAuth2)
			if err != nil {
				slog.Error("lark identity registration failed")
				return false, nil, err
			}
			userOAuth2.User.Role = defaultRole
		}
		return true, userOAuth2.User, nil
	case pkg.EmailLoginProvider:
		canonicalEmail, canonicalErr := CanonicalEmailIdentity(e.Email)
		if canonicalErr != nil {
			return false, nil, nil
		}
		e.Email = canonicalEmail
		if !center.EmailChallengeCapabilityEnabled(c) {
			return false, nil, nil
		}
		// verify captcha
		if e.Captcha == "" {
			return false, nil, nil
		}
		challenge := center.GetChallenge()
		if challenge == nil {
			return false, nil, storagecache.ErrChallengeUnavailable
		}
		ok, err := challenge.VerifyChallenge(c.Request.Context(), e.Email, pkg.EmailLoginChallengePurpose, e.Captcha)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
		// get user from db
		user, err := GetUserByEmail(c, e.Email)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil, nil
			}
			return false, nil, errors.Join(storagecache.ErrChallengeUnavailable, err)
		}
		return true, user, nil
	case pkg.EmailRegisterProvider:
		canonicalEmail, canonicalErr := CanonicalEmailIdentity(e.Email)
		if canonicalErr != nil {
			return false, nil, nil
		}
		e.Email = canonicalEmail
		if err := requirePublicRegistration(c); err != nil {
			return false, nil, err
		}
		if !center.EmailChallengeCapabilityEnabled(c) {
			return false, nil, nil
		}
		defaultRole, err := provisioningRole(c)
		if err != nil {
			return false, nil, err
		}
		// verify captcha
		if e.Captcha == "" {
			return false, nil, nil
		}
		challenge := center.GetChallenge()
		if challenge == nil {
			return false, nil, storagecache.ErrChallengeUnavailable
		}
		ok, err := challenge.VerifyChallenge(c.Request.Context(), e.Email, pkg.EmailRegisterChallengePurpose, e.Captcha)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
		// fixme: 头像生成需要自己实现
		user := &User{}
		// Email identifiers may be up to 100 bytes while the legacy username
		// column is varchar(20). Use an opaque bounded local username so a valid
		// challenge is not consumed before a cross-database truncation failure.
		user.Username = strings.ToLower(
			security.GenerateRandomKey6() + security.GenerateRandomKey6() + security.GenerateRandomKey6(),
		)
		user.Name = strings.Split(e.Email, "@")[0]
		user.Email = e.Email
		user.Password = e.Password
		user.Avatar = "https://avatars.githubusercontent.com/u/12806223?v=4"
		user.RoleID = defaultRole.ID
		user.Status = enum.Enabled                // register user
		user.Provider = pkg.EmailRegisterProvider // support email login
		registrationDB := center.GetDB(c, &User{})
		if registrationDB.Logger != nil {
			registrationDB = registrationDB.Session(&gorm.Session{Logger: registrationDB.Logger.LogMode(logger.Silent)})
		}
		err = registrationDB.Transaction(func(tx *gorm.DB) error {
			return createUserWithCanonicalEmail(tx, user)
		}, &sql.TxOptions{Isolation: sql.LevelSerializable})
		err = NormalizeEmailIdentityCreateError(registrationDB, user.Email, err)
		if err != nil {
			if errors.Is(err, ErrEmailIdentityExists) {
				return false, nil, nil
			}
			slog.Error("email registration transaction unavailable")
			return false, nil, errors.Join(storagecache.ErrChallengeUnavailable, err)
		}
		user.Role = defaultRole
		return true, user, nil
	}
	// username and password
	user, err := GetUserByUsername(ctx.(*gin.Context), e.Username)
	if err != nil {
		return false, nil, err
	}
	if user.LocalPasswordDisabled {
		return false, nil, nil
	}
	verify, err := security.SetPassword(e.Password, user.Salt)
	if err != nil {
		return false, nil, err
	}
	return verify == user.PasswordHash, user, nil
}

func (e *UserLogin) GetUserLarkOAuth2(c *gin.Context) (*UserOAuth2, error) {
	return e.getUserLarkOAuth2(c, &http.Client{Timeout: oauthUserInfoTimeout}, larkUserInfoURL)
}

func (e *UserLogin) getUserLarkOAuth2(
	c *gin.Context,
	client *http.Client,
	endpoint string,
) (*UserOAuth2, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("lark user request context is required")
	}
	if client == nil || client.Timeout <= 0 {
		return nil, errors.New("lark user client timeout is required")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		slog.Error("lark identity request creation failed")
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+e.Password)
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("lark identity request failed")
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("get user from lark error")
		slog.Error(err.Error())
		return nil, err
	}
	result := &larkauthen.GetUserInfoResp{}
	err = json.NewDecoder(io.LimitReader(resp.Body, oauthUserInfoMaxBodySize)).Decode(result)
	if err != nil {
		slog.Error("lark identity response decode failed")
		return nil, err
	}
	if result.Code != 0 || result.Data == nil || strings.TrimSpace(stringValue(result.Data.UnionId)) == "" {
		return nil, errors.New("get user from lark error")
	}
	data := result.Data
	identityKey, err := UserOAuthIdentityKey(
		pkg.LarkLoginProvider,
		stringValue(data.OpenId),
		stringValue(data.UnionId),
	)
	if err != nil {
		return nil, err
	}
	userOAuth2 := &UserOAuth2{}
	err = center.GetDB(c, &UserOAuth2{}).
		Preload("User.Role").
		First(userOAuth2, "identity_key = ?", identityKey).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("lark identity database lookup failed")
			return nil, err
		}
		err = nil
		email := stringValue(data.EnterpriseEmail)
		if email == "" {
			email = stringValue(data.Email)
		}
		preferredUsername := stringValue(data.UserId)
		if preferredUsername == "" {
			preferredUsername = stringValue(data.EnName)
		}
		if preferredUsername == "" {
			preferredUsername = stringValue(data.Name)
		}

		userOAuth2 = &UserOAuth2{
			UnionID:           stringValue(data.UnionId),
			OpenID:            stringValue(data.OpenId),
			Sub:               stringValue(data.TenantKey),
			Name:              stringValue(data.Name),
			Email:             email,
			Picture:           stringValue(data.AvatarUrl),
			NickName:          stringValue(data.Name),
			EmailVerified:     email != "",
			Provider:          pkg.LarkLoginProvider,
			PreferredUsername: preferredUsername,
		}
		if userOAuth2.Email != "" {
			userOAuth2.Email, err = CanonicalEmailIdentity(userOAuth2.Email)
			if err != nil {
				return nil, err
			}
		}
		userOAuth2.PhoneNumber = stringValue(data.Mobile)
		userOAuth2.EmployeeNO = stringValue(data.EmployeeNo)
	}
	return userOAuth2, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (e *UserLogin) GetUserGithubOAuth2(c *gin.Context) (*UserOAuth2, error) {
	githubEnabled, ok := userAppConfig(c, "security:githubEnabled")
	if !ok || !cast.ToBool(githubEnabled) {
		return nil, errors.New("github login is disabled")
	}
	clientID, _ := userAppConfig(c, "security:githubClientId")
	clientSecret, _ := userAppConfig(c, "security:githubClientSecret")
	redirectURL, _ := userAppConfig(c,
		"security:githubRedirectURI",
		"security:githubRedirectUrl",
		"security:githubRedirectURL",
	)
	scope, _ := userAppConfig(c, "security:githubScope")
	allowGroup, allowGroupConfigured := userAppConfig(c, "security:githubAllowGroup")
	allowGroups := splitGithubCSV(allowGroup)
	if allowGroupConfigured && strings.TrimSpace(allowGroup) != "" && len(allowGroups) == 0 {
		return nil, errors.New("github allow group configuration is invalid")
	}
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       splitGithubCSV(scope),
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
	requestContext := context.Context(c)
	if c.Request != nil {
		requestContext = c.Request.Context()
	}
	githubUser, err := pkg.GetUserFromGithub(requestContext, conf, e.Password)
	if err != nil {
		slog.Error("github identity request failed")
		return nil, err
	}
	if len(allowGroups) > 0 {
		org, err := pkg.GetOrganizationsFromGithub(requestContext, conf, e.Password)
		if err != nil {
			slog.Error("github organization request failed")
			return nil, err
		}
		if !githubOrganizationAllowed(allowGroups, org) {
			err = errors.New("user not in allow group")
			slog.Error(err.Error())
			return nil, err

		}
	}
	// custom access func
	if BeforeGithubVerify != nil {
		err = BeforeGithubVerify(c, githubUser, e.Password)
		if err != nil {
			return nil, err
		}
	}
	// get user from db
	openID := fmt.Sprintf("%d", githubUser.ID)
	identityKey, err := UserOAuthIdentityKey(pkg.GithubLoginProvider, openID, "")
	if err != nil {
		return nil, err
	}
	userOAuth2 := &UserOAuth2{}
	err = center.GetDB(c, &UserOAuth2{}).Preload("User.Role").First(userOAuth2, "identity_key = ?", identityKey).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("github identity database lookup failed")
			return nil, err
		}
		err = nil
		userOAuth2 = &UserOAuth2{
			UserID:            e.GetUserID(),
			OpenID:            openID,
			Sub:               "github",
			Name:              githubUser.Login,
			Email:             githubUser.Email,
			Profile:           githubUser.Blog,
			Picture:           githubUser.AvatarURL,
			NickName:          githubUser.Login,
			Website:           githubUser.HTMLURL,
			EmailVerified:     true,
			Locale:            githubUser.Location,
			Provider:          pkg.GithubLoginProvider,
			PreferredUsername: githubUser.Login,
		}
		if userOAuth2.Email != "" {
			userOAuth2.Email, err = CanonicalEmailIdentity(userOAuth2.Email)
			if err != nil {
				return nil, err
			}
		}
	}
	return userOAuth2, nil
}

func userAppConfig(c *gin.Context, keys ...string) (string, bool) {
	store := center.GetAppConfig()
	if store == nil {
		return "", false
	}
	for _, key := range keys {
		if value, ok := store.GetAppConfig(c, key); ok {
			return value, true
		}
	}
	return "", false
}

func splitGithubCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func githubOrganizationAllowed(allowed, memberships []string) bool {
	for _, expected := range allowed {
		for _, membership := range memberships {
			if strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(membership)) {
				return true
			}
		}
	}
	return false
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
	return createUserWithCanonicalEmail(center.GetDB(ctx, user), user)
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
