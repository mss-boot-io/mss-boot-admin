package center

import (
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/grafana/pyroscope-go"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/8 09:54:13
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/8 09:54:13
 */

var Default = &DefaultCenter{
	Manager:   server.New(),
	TenantImp: &SingleTenant{},
}

type DefaultCenter struct {
	NoticeImp
	TenantImp
	UserImp
	ConfigImp
	CustomConfigImp
	server.Manager
	gin.IRouter
	StageImp
	AppConfigImp
	UserConfigImp
	Profiler *pyroscope.Profiler
	StatisticsImp
	MakeRouterImp
	GRPCClientImp
	storage.AdapterCache
	storage.AdapterQueue
	storage.AdapterLocker
	challengeMu sync.RWMutex
	ChallengeImp
	runtimeChallengeMu sync.RWMutex
	runtimeChallenge   RuntimeChallengeImp
}

func (d *DefaultCenter) SetNotice(n NoticeImp) {
	d.NoticeImp = n
}

func (d *DefaultCenter) SetTenant(t TenantImp) {
	d.TenantImp = t
}

func (d *DefaultCenter) SetVerify(v UserImp) {
	d.UserImp = v
}

func (d *DefaultCenter) SetConfig(e ConfigImp) {
	d.ConfigImp = e
}

func (d *DefaultCenter) SetCustomConfig(e CustomConfigImp) {
	d.CustomConfigImp = e
}

func (d *DefaultCenter) SetServerManager(m server.Manager) {
	d.Manager = m
}

func (d *DefaultCenter) SetRouter(r gin.IRouter) {
	d.IRouter = r
}

func (d *DefaultCenter) SetAppConfig(a AppConfigImp) {
	d.AppConfigImp = a
}

func (d *DefaultCenter) SetUserConfig(u UserConfigImp) {
	d.UserConfigImp = u
}

func (d *DefaultCenter) SetProfiler(p *pyroscope.Profiler) {
	d.Profiler = p
}

func (d *DefaultCenter) SetStatistics(s StatisticsImp) {
	d.StatisticsImp = s
}

func (d *DefaultCenter) SetMakeRouter(m MakeRouterImp) {
	d.MakeRouterImp = m
}

func (d *DefaultCenter) SetGRPCClient(g GRPCClientImp) {
	d.GRPCClientImp = g
}

func (d *DefaultCenter) SetCache(c storage.AdapterCache) {
	d.AdapterCache = c
}

func (d *DefaultCenter) SetQueue(q storage.AdapterQueue) {
	d.AdapterQueue = q
}

func (d *DefaultCenter) SetLocker(l storage.AdapterLocker) {
	d.AdapterLocker = l
}

func (d *DefaultCenter) SetChallenge(v ChallengeImp) {
	d.challengeMu.Lock()
	defer d.challengeMu.Unlock()
	d.ChallengeImp = v
}

// SetRuntimeChallenge publishes a non-owning Runtime v2 capability. Config is
// the sole owner of the resource graph and clears this reference before close.
func (d *DefaultCenter) SetRuntimeChallenge(v RuntimeChallengeImp) {
	d.runtimeChallengeMu.Lock()
	defer d.runtimeChallengeMu.Unlock()
	d.runtimeChallenge = v
}

func (d *DefaultCenter) GetNotice() NoticeImp {
	return d.NoticeImp
}

func (d *DefaultCenter) GetTenant() TenantImp {
	return d.TenantImp
}

func (d *DefaultCenter) GetVerify() UserImp {
	return d.UserImp
}

func (d *DefaultCenter) GetConfig() ConfigImp {
	return d.ConfigImp
}

func (d *DefaultCenter) GetCustomConfig() CustomConfigImp {
	return d.CustomConfigImp
}

func (d *DefaultCenter) GetServerManager() server.Manager {
	return d.Manager
}

func (d *DefaultCenter) GetRouter() gin.IRouter {
	return d.IRouter
}

func (d *DefaultCenter) GetAppConfig() AppConfigImp {
	return d.AppConfigImp
}

func (d *DefaultCenter) GetUserConfig() UserConfigImp {
	return d.UserConfigImp
}

func (d *DefaultCenter) GetProfiler() *pyroscope.Profiler {
	return d.Profiler
}

func (d *DefaultCenter) GetStatistics() StatisticsImp {
	return d.StatisticsImp
}

func (d *DefaultCenter) GetMakeRouter() MakeRouterImp {
	return d.MakeRouterImp
}

func (d *DefaultCenter) GetGRPCClient() GRPCClientImp {
	return d.GRPCClientImp
}

func (d *DefaultCenter) GetCache() storage.AdapterCache {
	return d.AdapterCache
}

func (d *DefaultCenter) GetQueue() storage.AdapterQueue {
	return d.AdapterQueue
}

func (d *DefaultCenter) GetLocker() storage.AdapterLocker {
	return d.AdapterLocker
}

func (d *DefaultCenter) GetChallenge() ChallengeImp {
	d.challengeMu.RLock()
	defer d.challengeMu.RUnlock()
	return d.ChallengeImp
}

func (d *DefaultCenter) GetRuntimeChallenge() RuntimeChallengeImp {
	d.runtimeChallengeMu.RLock()
	defer d.runtimeChallengeMu.RUnlock()
	return d.runtimeChallenge
}

// EmailChallengeCapabilityEnabled is the authoritative runtime switch shared
// by challenge issuance and every challenge consumer. Missing, malformed, or
// false configuration fails closed.
func EmailChallengeCapabilityEnabled(ctx *gin.Context) bool {
	appConfig := GetAppConfig()
	if appConfig == nil {
		return false
	}
	value, exists := appConfig.GetAppConfig(ctx, "security:emailEnabled")
	if !exists {
		return false
	}
	return value == "true"
}

func (d *DefaultCenter) Stage() string {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = os.Getenv("stage")
	}
	if stage == "" {
		stage = "local"
	}
	return stage
}

func GetDB(ctx *gin.Context, table schema.Tabler) *gorm.DB {
	return Default.GetDB(ctx, table)
}

func SetNotice(n NoticeImp) *DefaultCenter {
	Default.SetNotice(n)
	return Default
}

func SetTenant(t TenantImp) *DefaultCenter {
	Default.SetTenant(t)
	return Default
}

func SetVerify(v security.Verifier) *DefaultCenter {
	Default.SetVerify(v)
	return Default
}

func SetConfig(e ConfigImp) *DefaultCenter {
	Default.SetConfig(e)
	return Default
}

func SetCustomConfig(e CustomConfigImp) *DefaultCenter {
	Default.SetCustomConfig(e)
	return Default
}

func SetServerManager(m server.Manager) *DefaultCenter {
	Default.SetServerManager(m)
	return Default
}

func SetAppConfig(a AppConfigImp) *DefaultCenter {
	Default.SetAppConfig(a)
	return Default
}

func SetUserConfig(u UserConfigImp) *DefaultCenter {
	Default.SetUserConfig(u)
	return Default
}

func SetRouter(r gin.IRouter) *DefaultCenter {
	Default.SetRouter(r)
	return Default
}

func SetProfiler(p *pyroscope.Profiler) *DefaultCenter {
	Default.SetProfiler(p)
	return Default
}

func SetStatistics(s StatisticsImp) *DefaultCenter {
	Default.SetStatistics(s)
	return Default
}

func SetMakeRouter(m MakeRouterImp) *DefaultCenter {
	Default.SetMakeRouter(m)
	return Default
}

func SetGRPCClient(g GRPCClientImp) *DefaultCenter {
	Default.SetGRPCClient(g)
	return Default
}

func SetCache(c storage.AdapterCache) *DefaultCenter {
	Default.SetCache(c)
	return Default
}

func SetQueue(q storage.AdapterQueue) *DefaultCenter {
	Default.SetQueue(q)
	return Default
}

func SetLocker(l storage.AdapterLocker) *DefaultCenter {
	Default.SetLocker(l)
	return Default
}

func SetChallenge(v ChallengeImp) *DefaultCenter {
	Default.SetChallenge(v)
	return Default
}

func SetRuntimeChallenge(v RuntimeChallengeImp) *DefaultCenter {
	Default.SetRuntimeChallenge(v)
	return Default
}

func GetNotice() NoticeImp {
	return Default.GetNotice()
}

func GetTenant() TenantImp {
	return Default.GetTenant()
}

func GetUser() UserImp {
	return Default.GetVerify()
}

func GetConfig() ConfigImp {
	return Default.GetConfig()
}

func GetCustomConfig() CustomConfigImp {
	return Default.GetCustomConfig()
}

func GetServerManager() server.Manager {
	return Default.GetServerManager()
}

func GetRouter() gin.IRouter {
	return Default.GetRouter()
}

func Stage() string {
	return Default.Stage()
}

func GetAppConfig() AppConfigImp {
	return Default.GetAppConfig()
}

func GetUserConfig() UserConfigImp {
	return Default.GetUserConfig()
}

func GetProfiler() *pyroscope.Profiler {
	return Default.GetProfiler()
}

func GetStatistics() StatisticsImp {
	return Default.GetStatistics()
}

func GetMakeRouter() MakeRouterImp {
	return Default.GetMakeRouter()
}

func GetGRPCClient() GRPCClientImp {
	return Default.GetGRPCClient()
}

func GetCache() storage.AdapterCache {
	return Default.GetCache()
}

func GetQueue() storage.AdapterQueue {
	return Default.GetQueue()
}

func GetLocker() storage.AdapterLocker {
	return Default.GetLocker()
}

func GetChallenge() ChallengeImp {
	return Default.GetChallenge()
}

func GetRuntimeChallenge() RuntimeChallengeImp {
	return Default.GetRuntimeChallenge()
}
