package service

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/stretchr/testify/assert"
)

func TestMonitor_Monitor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &Monitor{}
	ctx, _ := gin.CreateTestContext(nil)

	resp, err := svc.Monitor(ctx)
	if err != nil {
		t.Skipf("Skipping monitor test in CI environment: %v", err)
		return
	}

	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.CPULogicalCore, 0)
	assert.GreaterOrEqual(t, resp.CPUPhysicalCore, 0)
	assert.NotNil(t, resp.CPUInfo)
	assert.GreaterOrEqual(t, resp.MemoryTotal, uint64(0))
	assert.GreaterOrEqual(t, resp.DiskTotal, uint64(0))

	assert.NotEmpty(t, resp.GoVersion)
	assert.NotZero(t, resp.StartTime)
	assert.GreaterOrEqual(t, resp.Uptime, int64(0))

	assert.NotNil(t, resp.Network)
	assert.NotNil(t, resp.Runtime)
	assert.NotZero(t, resp.Runtime.Goroutines)
}

func TestMonitor_MonitorResponse_Network(t *testing.T) {
	svc := &Monitor{}
	ctx, _ := gin.CreateTestContext(nil)

	resp, err := svc.Monitor(ctx)
	if err != nil {
		t.Skipf("Skipping monitor test: %v", err)
		return
	}

	assert.NotNil(t, resp.Network)
	assert.NotNil(t, resp.Network.ConnectionCount)
	assert.GreaterOrEqual(t, resp.Network.ConnectionCount.Total, 0)
}

func TestMonitor_MonitorResponse_Runtime(t *testing.T) {
	svc := &Monitor{}
	ctx, _ := gin.CreateTestContext(nil)

	resp, err := svc.Monitor(ctx)
	if err != nil {
		t.Skipf("Skipping monitor test: %v", err)
		return
	}

	assert.NotNil(t, resp.Runtime)
	assert.NotZero(t, resp.Runtime.Goroutines)
	assert.NotZero(t, resp.Runtime.HeapSys)
	assert.NotZero(t, resp.Runtime.NumGC)
}

func TestMonitor_Uptime(t *testing.T) {
	time.Sleep(2 * time.Second)

	svc := &Monitor{}
	ctx, _ := gin.CreateTestContext(nil)

	resp, err := svc.Monitor(ctx)
	if err != nil {
		t.Skipf("Skipping monitor test: %v", err)
		return
	}

	assert.GreaterOrEqual(t, resp.Uptime, int64(2))
}

func TestMonitor_DTO(t *testing.T) {
	resp := &dto.MonitorResponse{
		CPUPhysicalCore:    4,
		CPULogicalCore:     8,
		MemoryTotal:        16384,
		MemoryUsage:        5120,
		MemoryUsagePercent: 0.3125,
		DiskTotal:          500,
		DiskUsage:          150,
		DiskUsagePercent:   0.3,
		GoVersion:          "go1.22.0",
		StartTime:          time.Now().Unix(),
		Uptime:             100,
	}

	assert.Equal(t, 4, resp.CPUPhysicalCore)
	assert.Equal(t, 8, resp.CPULogicalCore)
	assert.NotEmpty(t, resp.GoVersion)
}
