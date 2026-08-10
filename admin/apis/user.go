package apis

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/notice/email"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 22:13:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 22:13:11
 */

func init() {
	e := &User{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.User)),
			controller.WithSearch(new(dto.UserSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement, protectRootUserLifecycle}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement, protectRootUserLifecycle}),
		),
	}
	response.AppendController(e)
}

type User struct {
	*controller.Simple
	oauthStates          oauthStateStore
	oauthURLBuilder      oauthAuthorizeURLBuilder
	oauthCodeExchange    oauthCodeExchange
	oauthLoginComplete   oauthLoginCompleter
	oauthBindingComplete oauthBindingCompleter
	challengeSender      email.VerifyCodeSender
	challengeSendSlots   chan struct{}
}

var defaultEmailChallengeSendSlots = make(chan struct{}, 32)

// Other handler
func (e *User) Other(r *gin.RouterGroup) {
	r.POST("/user/login", middleware.PublicLoginHandler)
	r.POST("/user/auth-cookie/clear", e.ClearAuthCookie)
	r.POST("/user/reset-password", middleware.OptionalAuth(), e.ResetPassword)
	r.POST("/user/fakeCaptcha", e.FakeCaptcha)
	r.POST("/user/login/github", methodNotAllowed)
	r.POST("/user/refresh-token", middleware.Auth.RefreshHandler)
	r.GET("/user/refresh-token", methodNotAllowed)
	r.GET("/user/userInfo", middleware.Auth.MiddlewareFunc(), e.UserInfo)
	r.PUT("/user/:userID/password-reset", response.AuthHandler, e.PasswordReset)
	r.PUT("/user/userInfo", middleware.Auth.MiddlewareFunc(), e.UpdateUserInfo)
	r.POST("/user/avatar", middleware.Auth.MiddlewareFunc(), e.UpdateAvatar)
	r.GET("/user/oauth2", response.AuthHandler, e.GetOauth2)
	r.POST("/user/oauth2/authorize", middleware.OptionalAuth(), e.OAuthAuthorize)
	r.POST("/user/binding", response.AuthHandler, e.Binding)
	r.DELETE("/user/unbinding", response.AuthHandler, e.Unbinding)
	r.POST("/user/:provider/callback", middleware.OptionalAuth(), e.Callback)
	r.GET("/user/:provider/callback", methodNotAllowed)
}

// ClearAuthCookie expires the browser's HttpOnly Admin JWT cookie before a
// deliberate login/account-switch flow. It does not revoke a live session;
// authenticated users should use the session logout endpoint for that.
func (*User) ClearAuthCookie(c *gin.Context) {
	middleware.ClearAuthCookie(c)
	c.Status(http.StatusNoContent)
}

// Unbinding 解绑第三方登录
// @Summary 解绑第三方登录
// @Description 解绑第三方登录
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body models.UserLogin true "data"
// @Success 204
// @Router /admin/api/user/unbinding [delete]
// @Security Bearer
func (e *User) Unbinding(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.Err(http.StatusForbidden)
		return
	}
	if middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	req := &models.UserLogin{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	user := verify.(*models.User)
	err := center.GetDB(ctx, &models.UserOAuth2{}).Where("user_id = ?", user.ID).
		Where("provider = ?", req.Provider).
		Unscoped().Delete(&models.UserOAuth2{}).Error
	if err != nil {
		api.AddError(err).Log.Error("DeleteUserOAuth2 error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Binding 绑定第三方登录
// @Summary 绑定第三方登录
// @Description 绑定第三方登录
// Deprecated: browser-submitted provider tokens are rejected. Binding now
// completes inside the state-bound OAuth callback.
func (e *User) Binding(ctx *gin.Context) {
	verify := response.VerifyHandler(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		response.Make(ctx).Err(http.StatusForbidden)
		return
	}
	methodNotAllowed(ctx)
}

// GetOauth2 获取用户第三方登录信息
// @Summary 获取用户第三方登录信息
// @Description 获取用户第三方登录信息
// @Tags user
// @Accept  application/json
// @Product application/json
// @Success 200 {object} []models.UserOAuth2
// @Router /admin/api/user/oauth2 [get]
// @Security Bearer
func (e *User) GetOauth2(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.Err(http.StatusForbidden)
		return
	}
	user := &models.User{}
	err := center.Default.GetDB(ctx, &models.User{}).
		Preload("OAuth2").
		Where("id = ?", verify.GetUserID()).
		First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("GetUser error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(user.OAuth2)
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 重置密码
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body dto.ResetPasswordRequest true "data"
// @Success 200
// @Router /admin/api/user/reset-password [post]
func (e *User) ResetPassword(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	// A personal access token is an automation credential, not proof of an
	// interactive account-recovery session. Keep this defensive check in the
	// handler as well as the auth Authorizator so route wiring cannot bypass it.
	if middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	req := &dto.ResetPasswordRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if verify != nil {
		err := models.PasswordReset(ctx, verify.GetUserID(), req.Password)
		if err != nil {
			api.AddError(err).Log.Error("PasswordReset error")
			api.Err(http.StatusInternalServerError)
			return
		}
		api.OK(struct{}{})
		return
	}
	if req.Email == "" || req.Captcha == "" {
		api.Err(http.StatusForbidden)
		return

	}
	canonicalEmail, validEmail := pkg.CanonicalEmail(req.Email)
	if !validEmail {
		api.Err(http.StatusForbidden)
		return
	}
	req.Email = canonicalEmail
	if !center.EmailChallengeCapabilityEnabled(ctx) {
		api.Err(http.StatusForbidden)
		return
	}
	challenge := center.GetChallenge()
	if challenge == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	ok, err := challenge.VerifyChallenge(ctx.Request.Context(), req.Email, pkg.PasswordResetChallengePurpose, req.Captcha)
	if err != nil {
		api.AddError(storagecache.ErrChallengeUnavailable).Log.Warn("password recovery challenge unavailable")
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if !ok {
		api.Err(http.StatusForbidden)
		return
	}
	user, err := models.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusForbidden)
			return
		}
		api.Log.Error("password recovery account lookup unavailable")
		api.Err(http.StatusServiceUnavailable)
		return
	}
	err = models.PasswordReset(ctx, user.ID, req.Password)
	if err != nil {
		api.AddError(err).Log.Error("PasswordReset error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// UpdateAvatar stores a new avatar for the authenticated user.
// @Summary Upload the current user's avatar
// @Tags user
// @Accept multipart/form-data
// @Param file formData file true "Avatar"
// @Success 201 {object} dto.UpdateAvatarResponse
// @Failure 413 {object} response.Response
// @Failure 422 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/api/user/avatar [post]
// @Security Bearer
func (e *User) UpdateAvatar(ctx *gin.Context) {
	api := response.Make(ctx)
	s := service.Storage{}
	result, err := s.Upload(ctx, "file")
	if err != nil {
		writeUploadError(api, err)
		return
	}
	api.OK(dto.UpdateAvatarResponse{Avatar: result.URL})
}

// UpdateUserInfo 更新用户信息
// @Summary 更新用户信息
// @Description 更新用户信息
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body dto.UpdateUserInfoRequest true "data"
// @Success 200
// @Router /admin/api/user/userInfo [put]
// @Security Bearer
func (e *User) UpdateUserInfo(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}

	var reqMap map[string]any
	if err := ctx.ShouldBindJSON(&reqMap); err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if _, attemptsEmailChange := reqMap["email"]; attemptsEmailChange {
		// Email is an authentication and recovery identity. Until the v1.1.0 D2
		// canonical-identity wave lands, self-service mutation could create
		// an ambiguous identity and deny login/reset to another account.
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	user := &models.User{}
	err := center.Default.GetDB(ctx, &models.User{}).Where("id = ?", verify.GetUserID()).First(user).Error
	if err != nil {
		api.AddError(err).Log.Error("GetUser error")
		api.Err(http.StatusInternalServerError)
		return
	}

	if v, ok := reqMap["name"].(string); ok {
		user.Name = v
	}
	if v, ok := reqMap["avatar"].(string); ok {
		user.Avatar = v
	}
	if v, ok := reqMap["signature"].(string); ok {
		user.Signature = v
	}
	if v, ok := reqMap["title"].(string); ok {
		user.Title = v
	}
	if v, ok := reqMap["group"].(string); ok {
		user.Group = v
	}
	if v, ok := reqMap["country"].(string); ok {
		user.Country = v
	}
	if v, ok := reqMap["province"].(string); ok {
		user.Province = v
	}
	if v, ok := reqMap["city"].(string); ok {
		user.City = v
	}
	if v, ok := reqMap["address"].(string); ok {
		user.Address = v
	}
	if v, ok := reqMap["phone"].(string); ok {
		user.Phone = v
	}
	if v, ok := reqMap["profile"].(string); ok {
		user.Profile = v
	}
	if v, ok := reqMap["tags"].([]any); ok {
		tags := make([]string, 0, len(v))
		for _, tag := range v {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
		user.Tags = tags
	}

	err = center.Default.GetDB(ctx, &models.User{}).Model(&models.User{}).Where("id = ?", verify.GetUserID()).Updates(user).Error
	if err != nil {
		api.AddError(err).Log.Error("UpdateUserInfo error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Login 登录
// @Summary 登录
// @Description 登录
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body models.UserLogin true "data"
// @Success 200 {object} dto.LoginResponse "{"code": 200, "expire": "2023-12-10T12:31:30+08:00", "token": "xxx"}"
// @Router /admin/api/user/login [post]
func (e *User) Login(_ *gin.Context) {}

// RefreshToken 刷新token
// @Summary 刷新token
// @Description 刷新token
// @Tags user
// @Accept  application/json
// @Product application/json
// @Success 200 {object} dto.LoginResponse "{"code": 200, "expire": "2023-12-10T12:31:30+08:00", "token":
// @Router /admin/api/user/refresh-token [post]
// @Security Bearer
func (e *User) RefreshToken(_ *gin.Context) {

}

// FakeCaptcha 获取验证码
// @Summary 获取验证码
// @Description 获取验证码
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body dto.FakeCaptchaRequest true "data"
// @Success 202 {object} dto.FakeCaptchaResponse
// @Failure 403 {object} response.Response "Email challenge capability is disabled"
// @Failure 422 {object} response.Response "Invalid email or challenge purpose"
// @Failure 429 {object} response.Response "Caller, global, subject, or sender-concurrency limit exceeded"
// @Failure 503 {object} response.Response "Challenge store or email delivery is unavailable"
// @Router /admin/api/user/fakeCaptcha [post]
func (e *User) FakeCaptcha(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.FakeCaptchaRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	canonicalEmail, validEmail := pkg.CanonicalEmail(req.Email)
	if !validEmail {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	req.Email = canonicalEmail
	purpose, sendType, ok := emailChallengePurpose(req.UseBy)
	if !ok {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	appConfig := center.GetAppConfig()
	if appConfig == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if !center.EmailChallengeCapabilityEnabled(ctx) {
		api.Err(http.StatusForbidden)
		return
	}
	if purpose == pkg.EmailRegisterChallengePurpose {
		registerEnabled, exists := appConfig.GetAppConfig(ctx, "security:registerEnabled")
		enabled, parseErr := strconv.ParseBool(registerEnabled)
		if !exists || parseErr != nil || !enabled {
			api.Err(http.StatusForbidden)
			return
		}
	}
	challenge := center.GetChallenge()
	requestCtx := ctx.Request.Context()
	if challenge == nil || challenge.Ready(requestCtx) != nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	smtpHost, hostOK := appConfig.GetAppConfig(ctx, "email:smtpHost")
	smtpPort, portOK := appConfig.GetAppConfig(ctx, "email:smtpPort")
	username, usernameOK := appConfig.GetAppConfig(ctx, "email:username")
	password, passwordOK := appConfig.GetAppConfig(ctx, "email:password")
	if !hostOK || !portOK || !usernameOK || !passwordOK {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	organization, organizationOK := appConfig.GetAppConfig(ctx, "base:websiteName")
	if !organizationOK || organization == "" {
		organization = "mss-boot-io"
	}
	sender := e.challengeSender
	if sender == nil {
		sender = email.Sender[sendType]
	}
	if sender == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	slots := e.challengeSendSlots
	if slots == nil {
		slots = defaultEmailChallengeSendSlots
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		api.Err(http.StatusServiceUnavailable)
		return
	default:
		api.Err(http.StatusTooManyRequests)
		return
	}
	recipientName := strings.SplitN(req.Email, "@", 2)[0]
	err := challenge.Issue(requestCtx, emailChallengeCaller(ctx), req.Email, purpose, func(deliveryCtx context.Context, code string) error {
		return sender(deliveryCtx, smtpHost, smtpPort, username, password, recipientName, req.Email, code, organization)
	})
	if err != nil {
		switch {
		case errors.Is(err, storagecache.ErrChallengePending),
			errors.Is(err, storagecache.ErrChallengeCooldown),
			errors.Is(err, storagecache.ErrChallengeQuota):
			api.Err(http.StatusTooManyRequests)
		case errors.Is(err, storagecache.ErrChallengeInvalid):
			api.Err(http.StatusUnprocessableEntity)
		default:
			api.Log.Warn("email challenge delivery unavailable")
			api.Err(http.StatusServiceUnavailable)
		}
		return
	}
	ctx.AbortWithStatusJSON(http.StatusAccepted, &dto.FakeCaptchaResponse{Status: "accepted"})
}

// emailChallengeCaller uses Gin's engine-level trusted-proxy policy. The
// composition root explicitly disables forwarding headers unless operators
// configure an allowlist, preventing arbitrary X-Forwarded-For rotation.
func emailChallengeCaller(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil {
		return "unknown"
	}
	caller := strings.TrimSpace(ctx.ClientIP())
	if caller == "" {
		return "unknown"
	}
	return caller
}

func emailChallengePurpose(useBy string) (storagecache.ChallengePurpose, email.SendType, bool) {
	switch useBy {
	case email.RegisterSender.String():
		return pkg.EmailRegisterChallengePurpose, email.RegisterSender, true
	case email.LoginSender.String():
		return pkg.EmailLoginChallengePurpose, email.LoginSender, true
	case email.ResetPasswordSender.String():
		return pkg.PasswordResetChallengePurpose, email.ResetPasswordSender, true
	default:
		return "", "", false
	}
}

// UserInfo 获取登录用户信息
// @Summary 获取登录用户信息
// @Description 获取登录用户信息
// @Tags user
// @Accept  application/json
// @Product application/json
// @Success 200 {object} models.User
// @Router /admin/api/user/userInfo [get]
// @Security Bearer
func (e *User) UserInfo(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		api.Err(http.StatusForbidden)
		return
	}
	user := &models.User{}
	err := center.Default.GetDB(ctx, &models.User{}).Preload("Role").Where("id = ?", verify.GetUserID()).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("GetUser error")
		api.Err(http.StatusInternalServerError)
		return
	}
	user.Permissions = make(map[string]bool)
	if verify.Root() {
		user.Permissions["canAdmin"] = true
	} else {
		// userInfo is a self-service route, so the outer auth middleware does
		// not consult Casbin. Reconcile the durable authorization revision here
		// before projecting permissions into the frontend; otherwise an instance
		// that missed a watcher event could revive already-revoked controls.
		if err := ensureCurrentAuthorizationPolicies(
			ctx,
			center.Default.GetDB(ctx, &models.ConfigRevision{}),
		); err != nil {
			api.AddError(err).Log.Error("reconcile user permission projection", "err", err)
			api.Err(http.StatusServiceUnavailable)
			return
		}
		if gormdb.Enforcer == nil {
			api.AddError(errors.New("authorization policy enforcer is unavailable")).
				Log.Error("get user permission projection")
			api.Err(http.StatusServiceUnavailable)
			return
		}
		permissions, err := gormdb.Enforcer.GetFilteredPolicy(0, verify.GetRoleID(), pkg.MenuAccessType.String())
		if err != nil {
			api.AddError(err).Log.Error("get filtered policy error", "err", err)
			api.Err(http.StatusServiceUnavailable)
			return
		}
		enforcers, err := gormdb.Enforcer.GetFilteredPolicy(0, verify.GetRoleID(), pkg.ComponentAccessType.String())
		if err != nil {
			api.AddError(err).Log.Error("get filtered policy error", "err", err)
			api.Err(http.StatusServiceUnavailable)
			return
		}
		permissions = append(permissions, enforcers...)
		permissionPaths := make([]string, 0, len(permissions))
		for i := range permissions {
			if len(permissions[i]) >= 3 {
				permissionPaths = append(permissionPaths, permissions[i][2])
			}
		}
		activePermissionMetadata := make(map[string]struct{}, len(permissionPaths))
		if len(permissionPaths) > 0 {
			var activeMenus []models.Menu
			if err := center.Default.GetDB(ctx, &models.Menu{}).
				Select("type", "path").
				Where("type IN ?", []pkg.AccessType{pkg.MenuAccessType, pkg.ComponentAccessType}).
				Where("status = ?", enum.Enabled).
				Where("path IN ?", permissionPaths).
				Find(&activeMenus).Error; err != nil {
				api.AddError(err).Log.Error("load active permission metadata", "err", err)
				api.Err(http.StatusServiceUnavailable)
				return
			}
			for i := range activeMenus {
				activePermissionMetadata[activeMenus[i].Type.String()+"\x00"+activeMenus[i].Path] = struct{}{}
			}
		}
		user.Permissions = projectActiveUserPermissions(
			verify.GetRoleID(),
			permissions,
			activePermissionMetadata,
		)
	}
	if user.DepartmentID != "" && user.Department == nil {
		user.Department = &models.Department{}
		user.Department.ID = user.DepartmentID
	}
	if user.PostID != "" && user.Post == nil {
		user.Post = &models.Post{}
		user.Post.ID = user.PostID
	}
	api.OK(user)
}

func projectActiveUserPermissions(
	roleID string,
	policies [][]string,
	activeMetadata map[string]struct{},
) map[string]bool {
	result := make(map[string]bool)
	for i := range policies {
		if len(policies[i]) < 4 || policies[i][0] != roleID {
			continue
		}
		accessType := policies[i][1]
		if accessType != pkg.MenuAccessType.String() && accessType != pkg.ComponentAccessType.String() {
			continue
		}
		if _, active := activeMetadata[accessType+"\x00"+policies[i][2]]; active {
			result[policies[i][2]] = true
		}
	}
	return result
}

// PasswordReset 重置密码
// @Summary 重置密码
// @Description 重置密码
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param userID path string true "userID"
// @Param data body dto.PasswordResetRequest true "data"
// @Success 200
// @Router /admin/api/user/{userID}/password-reset [put]
// @Security Bearer
func (e *User) PasswordReset(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	req := &dto.PasswordResetRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	req.UserID = ctx.Param("userID")
	var target models.User
	if err := center.Default.GetDB(ctx, &models.User{}).
		Preload("Role").
		First(&target, "id = ?", req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("get password reset target error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if target.Role != nil && target.Role.Root && !verify.Root() {
		api.Err(http.StatusForbidden)
		return
	}
	err := models.PasswordReset(ctx, req.UserID, req.Password)
	if err != nil {
		api.AddError(err).Log.Error("PasswordReset error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Create 创建用户
// @Summary 创建用户
// @Description 创建用户
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param data body models.User true "data"
// @Success 201 {object} models.User
// @Router /admin/api/users [post]
// @Security Bearer
func (e *User) Create(*gin.Context) {}

// Update 更新用户
// @Summary 更新用户
// @Description 更新用户
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.User true "data"
// @Success 200 {object} models.User
// @Router /admin/api/users/{id} [put]
// @Security Bearer
func (e *User) Update(*gin.Context) {}

// Get 获取用户
// @Summary 获取用户
// @Description 获取用户
// @Tags user
// @Param id path string true "id"
// @Success 200 {object} models.User
// @Router /admin/api/users/{id} [get]
// @Security Bearer
func (e *User) Get(*gin.Context) {}

// List 用户列表
// @Summary 用户列表
// @Description 用户列表
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Success 200 {object} response.Page{data=[]models.User}
// @Router /admin/api/users [get]
// @Security Bearer
func (e *User) List(*gin.Context) {}

// Delete 删除用户
// @Summary 删除用户
// @Description 删除用户
// @Tags user
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/users/{id} [delete]
// @Security Bearer
func (e *User) Delete(*gin.Context) {}

// Callback oauth2回调
// @Summary oauth2回调
// @Description oauth2回调
// @Tags user
// @Accept  application/json
// @Product application/json
// @Param provider path string true "provider"
// @Param data body dto.OauthCallbackReq true "OAuth callback code and state"
// @Success 201 {object} dto.OAuthCallbackResponse
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/user/{provider}/callback [post]
func (e *User) Callback(c *gin.Context) {
	e.oauthCallback(c)
}
