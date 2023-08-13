package apis

import (
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

func init() {
	e := &Message{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Message)),
			controller.WithSearch(new(dto.MessageSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Message struct {
	*controller.Simple
}

// GetAction get action
func (e *Message) GetAction(key string) response.Action {
	if key != response.Search {
		return nil
	}
	return e.Simple.GetAction(key)
}
