package apis

import (
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/virtual"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 09:03:13
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 09:03:13
 */

type Virtual struct {
	*controller.Virtual
}

func init() {
	e := &Virtual{
		Virtual: controller.NewVirtual(
			virtual.GetBase(),
			//controller.WithAuth(true),
		),
	}
	response.AppendController(e)
}
