package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"time"
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
	r.GET("/user/refresh-token", middleware.Auth.RefreshHandler)
	r.GET("/userInfo", e.UserInfo)
}

func (e *User) Login(ctx *gin.Context) {
	api := response.Make(ctx)
	api.OK("登录成功")
}

func (e *User) UserInfo(ctx *gin.Context) {
	api := response.Make(ctx)
	user := &models.User{
		UserLogin: models.UserLogin{
			Email: "wangliqun@email.com",
		},
		Name:             "王立群",
		Avatar:           "https://lf1-xgcdn-tos.pstatp.com/obj/vcloud/vadmin/start.8e0e4855ee346a46ccff8ff3e24db27b.png",
		Job:              "frontend",
		JobName:          "前端开发工程师",
		Organization:     "Frontend",
		OrganizationName: "前端",
		Location:         "beijing",
		LocationName:     "北京",
		Introduction:     "王力群并非是一个真实存在的人。",
		PersonalWebsite:  "https://www.arco.design",
		Verified:         true,
		PhoneNumber:      "18012345678",
		AccountID:        "1234567890",
		RegistrationTime: time.Now(),
		Permissions: map[string][]string{
			"menu.dashboard.workplace": {"*"},
		},
	}
	api.OK(user)
}
