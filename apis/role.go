/*
 * @Author: lwnmengjing
 * @Date: 2023/5/1 19:51:49
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/5/1 19:51:49
 */

package apis

import (
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

func init() {
	e := &Role{
		Simple: controller.NewSimple(
			controller.WithAuth(false),
			controller.WithModel(new(models.Role)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Role struct {
	*controller.Simple
}
