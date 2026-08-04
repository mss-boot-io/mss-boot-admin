package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
)

func TestInitRouterRegistersOperationalRoutesByMode(t *testing.T) {
	previousMode := config.Cfg.Application.Mode
	previousGinMode := gin.Mode()
	t.Cleanup(func() {
		config.Cfg.Application.Mode = previousMode
		gin.SetMode(previousGinMode)
	})

	tests := []struct {
		name        string
		mode        config.Mode
		wantSwagger bool
	}{
		{name: "development", mode: config.ModeDev, wantSwagger: true},
		{name: "production", mode: config.ModeProd, wantSwagger: false},
		{name: "test", mode: config.ModeTest, wantSwagger: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			config.Cfg.Application.Mode = test.mode
			engine := gin.New()
			InitRouter(engine.Group("/admin"))

			routes := engine.Routes()
			if !hasRoute(routes, http.MethodOptions, "/admin/api/*path") {
				t.Fatalf("OPTIONS route missing: %#v", routes)
			}
			if got := hasRoute(routes, http.MethodGet, "/admin/swagger/*any"); got != test.wantSwagger {
				t.Fatalf("swagger route present = %t, want %t", got, test.wantSwagger)
			}
			if len(routes) < 2 {
				t.Fatalf("expected controller routes, got %#v", routes)
			}
		})
	}
}

func TestMakeRouterRunsRegisteredFunctionsInOrder(t *testing.T) {
	engine := gin.New()
	group := engine.Group("/root")
	var calls []string
	makeRouter := &MakeRouter{}
	makeRouter.SetFunc(
		func(router *gin.RouterGroup) {
			calls = append(calls, "first")
			router.GET("/first", func(*gin.Context) {})
		},
		func(router *gin.RouterGroup) {
			calls = append(calls, "second")
			router.POST("/second", func(*gin.Context) {})
		},
	)
	if len(makeRouter.GetFunc()) != 2 {
		t.Fatalf("registered function count = %d", len(makeRouter.GetFunc()))
	}
	makeRouter.MakeRouter(group)
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("call order = %#v", calls)
	}
	if !hasRoute(engine.Routes(), http.MethodGet, "/root/first") ||
		!hasRoute(engine.Routes(), http.MethodPost, "/root/second") {
		t.Fatalf("registered routes = %#v", engine.Routes())
	}
}

func hasRoute(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
