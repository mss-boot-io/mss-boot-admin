package apis

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

func TestMonitorAPIHistoryLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		query     string
		wantCode  int
		wantLimit int
		wantCalls int
	}{
		{name: "default", wantCode: http.StatusOK, wantLimit: 120, wantCalls: 1},
		{name: "zero", query: "?historyLimit=0", wantCode: http.StatusOK, wantLimit: 0, wantCalls: 1},
		{name: "maximum", query: "?historyLimit=120", wantCode: http.StatusOK, wantLimit: 120, wantCalls: 1},
		{name: "negative", query: "?historyLimit=-1", wantCode: http.StatusBadRequest},
		{name: "too-large", query: "?historyLimit=121", wantCode: http.StatusBadRequest},
		{name: "not-a-number", query: "?historyLimit=nope", wantCode: http.StatusBadRequest},
		{name: "empty", query: "?historyLimit=", wantCode: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &monitorSnapshotStub{response: &dto.MonitorResponse{History: make([]dto.MonitorHistoryPoint, 0)}}
			recorder := executeMonitorRequest(newMonitorController(stub), test.query)
			if recorder.Code != test.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.wantCode, recorder.Body.String())
			}
			if stub.calls != test.wantCalls {
				t.Fatalf("Snapshot() calls = %d, want %d", stub.calls, test.wantCalls)
			}
			if test.wantCalls > 0 && stub.historyLimit != test.wantLimit {
				t.Fatalf("history limit = %d, want %d", stub.historyLimit, test.wantLimit)
			}
		})
	}
}

func TestMonitorAPINotReadyReturnsRetryableServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &monitorSnapshotStub{
		err:      service.ErrMonitorNotReady,
		interval: service.DefaultMonitorSampleInterval,
	}
	recorder := executeMonitorRequest(newMonitorController(stub), "")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, want 5", got)
	}
}

func TestMonitorAPIEmitsBackgroundHistoryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &monitorSnapshotStub{response: &dto.MonitorResponse{
		CollectedAt:      1_700_000_000_000,
		SampleIntervalMS: 5_000,
		Stale:            true,
		InstanceID:       "instance-1",
		History: []dto.MonitorHistoryPoint{{
			Timestamp:          1_700_000_000_000,
			CPUUsage:           12.5,
			MemoryUsagePercent: 45.5,
		}},
	}}
	recorder := executeMonitorRequest(newMonitorController(stub), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		CollectedAt      int64  `json:"collectedAt"`
		SampleIntervalMS int64  `json:"sampleIntervalMs"`
		Stale            bool   `json:"stale"`
		InstanceID       string `json:"instanceId"`
		History          []struct {
			Timestamp          int64   `json:"timestamp"`
			CPUUsage           float64 `json:"cpuUsage"`
			MemoryUsagePercent float64 `json:"memoryUsagePercent"`
		} `json:"history"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode monitor response: %v", err)
	}
	if payload.CollectedAt != 1_700_000_000_000 || payload.SampleIntervalMS != 5_000 ||
		!payload.Stale || payload.InstanceID != "instance-1" || len(payload.History) != 1 ||
		payload.History[0].Timestamp != 1_700_000_000_000 || payload.History[0].CPUUsage != 12.5 ||
		payload.History[0].MemoryUsagePercent != 45.5 {
		t.Fatalf("monitor contract payload = %#v", payload)
	}
}

func TestMonitorAPIUnexpectedFailureIsInternalServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &monitorSnapshotStub{err: errors.New("boom")}
	recorder := executeMonitorRequest(newMonitorController(stub), "")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
}

type monitorSnapshotStub struct {
	response     *dto.MonitorResponse
	err          error
	interval     time.Duration
	historyLimit int
	calls        int
}

func (e *monitorSnapshotStub) Snapshot(historyLimit int) (*dto.MonitorResponse, error) {
	e.calls++
	e.historyLimit = historyLimit
	return e.response, e.err
}

func (e *monitorSnapshotStub) SampleInterval() time.Duration {
	return e.interval
}

func executeMonitorRequest(controller *Monitor, query string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/monitor"+query, nil)
	controller.Monitor(ctx)
	return recorder
}
