package apis

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
)

func TestDurableHistoryControllersAreReadOnly(t *testing.T) {
	t.Parallel()

	controllers := map[string]interface {
		GetAction(string) response.Action
	}{
		"audit log": &AuditLogAPI{Simple: controller.NewSimple(
			controller.WithModel(new(models.AuditLog)),
			controller.WithSearch(new(dto.AuditLogSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		)},
		"alert history": &AlertHistory{Simple: controller.NewSimple(
			controller.WithModel(new(models.AlertHistory)),
			controller.WithSearch(new(dto.AlertHistorySearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		)},
	}

	for name, c := range controllers {
		t.Run(name, func(t *testing.T) {
			if c.GetAction(response.Get) == nil {
				t.Fatal("get action must remain available")
			}
			if c.GetAction(response.Search) == nil {
				t.Fatal("search action must remain available")
			}
			for _, mutation := range []string{response.Control, response.Delete} {
				if c.GetAction(mutation) != nil {
					t.Fatalf("mutation action %q must not be exposed", mutation)
				}
			}
		})
	}
}
