package apis

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	runtimechallenge "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/challenge"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

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
	response.AppendController(newUserController())
}

func newUserController() *User {
	return &User{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.User)),
			controller.WithSearch(new(dto.UserSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(redactUserPersistenceDiagnostics),
			controller.WithWriteErrorMapper(mapUserWriteError),
			controller.WithAfterCommitCreate(recordCreatedUserStatistics),
			controller.WithAfterDelete(recordDeletedUserStatistics),
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement, protectRootUserLifecycle}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement, protectRootUserLifecycle}),
		),
	}
}

func recordCreatedUserStatistics(ctx *gin.Context, _ *gorm.DB, model schema.Tabler) error {
	user, ok := model.(*models.User)
	if !ok {
		return errors.New("created user statistics model is invalid")
	}
	models.RecordUserCreated(ctx, user)
	return nil
}

func recordDeletedUserStatistics(ctx *gin.Context, _ *gorm.DB, model schema.Tabler) error {
	user, ok := model.(*models.User)
	if !ok {
		return errors.New("deleted user statistics model is invalid")
	}
	models.RecordUserDeleted(ctx, user)
	return nil
}

func mapUserWriteError(
	_ *gin.Context,
	_ actions.WriteOperation,
	err error,
) (actions.PublicWriteError, bool) {
	err = models.NormalizeEmailIdentityPersistenceError(err)
	switch {
	case errors.Is(err, models.ErrEmailIdentityInvalid):
		return actions.PublicWriteError{
			Status: http.StatusUnprocessableEntity,
			Error:  response.NewError("INVALID_EMAIL_IDENTITY", "email identity is invalid"),
		}, true
	case errors.Is(err, models.ErrEmailIdentityExists),
		errors.Is(err, models.ErrEmailIdentityAmbiguous):
		return actions.PublicWriteError{
			Status: http.StatusConflict,
			Error:  response.NewError("EMAIL_IDENTITY_UNAVAILABLE", "email identity is unavailable"),
		}, true
	default:
		return actions.PublicWriteError{}, false
	}
}

// redactUserPersistenceDiagnostics keeps the generic user controller's GORM
// logger from interpolating request identities, password material, SQL, or
// driver constraint text before the public write-error mapper can classify the
// result. The mapper emits a fixed operation and error code for diagnostics.
func redactUserPersistenceDiagnostics(_ *gin.Context, _ schema.Tabler) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Session(&gorm.Session{Logger: logger.Discard})
	}
}

type User struct {
	*controller.Simple
	oauthStates          oauthStateStore
	oauthURLBuilder      oauthAuthorizeURLBuilder
	oauthCodeExchange    oauthCodeExchange
	oauthLoginComplete   oauthLoginCompleter
	oauthBindingComplete oauthBindingCompleter
	oauthReauthComplete  oauthReauthenticationCompleter
	challengeSender      email.VerifyCodeSender
	challengeSendSlots   chan struct{}
}

var defaultEmailChallengeSendSlots = make(chan struct{}, 32)

// Other handler
func (e *User) Other(r *gin.RouterGroup) {
	r.POST("/user/login", middleware.PublicLoginHandler)
	r.POST("/user/session/login", middleware.RequireTrustedBrowserOrigin(), e.SessionLogin)
	r.POST("/user/session/refresh-token", middleware.RequireTrustedBrowserOrigin(), e.SessionRefreshToken)
	r.POST("/user/auth-cookie/clear", middleware.RequireTrustedBrowserOrigin(), e.ClearAuthCookie)
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
	r.GET("/user/security", response.AuthHandler, e.GetAccountSecurity)
	r.POST("/user/security/reauthenticate", response.AuthHandler, e.ReauthenticateAccount)
	r.PUT("/user/security/password", response.AuthHandler, e.ChangeAccountPassword)
	r.POST("/user/oauth2/authorize", middleware.OptionalAuth(), e.OAuthAuthorize)
	r.POST("/user/session/oauth2/authorize", middleware.RequireTrustedBrowserOrigin(), middleware.OptionalAuth(), e.SessionOAuthAuthorize)
	r.POST("/user/binding", response.AuthHandler, e.Binding)
	r.DELETE("/user/unbinding", response.AuthHandler, e.Unbinding)
	r.DELETE("/user/oauth2/:provider", response.AuthHandler, e.DisconnectOAuth)
	r.POST("/user/:provider/callback", middleware.OptionalAuth(), e.Callback)
	r.POST("/user/session/:provider/callback", middleware.RequireTrustedBrowserOrigin(), middleware.OptionalAuth(), e.SessionCallback)
	r.GET("/user/:provider/callback", methodNotAllowed)
}

// SessionLogin establishes the opt-in V6 browser session without returning an
// Admin JWT to browser-visible JavaScript.
// @Summary Login with an HttpOnly browser session
// @Tags user
// @Accept json
// @Produce json
// @Param data body models.UserLogin true "data"
// @Success 200 {object} dto.BrowserSessionResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/user/session/login [post]
func (*User) SessionLogin(c *gin.Context) {
	middleware.BrowserSessionLoginHandler(c)
}

// SessionRefreshToken rotates the V6 HttpOnly session and signed CSRF cookie.
// @Summary Refresh an HttpOnly browser session
// @Tags user
// @Produce json
// @Success 200 {object} dto.BrowserSessionResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/user/session/refresh-token [post]
func (*User) SessionRefreshToken(c *gin.Context) {
	middleware.BrowserSessionRefreshHandler(c)
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
	ctx.Header("Cache-Control", "no-store")
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
	e.disconnectOAuth(ctx, verify.GetUserID(), req.Provider)
}

// DisconnectOAuth removes one provider only after recent proof and while
// preserving at least one verified login method.
func (e *User) DisconnectOAuth(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	verify := response.VerifyHandler(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		response.Make(ctx).Err(http.StatusForbidden)
		return
	}
	provider := pkg.LoginProvider(strings.ToLower(strings.TrimSpace(ctx.Param("provider"))))
	e.disconnectOAuth(ctx, verify.GetUserID(), provider)
}

func (*User) disconnectOAuth(ctx *gin.Context, userID string, provider pkg.LoginProvider) {
	api := response.Make(ctx)
	if provider != pkg.GithubLoginProvider && provider != pkg.LarkLoginProvider {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := service.Session.DisconnectOAuth(
		ctx,
		center.Default.GetDB(ctx, &models.UserOAuth2{}),
		middleware.CurrentSessionID(ctx),
		userID,
		provider,
	)
	if err != nil {
		writeAccountSecurityError(ctx, api, err)
		return
	}
	ctx.Status(http.StatusNoContent)
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
	ctx.Header("Cache-Control", "no-store")
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

// GetAccountSecurity returns only safe credential capabilities and bounded
// recent-authentication metadata for the current server session.
func (*User) GetAccountSecurity(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	user := &models.User{}
	if err := center.Default.GetDB(ctx, user).
		Select("id", "local_password_disabled", "password_hash", "salt").
		Where("id = ?", verify.GetUserID()).
		First(user).Error; err != nil {
		api.AddError(err).Log.Error("load account security capability failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	status, err := service.Session.RecentAuthenticationStatus(
		ctx,
		center.Default.GetDB(ctx, &models.UserSession{}),
		middleware.CurrentSessionID(ctx),
		verify.GetUserID(),
	)
	if err != nil {
		writeAccountSecurityError(ctx, api, err)
		return
	}
	api.OK(&dto.AccountSecurityStatus{
		HasLocalPassword:              !user.LocalPasswordDisabled && user.PasswordHash != "" && user.Salt != "",
		RecentAuthentication:          status.Recent,
		RecentAuthenticationExpiresAt: status.ExpiresAt,
		ReauthenticationLockedUntil:   status.LockedUntil,
	})
}

// ReauthenticateAccount proves the current local password and records the
// result only against the current durable browser session.
func (*User) ReauthenticateAccount(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	req := &dto.AccountReauthenticationRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := service.Session.ReauthenticateWithPassword(
		ctx,
		center.Default.GetDB(ctx, &models.UserSession{}),
		middleware.CurrentSessionID(ctx),
		verify.GetUserID(),
		req.Password,
	)
	req.Password = ""
	if err != nil {
		writeAccountSecurityError(ctx, api, err)
		return
	}
	status, err := service.Session.RecentAuthenticationStatus(
		ctx,
		center.Default.GetDB(ctx, &models.UserSession{}),
		middleware.CurrentSessionID(ctx),
		verify.GetUserID(),
	)
	if err != nil {
		writeAccountSecurityError(ctx, api, err)
		return
	}
	api.OK(&dto.AccountSecurityStatus{
		RecentAuthentication:          status.Recent,
		RecentAuthenticationExpiresAt: status.ExpiresAt,
		ReauthenticationLockedUntil:   status.LockedUntil,
	})
}

// ChangeAccountPassword rotates the one-way password verifier and revokes all
// existing sessions and PATs. The initiating browser is signed out as well.
func (*User) ChangeAccountPassword(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	req := &dto.AccountPasswordChangeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := service.Session.ChangePassword(
		ctx,
		center.Default.GetDB(ctx, &models.UserSession{}),
		middleware.CurrentSessionID(ctx),
		verify.GetUserID(),
		req.NewPassword,
	)
	req.NewPassword = ""
	if err != nil {
		writeAccountSecurityError(ctx, api, err)
		return
	}
	middleware.ClearBrowserSessionCookies(ctx)
	api.OK(&dto.AccountPasswordChangeResponse{SignedOut: true})
}

func writeAccountSecurityError(ctx *gin.Context, api *response.API, err error) {
	if api == nil {
		return
	}
	code := "ACCOUNT_SECURITY_UNAVAILABLE"
	message := "account security operation is unavailable"
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrSecuritySessionUnavailable):
		code, message, status = "SECURITY_SESSION_REQUIRED", "an interactive server session is required", http.StatusUnauthorized
	case errors.Is(err, service.ErrRecentAuthenticationRequired):
		code, message, status = "RECENT_AUTHENTICATION_REQUIRED", "recent authentication is required", http.StatusPreconditionRequired
	case errors.Is(err, service.ErrReauthenticationLocked):
		code, message, status = "REAUTHENTICATION_LOCKED", "reauthentication is temporarily locked", http.StatusTooManyRequests
		ctx.Header("Retry-After", "300")
	case errors.Is(err, service.ErrInvalidCurrentPassword):
		code, message, status = "REAUTHENTICATION_FAILED", "identity verification failed", http.StatusForbidden
	case errors.Is(err, service.ErrLocalPasswordUnavailable):
		code, message, status = "LOCAL_PASSWORD_UNAVAILABLE", "use a connected provider to verify your identity", http.StatusConflict
	case errors.Is(err, service.ErrPasswordPolicy):
		code, message, status = "PASSWORD_POLICY_FAILED", "password must be 8 to 128 characters and contain letters and numbers", http.StatusUnprocessableEntity
	case errors.Is(err, service.ErrPasswordUnchanged):
		code, message, status = "PASSWORD_UNCHANGED", "new password must differ from the current password", http.StatusConflict
	case errors.Is(err, service.ErrOAuthBindingNotFound):
		code, message, status = "OAUTH_BINDING_NOT_FOUND", "oauth connection was not found", http.StatusNotFound
	case errors.Is(err, service.ErrFinalLoginMethod):
		code, message, status = "FINAL_LOGIN_METHOD", "the final verified login method cannot be disconnected", http.StatusConflict
	default:
		api.AddError(err).Log.Error("account security operation failed")
	}
	api.AddError(response.NewError(code, message)).Err(status)
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
	ctx.Header("Cache-Control", "no-store")
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
		err := service.Session.ChangePassword(
			ctx,
			center.Default.GetDB(ctx, &models.UserSession{}),
			middleware.CurrentSessionID(ctx),
			verify.GetUserID(),
			req.Password,
		)
		req.Password = ""
		if err != nil {
			writeAccountSecurityError(ctx, api, err)
			return
		}
		middleware.ClearBrowserSessionCookies(ctx)
		api.OK(&dto.AccountPasswordChangeResponse{SignedOut: true})
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
	challenge := center.GetRuntimeChallenge()
	if challenge == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	outcome, err := challenge.Verify(ctx.Request.Context(), runtimechallenge.VerifyRequest{
		Subject: req.Email,
		Purpose: runtimechallenge.Purpose(pkg.PasswordResetChallengePurpose),
		Code:    req.Captcha,
	})
	if err != nil {
		api.AddError(runtimechallenge.ErrUnavailable).Log.Warn("password recovery challenge unavailable")
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if outcome != runtimechallenge.VerifyVerified {
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
// @Failure 503 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/api/user/avatar [post]
// @Security Bearer
func (e *User) UpdateAvatar(ctx *gin.Context) {
	api := response.Make(ctx)
	result, err := defaultUploadService.Upload(ctx, "file")
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
	updates, err := normalizeSelfProfileUpdates(reqMap)
	if err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if len(updates) == 0 {
		api.Err(http.StatusUnprocessableEntity)
		return
	}

	err = center.Default.GetDB(ctx, &models.User{}).
		Model(&models.User{}).
		Where("id = ?", verify.GetUserID()).
		Updates(updates).Error
	if err != nil {
		api.AddError(err).Log.Error("UpdateUserInfo error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

var selfProfileStringFields = map[string]string{
	"name":      "name",
	"avatar":    "avatar",
	"signature": "signature",
	"title":     "title",
	"group":     "group",
	"country":   "country",
	"province":  "province",
	"city":      "city",
	"address":   "address",
	"phone":     "phone",
	"profile":   "profile",
}

// normalizeSelfProfileUpdates translates the public JSON patch into an exact
// persistence allowlist. A map update is intentional: unlike a GORM struct it
// persists empty strings and empty tag arrays, allowing users to clear optional
// profile values. Authentication identities and unknown fields fail closed.
func normalizeSelfProfileUpdates(input map[string]any) (map[string]any, error) {
	updates := make(map[string]any, len(input))
	for field, value := range input {
		if column, ok := selfProfileStringFields[field]; ok {
			text, valid := value.(string)
			if !valid {
				return nil, errors.New("profile field must be a string")
			}
			updates[column] = text
			continue
		}
		if field == "tags" {
			values, valid := value.([]any)
			if !valid {
				return nil, errors.New("profile tags must be an array")
			}
			tags := make(models.ArrayString, 0, len(values))
			for _, value := range values {
				tag, valid := value.(string)
				if !valid {
					return nil, errors.New("profile tag must be a string")
				}
				tags = append(tags, tag)
			}
			updates[field] = tags
			continue
		}
		// Email is an authentication and recovery identity. It requires a
		// dedicated verified-change workflow; username and every other unknown
		// property are likewise outside this self-service profile contract.
		return nil, errors.New("profile field is not self-service mutable")
	}
	return updates, nil
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
	challenge := center.GetRuntimeChallenge()
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
	outcome, err := challenge.BeginIssue(requestCtx, runtimechallenge.BeginRequest{
		Caller:  emailChallengeCaller(ctx),
		Subject: req.Email,
		Purpose: runtimechallenge.Purpose(purpose),
	})
	if err != nil {
		if errors.Is(err, runtimechallenge.ErrInvalid) {
			api.Err(http.StatusUnprocessableEntity)
		} else {
			api.Log.Warn("email challenge delivery unavailable")
			api.Err(http.StatusServiceUnavailable)
		}
		return
	}
	if outcome.State() != runtimechallenge.BeginReserved {
		switch outcome.State() {
		case runtimechallenge.BeginPending, runtimechallenge.BeginCooldown, runtimechallenge.BeginQuota:
			api.Err(http.StatusTooManyRequests)
		default:
			api.Err(http.StatusServiceUnavailable)
		}
		return
	}
	reservation, reserved := outcome.Reservation()
	if !reserved {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if err := sender(requestCtx, smtpHost, smtpPort, username, password, recipientName, req.Email, reservation.Code(), organization); err != nil {
		abortCtx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), 2*time.Second)
		abortErr := challenge.Abort(abortCtx, reservation)
		cancel()
		if abortErr != nil {
			api.Log.Warn("email challenge delivery rollback unavailable")
		} else {
			api.Log.Warn("email challenge delivery unavailable")
		}
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if err := challenge.Commit(requestCtx, reservation); err != nil {
		api.Log.Warn("email challenge activation unavailable")
		api.Err(http.StatusServiceUnavailable)
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
