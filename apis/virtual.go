package apis

import (
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/virtual/action"
	"github.com/mss-boot-io/mss-boot/virtual/api"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 09:03:13
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 09:03:13
 */

type Virtual struct {
	*api.Virtual
}

func init() {
	e := &Virtual{
		Virtual: api.NewVirtual(
			action.GetBase(),
			//controller.WithAuth(true),
		),
	}
	response.AppendController(e)
}
