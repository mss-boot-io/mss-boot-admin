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

	controllers := map[string]struct {
		controller interface {
			GetAction(string) response.Action
		}
		genericReads bool
	}{
		"audit log": {
			controller: &AuditLogAPI{Simple: controller.NewSimple(
				controller.WithModel(new(models.AuditLog)),
				controller.WithSearch(new(dto.AuditLogSearch)),
				controller.WithModelProvider(actions.ModelProviderGorm),
			)},
			// AuditLog has dedicated bounded projection routes because the raw
			// model contains request and response bodies.
			genericReads: false,
		},
		"alert history": {
			controller: &AlertHistory{Simple: controller.NewSimple(
				controller.WithModel(new(models.AlertHistory)),
				controller.WithSearch(new(dto.AlertHistorySearch)),
				controller.WithModelProvider(actions.ModelProviderGorm),
			)},
			genericReads: true,
		},
	}

	for name, fixture := range controllers {
		t.Run(name, func(t *testing.T) {
			for _, read := range []string{response.Get, response.Search} {
				available := fixture.controller.GetAction(read) != nil
				if available != fixture.genericReads {
					t.Fatalf("generic read action %q availability = %t, want %t", read, available, fixture.genericReads)
				}
			}
			for _, mutation := range []string{response.Control, response.Delete} {
				if fixture.controller.GetAction(mutation) != nil {
					t.Fatalf("mutation action %q must not be exposed", mutation)
				}
			}
		})
	}
}
