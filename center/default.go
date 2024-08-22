package center

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/grafana/pyroscope-go"
	"github.com/mss-boot-io/mss-boot-admin/storage"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/pkg/security"
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
	Manager: server.New(),
}

type DefaultCenter struct {
	NoticeImp
	TenantImp
	TenantMigrator
	UserImp
	VirtualModelImp
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
	VerifyCodeStoreImp
}

func (d *DefaultCenter) SetNotice(n NoticeImp) {
	d.NoticeImp = n
}

func (d *DefaultCenter) SetTenant(t TenantImp) {
	d.TenantImp = t
}

func (d *DefaultCenter) SetTenantMigrator(t TenantMigrator) {
	d.TenantMigrator = t
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

func (d *DefaultCenter) SetVirtualModel(v VirtualModelImp) {
	d.VirtualModelImp = v
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

func (d *DefaultCenter) SetVerifyCodeStore(v VerifyCodeStoreImp) {
	d.VerifyCodeStoreImp = v
}

func (d *DefaultCenter) GetNotice() NoticeImp {
	return d.NoticeImp
}

func (d *DefaultCenter) GetTenant() TenantImp {
	return d.TenantImp
}

func (d *DefaultCenter) GetTenantMigrator() TenantMigrator {
	return d.TenantMigrator
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

func (d *DefaultCenter) GetVirtualModel() VirtualModelImp {
	return d.VirtualModelImp
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

func (d *DefaultCenter) GetVerifyCodeStore() VerifyCodeStoreImp {
	return d.VerifyCodeStoreImp
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

func SetTenantMigrator(t TenantMigrator) *DefaultCenter {
	Default.SetTenantMigrator(t)
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

func SetVirtualModel(v VirtualModelImp) *DefaultCenter {
	Default.SetVirtualModel(v)
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

func SetVerifyCodeStore(v VerifyCodeStoreImp) *DefaultCenter {
	Default.SetVerifyCodeStore(v)
	return Default
}

func GetNotice() NoticeImp {
	return Default.GetNotice()
}

func GetTenant() TenantImp {
	return Default.GetTenant()
}

func GetTenantMigrator() TenantMigrator {
	return Default.GetTenantMigrator()
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

func GetVirtualModel() VirtualModelImp {
	return Default.GetVirtualModel()
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

func GetVerifyCodeStore() VerifyCodeStoreImp {
	return Default.GetVerifyCodeStore()
}
