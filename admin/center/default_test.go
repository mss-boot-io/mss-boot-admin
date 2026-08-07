package center

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grafana/pyroscope-go"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
)

func TestDefaultCenterSettersAndGetters(t *testing.T) {
	manager := server.New(server.WithoutSignalHandling())
	router := gin.New()
	profiler := &pyroscope.Profiler{}
	center := &DefaultCenter{}

	center.SetNotice(nil)
	center.SetTenant(nil)
	center.SetVerify(nil)
	center.SetConfig(nil)
	center.SetCustomConfig(nil)
	center.SetServerManager(manager)
	center.SetRouter(router)
	center.SetAppConfig(nil)
	center.SetUserConfig(nil)
	center.SetProfiler(profiler)
	center.SetStatistics(nil)
	center.SetMakeRouter(nil)
	center.SetGRPCClient(nil)
	center.SetCache(nil)
	center.SetQueue(nil)
	center.SetLocker(nil)
	center.SetVerifyCodeStore(nil)

	if center.GetNotice() != nil || center.GetTenant() != nil || center.GetVerify() != nil {
		t.Fatal("nil composition dependencies were not preserved")
	}
	if center.GetConfig() != nil || center.GetCustomConfig() != nil {
		t.Fatal("nil configuration dependencies were not preserved")
	}
	if center.GetServerManager() != manager || center.GetRouter() != router {
		t.Fatal("runtime dependencies were not returned")
	}
	if center.GetAppConfig() != nil || center.GetUserConfig() != nil {
		t.Fatal("nil app configuration dependencies were not preserved")
	}
	if center.GetProfiler() != profiler {
		t.Fatal("profiler was not returned")
	}
	if center.GetStatistics() != nil || center.GetMakeRouter() != nil || center.GetGRPCClient() != nil {
		t.Fatal("nil service dependencies were not preserved")
	}
	if center.GetCache() != nil || center.GetQueue() != nil || center.GetLocker() != nil || center.GetVerifyCodeStore() != nil {
		t.Fatal("nil storage dependencies were not preserved")
	}
}

func TestGlobalCenterAccessorsUseCurrentDefault(t *testing.T) {
	previous := Default
	Default = &DefaultCenter{}
	t.Cleanup(func() { Default = previous })

	manager := server.New(server.WithoutSignalHandling())
	router := gin.New()
	profiler := &pyroscope.Profiler{}

	if SetNotice(nil) != Default || SetTenant(nil) != Default || SetVerify(nil) != Default {
		t.Fatal("global setters must return the active center")
	}
	SetConfig(nil)
	SetCustomConfig(nil)
	SetServerManager(manager)
	SetRouter(router)
	SetAppConfig(nil)
	SetUserConfig(nil)
	SetProfiler(profiler)
	SetStatistics(nil)
	SetMakeRouter(nil)
	SetGRPCClient(nil)
	SetCache(nil)
	SetQueue(nil)
	SetLocker(nil)
	SetVerifyCodeStore(nil)

	if GetNotice() != nil || GetTenant() != nil || GetUser() != nil {
		t.Fatal("unexpected global identity dependencies")
	}
	if GetConfig() != nil || GetCustomConfig() != nil {
		t.Fatal("unexpected global configuration dependencies")
	}
	if GetServerManager() != manager || GetRouter() != router || GetProfiler() != profiler {
		t.Fatal("global runtime accessors did not use Default")
	}
	if GetAppConfig() != nil || GetUserConfig() != nil || GetStatistics() != nil || GetMakeRouter() != nil || GetGRPCClient() != nil {
		t.Fatal("unexpected global service dependencies")
	}
	if GetCache() != nil || GetQueue() != nil || GetLocker() != nil || GetVerifyCodeStore() != nil {
		t.Fatal("unexpected global storage dependencies")
	}
}

func TestStageEnvironmentPrecedence(t *testing.T) {
	previousUpper, upperSet := os.LookupEnv("STAGE")
	previousLower, lowerSet := os.LookupEnv("stage")
	t.Cleanup(func() {
		restoreEnvironment("STAGE", previousUpper, upperSet)
		restoreEnvironment("stage", previousLower, lowerSet)
	})

	_ = os.Unsetenv("STAGE")
	_ = os.Unsetenv("stage")
	if got := (&DefaultCenter{}).Stage(); got != "local" {
		t.Fatalf("default stage = %q", got)
	}
	_ = os.Setenv("stage", "dev")
	if got := (&DefaultCenter{}).Stage(); got != "dev" {
		t.Fatalf("lowercase stage = %q", got)
	}
	_ = os.Setenv("STAGE", "prod")
	if got := (&DefaultCenter{}).Stage(); got != "prod" {
		t.Fatalf("uppercase stage = %q", got)
	}

	previous := Default
	Default = &DefaultCenter{}
	t.Cleanup(func() { Default = previous })
	if got := Stage(); got != "prod" {
		t.Fatalf("global stage = %q", got)
	}
}

func restoreEnvironment(key, value string, existed bool) {
	if existed {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}
