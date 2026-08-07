package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
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
	r.POST("/user-auth-tokens", response.AuthHandler, e.Generate)
	r.GET("/user-auth-token/generate", methodNotAllowed)
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
// @Success 200 {object} dto.UserAuthTokenSecretResponse
// @Router /admin/api/user-auth-token/{id}/refresh [put]
// @Security Bearer
func (e *UserAuthToken) Refresh(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	id := ctx.Param("id")
	userAuthToken, token, err := models.RotateUserAuthToken(ctx, middleware.Auth, verify, id)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			api.Err(http.StatusNotFound)
		case errors.Is(err, models.ErrUserAuthTokenRevoked),
			errors.Is(err, models.ErrUserAuthTokenInvalidDigest):
			api.Err(http.StatusForbidden)
		case errors.Is(err, models.ErrUserAuthTokenRotationConflict):
			api.Err(http.StatusConflict)
		default:
			api.AddError(err).Log.Error("refresh user auth token failed")
			api.Err(http.StatusInternalServerError)
		}
		return
	}
	setSecretResponseHeaders(ctx)
	api.OK(userAuthTokenSecretResponse(userAuthToken, token))
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
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	id := ctx.Param("id")
	err := center.GetDB(ctx, &models.UserAuthToken{}).
		Model(&models.UserAuthToken{}).
		Where("id = ?", id).
		Where("user_id = ?", verify.GetUserID()).
		Where("revoked = ?", false).
		Updates(map[string]any{
			"revoked": true,
			"token":   "",
		}).Error
	if err != nil {
		api.AddError(err).Log.Error("revoke user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

// Generate 生成用户令牌
// @Summary 生成用户令牌
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Param validityPeriod query string true "有效期"
// @Success 201 {object} dto.UserAuthTokenSecretResponse
// @Router /admin/api/user-auth-tokens [post]
// @Security Bearer
func (e *UserAuthToken) Generate(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	req := &dto.UserAuthTokenGenerateRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	userAuthToken, token, err := models.GenerateUserAuthToken(ctx, middleware.Auth, verify, req.ValidityPeriod)
	if err != nil {
		api.AddError(err).Log.Error("generate user auth token failed")
		api.Err(http.StatusInternalServerError)
		return
	}
	setSecretResponseHeaders(ctx)
	api.OK(userAuthTokenSecretResponse(userAuthToken, token))
}

// List 列表
// @Summary 列表
// @Tags UserAuthToken
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Page{data=[]dto.UserAuthTokenSummary}
// @Router /admin/api/user-auth-tokens [get]
// @Security Bearer
func (e *UserAuthToken) List(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	if verify == nil || middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	list := make([]dto.UserAuthTokenSummary, 0)
	err := center.GetDB(ctx, &models.UserAuthToken{}).
		Model(&models.UserAuthToken{}).
		Select("id", "user_id", "fingerprint", "expired_at", "revoked", "created_at", "updated_at").
		Where("user_id = ?", verify.GetUserID()).
		Where("revoked = ?", false).
		Order("created_at desc").Find(&list).Error
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	api.PageOK(list, int64(len(list)), 0, 999)
}

func userAuthTokenSummary(userAuthToken *models.UserAuthToken) dto.UserAuthTokenSummary {
	return dto.UserAuthTokenSummary{
		ID:          userAuthToken.ID,
		UserID:      userAuthToken.UserID,
		Fingerprint: userAuthToken.Fingerprint,
		ExpiredAt:   userAuthToken.ExpiredAt,
		Revoked:     userAuthToken.Revoked,
		CreatedAt:   userAuthToken.CreatedAt,
		UpdatedAt:   userAuthToken.UpdatedAt,
	}
}

func userAuthTokenSecretResponse(userAuthToken *models.UserAuthToken, token string) dto.UserAuthTokenSecretResponse {
	return dto.UserAuthTokenSecretResponse{
		UserAuthTokenSummary: userAuthTokenSummary(userAuthToken),
		Token:                token,
	}
}

func setSecretResponseHeaders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
}
