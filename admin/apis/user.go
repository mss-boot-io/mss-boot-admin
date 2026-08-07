package apis

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
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

var (
	errNotSupportEmail = errors.New("not support send email")
)

func init() {
	e := &User{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.User)),
			controller.WithSearch(new(dto.UserSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
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
	oauthCredentials     oauthCredentialStore
}

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
	ok, err := center.Default.VerifyCode(ctx, req.Email, req.Captcha)
	if err != nil {
		api.AddError(err).Log.Error("VerifyCode error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if !ok {
		api.Err(http.StatusForbidden)
		return
	}
	user, err := models.GetUserByEmail(ctx, req.Email)
	if err != nil {
		api.AddError(err).Log.Error("GetUser error")
		api.Err(http.StatusInternalServerError)
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

func (e *User) UpdateAvatar(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	file, err := ctx.FormFile("file")
	if err != nil {
		api.AddError(err).Log.Error("FormFile error")
		api.Err(http.StatusInternalServerError)
		return
	}
	s := service.Storage{}
	result, err := s.Upload(ctx, file, verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("upload error")
		api.Err(http.StatusInternalServerError)
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
	if v, ok := reqMap["email"].(string); ok {
		user.Email = v
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
// @Success 200 {object} dto.FakeCaptchaResponse
// @Router /admin/api/user/fakeCaptcha [post]
func (e *User) FakeCaptcha(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.FakeCaptchaRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	resp := &dto.FakeCaptchaResponse{}
	if req.Email != "" {
		// setup 01 get user by email
		user := &models.User{}
		user.Email = req.Email
		if req.UseBy != email.RegisterSender.String() {
			err := center.Default.
				GetDB(ctx, &models.User{}).
				Where("email = ?", req.Email).
				First(user).Error
			if err != nil {
				api.AddError(err)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					api.Err(http.StatusNotFound)
					return
				}
				api.Log.Error("GetUser error")
				api.Err(http.StatusInternalServerError)
				return
			}
		}
		// setup 02 generate verify code
		code, err := center.Default.GenerateCode(ctx, req.Email, 5*time.Minute)
		if err != nil {
			api.AddError(err).Log.Error("GenerateCode error")
			api.Err(http.StatusInternalServerError)
			return
		}
		// setup 03 send email
		smtpHost, ok := center.GetAppConfig().GetAppConfig(ctx, "email:smtpHost")
		if !ok {
			api.AddError(errNotSupportEmail).
				Err(http.StatusNotImplemented)
			return
		}
		smtpPort, ok := center.GetAppConfig().GetAppConfig(ctx, "email:smtpPort")
		if !ok {
			api.AddError(errNotSupportEmail).
				Err(http.StatusNotImplemented)
			return
		}
		username, ok := center.GetAppConfig().GetAppConfig(ctx, "email:username")
		if !ok {
			api.AddError(errNotSupportEmail).
				Err(http.StatusNotImplemented)
			return
		}
		password, ok := center.GetAppConfig().GetAppConfig(ctx, "email:password")
		if !ok {
			api.AddError(errNotSupportEmail).
				Err(http.StatusNotImplemented)
			return
		}
		organization, ok := center.GetAppConfig().GetAppConfig(ctx, "base:websiteName")
		if !ok || organization == "" {
			organization = "mss-boot-io"
		}
		var sender email.VerifyCodeSender
		switch req.UseBy {
		case email.RegisterSender.String(), email.LoginSender.String(), email.ResetPasswordSender.String():
			sender = email.Sender[email.SendType(req.UseBy)]
		default:
			api.AddError(errNotSupportEmail).
				Err(http.StatusNotImplemented)
			return
		}
		err = sender(smtpHost, smtpPort,
			username, password,
			user.Username,
			user.Email,
			code,
			organization)

		if err != nil {
			api.AddError(err).Log.Error("send email error")
			api.Err(http.StatusInternalServerError)
			return
		}

		resp.Status = "ok"
		api.OK(resp)
		return
	}
	err := fmt.Errorf("not support phone")
	api.AddError(err).Err(http.StatusNotImplemented)
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
	permissions, err := gormdb.Enforcer.GetFilteredPolicy(0, verify.GetRoleID(), pkg.MenuAccessType.String())
	if err != nil {
		api.AddError(err).Log.Error("get filtered policy error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	enforcers, err := gormdb.Enforcer.GetFilteredPolicy(0, verify.GetRoleID(), pkg.ComponentAccessType.String())
	if err != nil {
		api.AddError(err).Log.Error("get filtered policy error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	permissions = append(permissions, enforcers...)
	user.Permissions = make(map[string]bool)
	if verify.Root() {
		user.Permissions["canAdmin"] = true
	}
	for i := range permissions {
		if len(permissions[i]) < 4 {
			continue
		}
		if permissions[i][0] == verify.GetRoleID() &&
			(permissions[i][1] == pkg.MenuAccessType.String() ||
				permissions[i][1] == pkg.ComponentAccessType.String()) {
			user.Permissions[permissions[i][2]] = true
		}
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
	req := &dto.PasswordResetRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	req.UserID = ctx.Param("userID")
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
