package apis

import (
	"net/http"
	"time"

	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/6 01:28:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/6 01:28:48
 */

func init() {
	e := &Lark{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type Lark struct {
	*controller.Simple
}

func (*Lark) GetKey() string {
	return "lark"
}

func (*Lark) GetAction(string) response.Action {
	return nil
}

func (e *Lark) Other(r *gin.RouterGroup) {
	r.GET("/lark/callback", e.Callback)
}

// Callback lark回调
// @Summary lark回调
// @Description lark回调
// @Tags lark
// @Accept  application/json
// @Product application/json
// @Param code query string true "code"
// @Param state query string true "state"
// @Success 200 {object} dto.OauthToken
// @Failure 422 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/api/lark/callback [get]
func (e *Lark) Callback(c *gin.Context) {
	api := response.Make(c)
	r := &dto.OauthCallbackReq{}
	if api.Bind(r).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	appID, _ := center.GetAppConfig().GetAppConfig(c, "security.larkAppId")
	appSecret, _ := center.GetAppConfig().GetAppConfig(c, "security.larkAppSecret")
	client := lark.NewClient(appID, appSecret)
	req := larkauthen.NewCreateAccessTokenReqBuilder().
		Body(larkauthen.NewCreateAccessTokenReqBodyBuilder().
			GrantType(`authorization_code`).
			Code(r.Code).
			Build()).Build()

	// 发起请求
	resp, err := client.Authen.AccessToken.Create(c, req)
	if err != nil {
		api.AddError(err).Err(http.StatusUnauthorized)
		return
	}

	// 服务端错误处理
	if !resp.Success() {
		api.Err(http.StatusUnauthorized)
		return
	}
	expiry := time.Now().Add(time.Duration(*resp.Data.ExpiresIn) * time.Second)
	refreshExpiry := time.Now().Add(time.Duration(*resp.Data.RefreshExpiresIn) * time.Second)

	result := &dto.OauthToken{
		AccessToken:   *resp.Data.AccessToken,
		TokenType:     *resp.Data.TokenType,
		RefreshToken:  *resp.Data.RefreshToken,
		Expiry:        &expiry,
		RefreshExpiry: &refreshExpiry,
	}
	api.OK(result)
}
