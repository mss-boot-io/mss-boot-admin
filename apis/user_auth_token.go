package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/30 14:04:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/30 14:04:12
 */

func init() {
	e := &UserAuthToken{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.UserAuthToken)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type UserAuthToken struct {
	*controller.Simple
}

func (e *UserAuthToken) GetAction(_ string) response.Action {
	return nil
}

func (e *UserAuthToken) Other(r *gin.RouterGroup) {
	r.GET("/user-auth-token/generate", response.AuthHandler, e.Generate)
	r.GET("/user-auth-tokens", response.AuthHandler, e.List)
	r.PUT("/user-auth-token/:id/revoke", response.AuthHandler, e.Revoked)
	r.PUT("/user-auth-token/:id/refresh", response.AuthHandler, e.Refresh)
}

// Refresh 刷新用户令牌
// @Summary 刷新用户令牌
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 200 {object} models.UserAuthToken
// @Router /admin/api/user-auth-token/{id}/refresh [put]
// @Security Bearer
func (e *UserAuthToken) Refresh(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	id := ctx.Param("id")
	userAuthToken := &models.UserAuthToken{}
	err := center.GetDB(ctx, userAuthToken).
		Where("id = ?", id).
		Where("user_id = ?", verify.GetUserID()).
		First(userAuthToken).Error
	if err != nil {
		api.AddError(err).Log.Error("refresh user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	if userAuthToken.Revoked {
		api.Err(http.StatusForbidden)
		return
	}
	userAuthToken.Token, userAuthToken.ExpiredAt, err = middleware.Auth.TokenGenerator(verify)
	if err != nil {
		api.AddError(err).Log.Error("refresh user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	err = center.GetDB(ctx, userAuthToken).Save(userAuthToken).Error
	if err != nil {
		api.AddError(err).Log.Error("refresh user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(userAuthToken)
}

// Revoked 撤销用户令牌
// @Summary 撤销用户令牌
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 200
// @Router /admin/api/user-auth-token/{id}/revoke [put]
// @Security Bearer
func (e *UserAuthToken) Revoked(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	id := ctx.Param("id")
	err := center.GetDB(ctx, &models.UserAuthToken{}).
		Where("id = ?", id).
		Where("user_id = ?", verify.GetUserID()).
		Updates(&models.UserAuthToken{
			Revoked: true,
		}).Error
	if err != nil {
		api.AddError(err).Log.Error("revoke user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// Generate 生成用户令牌
// @Summary 生成用户令牌
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Param validityPeriod query string true "有效期"
// @Success 200 {object} models.UserAuthToken
// @Router /admin/api/user-auth-token/generate [get]
// @Security Bearer
func (e *UserAuthToken) Generate(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	req := &dto.UserAuthTokenGenerateRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	userAuthToken, err := models.GenerateUserAuthToken(ctx, verify, req.ValidityPeriod)
	if err != nil {
		api.AddError(err).Log.Error("generate user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(userAuthToken)
}

// List 列表
// @Summary 列表
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Page{data=[]models.UserAuthToken}
// @Router /admin/api/user-auth-tokens [get]
// @Security Bearer
func (e *UserAuthToken) List(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	list := make([]*models.UserAuthToken, 0)
	err := center.GetDB(ctx, &models.UserAuthToken{}).
		Where("user_id = ?", verify.GetUserID()).
		Where("revoked = ?", false).
		Order("created_at desc").Find(&list).Error
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	api.PageOK(list, int64(len(list)), 0, 999)
}
