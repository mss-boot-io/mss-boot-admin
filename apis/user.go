package apis

import (
	"errors"
	"gorm.io/gorm"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/service"
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
			controller.WithScope(center.Default.Scope),
		),
	}
	response.AppendController(e)
}

type User struct {
	*controller.Simple
}

// Other handler
func (e *User) Other(r *gin.RouterGroup) {
	r.POST("/user/login", middleware.Auth.LoginHandler)
	r.POST("/user/login/github", middleware.Auth.LoginHandler)
	r.GET("/user/refresh-token", middleware.Auth.RefreshHandler)
	r.GET("/user/userInfo", middleware.Auth.MiddlewareFunc(), e.UserInfo)
	r.PUT("/user/:userID/password-reset", e.PasswordReset)
	r.PUT("/user/userInfo", middleware.Auth.MiddlewareFunc(), e.UpdateUserInfo)
	r.POST("/user/avatar", middleware.Auth.MiddlewareFunc(), e.UpdateAvatar)
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
	filename, err := s.Upload(ctx, file, verify.GetTenantID(), verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("upload error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(dto.UpdateAvatarResponse{Avatar: filename})
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
	req := &dto.UpdateUserInfoRequest{}
	if api.Bind(req).Error != nil {
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
	user.Name = req.Name
	user.Email = req.Email
	user.Avatar = req.Avatar
	user.Signature = req.Signature
	user.Title = req.Title
	user.Group = req.Group
	user.Country = req.Country
	user.Province = req.Province
	user.City = req.City
	user.Address = req.Address
	user.Phone = req.Phone
	user.Profile = req.Profile
	user.Tags = req.Tags
	err = center.Default.GetDB(ctx, &models.User{}).Model(&models.User{}).Where("id = ?", verify.GetUserID()).Updates(user).Error
	if err != nil {
		api.AddError(err).Log.Error("UpdateUserInfo error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
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
// @Router /admin/api/user/refresh-token [get]
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
func (e *User) FakeCaptcha(*gin.Context) {}

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
	err := models.PasswordReset(ctx, req.UserID, req.Password)
	if err != nil {
		api.AddError(err).Log.Error("PasswordReset error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
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
// @Param page query int false "page"
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
