package apis

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLanguageOtherRegistersPublicListRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	(&Language{}).Other(engine.Group("/admin/api"))

	for _, route := range engine.Routes() {
		if route.Method == "GET" && route.Path == "/admin/api/languages/public" {
			return
		}
	}

	t.Fatalf("expected public language route to be registered")
}
