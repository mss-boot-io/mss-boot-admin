package router

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/9 17:59:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/9 17:59:55
 */

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/apis"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RouteGroups exposes the route groups that the Admin composition root may
// extend. ProtectedAPI always carries the canonical CORS and browser CSRF
// policy; business modules receive only a child of this group.
type RouteGroups struct {
	ProtectedAPI *gin.RouterGroup
}

// Dependencies are application-owned core controllers that cannot be safely
// discovered through package initialization.
type Dependencies struct {
	PresentationProfiles response.Controller
}

// InitRouter preserves the historical router entrypoint.
func InitRouter(r *gin.RouterGroup) {
	InitRouteGroups(r)
}

// InitRouteGroups mounts the complete core Admin routes and returns the single
// protected API group for explicit business-module composition.
func InitRouteGroups(r *gin.RouterGroup) RouteGroups {
	registry := presentation.MustNewFrozenRegistry()
	policy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionDisabled, nil, false, registry,
	)
	profileService, err := service.NewPresentationProfileService(registry, policy)
	if err != nil {
		panic(err)
	}
	presentationController, err := apis.NewPresentationProfileController(profileService)
	if err != nil {
		panic(err)
	}
	groups, err := InitRouteGroupsWithDependencies(r, Dependencies{
		PresentationProfiles: presentationController,
	})
	if err != nil {
		panic(err)
	}
	return groups
}

// InitRouteGroupsWithDependencies is the production route composition path.
func InitRouteGroupsWithDependencies(r *gin.RouterGroup, dependencies Dependencies) (RouteGroups, error) {
	if r == nil {
		return RouteGroups{}, errors.New("admin route group is required")
	}
	if dependencies.PresentationProfiles == nil {
		return RouteGroups{}, errors.New("presentation profile controller is required")
	}
	if config.Cfg.Application.Mode == config.ModeDev {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	v1 := newProtectedAPI(r)
	mountCoreRoutes(v1, dependencies.PresentationProfiles)
	return RouteGroups{ProtectedAPI: v1}, nil
}

func newProtectedAPI(r *gin.RouterGroup) *gin.RouterGroup {
	v1 := r.Group("/api")
	configCors := cors.DefaultConfig()
	configCors.AllowOrigins = trustedCORSOrigins(config.Cfg.CORS.AllowOrigins)
	if len(configCors.AllowOrigins) == 0 {
		// gin-contrib/cors rejects a config with no origin matcher. A matcher
		// that always returns false keeps same-origin requests working while
		// cross-origin access fails closed until an explicit origin is set.
		configCors.AllowOriginFunc = func(string) bool { return false }
	}
	configCors.AllowCredentials = true
	if len(config.Cfg.CORS.AllowMethods) > 0 {
		configCors.AllowMethods = append([]string(nil), config.Cfg.CORS.AllowMethods...)
	}
	if len(config.Cfg.CORS.AllowHeaders) > 0 {
		configCors.AllowHeaders = append([]string(nil), config.Cfg.CORS.AllowHeaders...)
	} else {
		configCors.AddAllowHeaders("Authorization")
	}
	if len(config.Cfg.CORS.ExposeHeaders) > 0 {
		configCors.ExposeHeaders = append([]string(nil), config.Cfg.CORS.ExposeHeaders...)
	}
	if config.Cfg.CORS.MaxAge > 0 {
		configCors.MaxAge = config.Cfg.CORS.MaxAge
	}
	v1.Use(cors.New(configCors))
	v1.Use(middleware.EnforceBrowserCSRF())
	v1.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return v1
}

func mountCoreRoutes(v1 *gin.RouterGroup, explicit ...response.Controller) {
	for i := range response.Controllers {
		mountController(v1, response.Controllers[i])
	}
	for _, current := range explicit {
		mountController(v1, current)
	}
}

func mountController(v1 *gin.RouterGroup, current response.Controller) {
	current.Other(v1.Group("", current.Handlers()...))
	e := v1.Group(current.Path(), current.Handlers()...)
	if action := current.GetAction(response.Get); action != nil {
		e.GET("/:"+current.GetKey(), action.Handler()...)
	}
	if action := current.GetAction(response.Control); action != nil {
		e.POST("", action.Handler()...)
		e.PUT("/:"+current.GetKey(), action.Handler()...)
	}
	if action := current.GetAction(response.Create); action != nil {
		e.POST("", action.Handler()...)
	}
	if action := current.GetAction(response.Update); action != nil {
		e.PUT("/:"+current.GetKey(), action.Handler()...)
	}
	if action := current.GetAction(response.Delete); action != nil {
		e.DELETE("/:"+current.GetKey(), action.Handler()...)
	}
	if action := current.GetAction(response.Search); action != nil {
		e.GET("", action.Handler()...)
	}
}

func trustedCORSOrigins(configured []string) []string {
	return browsersecurity.TrustedOrigins(configured)
}

var DefaultMakeRouter = &MakeRouter{
	funcs: []func(*gin.RouterGroup){InitRouter},
}

type MakeRouter struct {
	funcs []func(*gin.RouterGroup)
}

func (m *MakeRouter) SetFunc(f ...func(*gin.RouterGroup)) {
	if m.funcs == nil {
		m.funcs = make([]func(*gin.RouterGroup), 0)
	}
	m.funcs = append(m.funcs, f...)
}

func (m *MakeRouter) GetFunc() []func(*gin.RouterGroup) {
	return m.funcs
}

func (m *MakeRouter) MakeRouter(r *gin.RouterGroup) {
	for i := range m.funcs {
		m.funcs[i](r)
	}
}
