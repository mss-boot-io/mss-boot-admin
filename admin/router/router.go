package router

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/9 17:59:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/9 17:59:55
 */

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mss-boot-io/mss-boot-admin/admin/apis"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const oauthCredentialCORSHeader = "X-MSS-OAuth-Credential"

func InitRouter(r *gin.RouterGroup) {
	v1 := r.Group("/api")
	if config.Cfg.Application.Mode == config.ModeDev {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
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
	// Generator OAuth credentials are sent via an opaque, short-lived handle.
	// Always allow its header even when deployments replace the default CORS
	// header list with an explicit one.
	configCors.AddAllowHeaders(oauthCredentialCORSHeader)
	if len(config.Cfg.CORS.ExposeHeaders) > 0 {
		configCors.ExposeHeaders = append([]string(nil), config.Cfg.CORS.ExposeHeaders...)
	}
	if config.Cfg.CORS.MaxAge > 0 {
		configCors.MaxAge = config.Cfg.CORS.MaxAge
	}
	v1.Use(cors.New(configCors))
	v1.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := range response.Controllers {
		response.Controllers[i].Other(v1.Group("", response.Controllers[i].Handlers()...))
		e := v1.Group(response.Controllers[i].Path(), response.Controllers[i].Handlers()...)
		if action := response.Controllers[i].GetAction(response.Get); action != nil {
			e.GET("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Control); action != nil {
			e.POST("", action.Handler()...)
			e.PUT("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Create); action != nil {
			e.POST("", action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Update); action != nil {
			e.PUT("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Delete); action != nil {
			e.DELETE("/:"+response.Controllers[i].GetKey(), action.Handler()...)
		}
		if action := response.Controllers[i].GetAction(response.Search); action != nil {
			e.GET("", action.Handler()...)
		}
	}
}

func trustedCORSOrigins(configured []string) []string {
	trusted := make([]string, 0, len(configured))
	seen := make(map[string]struct{}, len(configured))
	for _, raw := range configured {
		origin := strings.TrimSpace(raw)
		parsed, err := url.Parse(origin)
		if err != nil || origin == "*" || parsed.User != nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			slog.Warn("ignore unsafe CORS origin", "origin", origin)
			continue
		}
		normalized := strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		trusted = append(trusted, normalized)
	}
	return trusted
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
