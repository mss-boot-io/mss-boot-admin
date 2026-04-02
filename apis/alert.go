package apis

import (
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

func init() {
	e := &AlertRule{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.AlertRule{}),
			controller.WithSearch(&dto.AlertRuleSearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)

	h := &AlertHistory{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.AlertHistory{}),
			controller.WithSearch(&dto.AlertHistorySearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(h)
}

type AlertRule struct {
	*controller.Simple
}

func (*AlertRule) Path() string {
	return "alert-rule"
}

type AlertHistory struct {
	*controller.Simple
}

func (*AlertHistory) Path() string {
	return "alert-history"
}