package apis

import (
	"github.com/mss-boot-io/mss-boot-admin-api/app/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/app/admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/8 18:14:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/8 18:14:12
 */

func init() {
	e := &Tenant{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Tenant)),
			controller.WithSearch(new(dto.TenantSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Tenant struct {
	*controller.Simple
}
