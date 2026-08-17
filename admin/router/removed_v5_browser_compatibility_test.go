package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRetiredBrowserAndMutationCompatibilityRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	InitRouter(engine.Group("/admin"))
	routes := engine.Routes()

	retired := []routeKey{
		{method: http.MethodPost, path: "/admin/api/user/login"},
		{method: http.MethodPost, path: "/admin/api/user/login/github"},
		{method: http.MethodPost, path: "/admin/api/user/refresh-token"},
		{method: http.MethodGet, path: "/admin/api/user/refresh-token"},
		{method: http.MethodPost, path: "/admin/api/user/oauth2/authorize"},
		{method: http.MethodPost, path: "/admin/api/user/binding"},
		{method: http.MethodDelete, path: "/admin/api/user/unbinding"},
		{method: http.MethodPost, path: "/admin/api/user/:provider/callback"},
		{method: http.MethodGet, path: "/admin/api/user/:provider/callback"},
		{method: http.MethodGet, path: "/admin/api/ws/connect-v6"},
		{method: http.MethodGet, path: "/admin/api/task/:operate/:id"},
		{method: http.MethodGet, path: "/admin/api/user-auth-token/generate"},
	}
	for _, route := range retired {
		if hasRoute(routes, route.method, route.path) {
			t.Errorf("retired compatibility route is registered: %s %s", route.method, route.path)
		}
	}

	for _, route := range []routeKey{
		{method: http.MethodPost, path: "/admin/api/user/session/login"},
		{method: http.MethodPost, path: "/admin/api/user/session/refresh-token"},
		{method: http.MethodPost, path: "/admin/api/user/session/oauth2/authorize"},
		{method: http.MethodPost, path: "/admin/api/user/session/:provider/callback"},
		{method: http.MethodPost, path: "/admin/api/ws/tickets"},
		{method: http.MethodGet, path: "/admin/api/ws/connect"},
	} {
		if !hasRoute(routes, route.method, route.path) {
			t.Errorf("canonical V6 route is missing: %s %s", route.method, route.path)
		}
	}
}
