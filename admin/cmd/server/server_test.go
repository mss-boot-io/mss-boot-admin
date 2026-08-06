package server

import (
	"reflect"
	"testing"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

func TestSystemTaskSchedulesKeepInternalJobsWhenUserTasksDisabled(t *testing.T) {
	schedules := systemTaskSchedules(false, "ignored", service.DefaultMonitor, nil)
	if len(schedules) != 2 {
		t.Fatalf("disabled user-task schedules = %d, want monitor and session cleanup", len(schedules))
	}
	if schedules[0].key != "monitor-sampler" ||
		schedules[0].spec != service.DefaultMonitor.ScheduleSpec() ||
		schedules[0].job != service.DefaultMonitor {
		t.Fatalf("monitor system schedule = %#v", schedules[0])
	}
	if schedules[1].key != "session-cleanup" {
		t.Fatalf("second system schedule key = %q, want session-cleanup", schedules[1].key)
	}
}

func TestSystemTaskSchedulesAddDatabaseJobsOnlyWhenEnabled(t *testing.T) {
	const userTaskSpec = "0 */1 * * * *"
	schedules := systemTaskSchedules(true, userTaskSpec, service.DefaultMonitor, func(operation func(*gorm.DB) error) error {
		return operation(&gorm.DB{})
	})
	keys := make([]string, len(schedules))
	for i := range schedules {
		keys[i] = schedules[i].key
	}
	if want := []string{"monitor-sampler", "session-cleanup", "task"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("enabled system-task keys = %v, want %v", keys, want)
	}
	if schedules[2].spec != userTaskSpec {
		t.Fatalf("task reconciliation spec = %q, want %q", schedules[2].spec, userTaskSpec)
	}
}
