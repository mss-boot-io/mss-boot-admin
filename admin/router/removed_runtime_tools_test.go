package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRuntimeDeveloperToolRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	InitRouter(engine.Group("/admin"))

	retired := map[routeKey]bool{
		{method: http.MethodGet, path: "/admin/api/:key"}:                  true,
		{method: http.MethodPost, path: "/admin/api/:key"}:                 true,
		{method: http.MethodGet, path: "/admin/api/:key/:id"}:              true,
		{method: http.MethodPut, path: "/admin/api/:key/:id"}:              true,
		{method: http.MethodDelete, path: "/admin/api/:key/:id"}:           true,
		{method: http.MethodGet, path: "/admin/api/documentation/:key"}:    true,
		{method: http.MethodGet, path: "/admin/api/models"}:                true,
		{method: http.MethodPost, path: "/admin/api/models"}:               true,
		{method: http.MethodGet, path: "/admin/api/models/:id"}:            true,
		{method: http.MethodPut, path: "/admin/api/models/:id"}:            true,
		{method: http.MethodDelete, path: "/admin/api/models/:id"}:         true,
		{method: http.MethodGet, path: "/admin/api/fields"}:                true,
		{method: http.MethodPost, path: "/admin/api/fields"}:               true,
		{method: http.MethodGet, path: "/admin/api/fields/:id"}:            true,
		{method: http.MethodPut, path: "/admin/api/fields/:id"}:            true,
		{method: http.MethodDelete, path: "/admin/api/fields/:id"}:         true,
		{method: http.MethodPut, path: "/admin/api/model/generate-data"}:   true,
		{method: http.MethodGet, path: "/admin/api/template/get-branches"}: true,
		{method: http.MethodGet, path: "/admin/api/template/get-path"}:     true,
		{method: http.MethodGet, path: "/admin/api/template/get-params"}:   true,
		{method: http.MethodPost, path: "/admin/api/template/generate"}:    true,
		{method: http.MethodGet, path: "/admin/api/github/get-login-url"}:  true,
	}
	routes := engine.Routes()
	for _, route := range routes {
		key := routeKey{method: route.Method, path: route.Path}
		if retired[key] {
			t.Errorf("retired runtime developer tool route is registered: %s %s", route.Method, route.Path)
		}
	}

	for _, expected := range []routeKey{
		{method: http.MethodGet, path: "/admin/api/monitor"},
		{method: http.MethodGet, path: "/admin/api/menus"},
		{method: http.MethodPost, path: "/admin/api/user/login"},
	} {
		if !hasRoute(routes, expected.method, expected.path) {
			t.Errorf("unrelated route missing after runtime tools removal: %s %s", expected.method, expected.path)
		}
	}
}

type routeKey struct {
	method string
	path   string
}
