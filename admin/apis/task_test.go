package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/task"
)

func TestTaskOperateRejectsUserTaskActionsWhenSchedulerDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := task.UserSchedulesEnabled()
	task.New(task.WithUserSchedulesEnabled(false))
	t.Cleanup(func() { task.New(task.WithUserSchedulesEnabled(previous)) })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/api/tasks/example/actions/start", nil)
	ctx.Params = gin.Params{
		{Key: "id", Value: "example"},
		{Key: "operate", Value: "start"},
	}

	(&Task{}).Operate(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled task operation status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
