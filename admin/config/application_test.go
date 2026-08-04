package config

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

func TestApplicationInitConfiguresModeAndStaticRoutes(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })

	application := &Application{
		StaticPath: map[string]string{
			"/assets":       "public/assets",
			"/favicon.ico": "public/favicon.ico",
		},
	}
	engine := gin.New()
	application.Init(engine)
	if application.Mode != ModeDev || gin.Mode() != gin.DebugMode {
		t.Fatalf("default mode = %q, gin mode = %q", application.Mode, gin.Mode())
	}
	for _, expected := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/assets/*filepath"},
		{http.MethodHead, "/assets/*filepath"},
		{http.MethodGet, "/favicon.ico"},
		{http.MethodGet, "/swagger.json"},
		{http.MethodGet, "/swagger.yaml"},
	} {
		if !applicationRouteExists(engine.Routes(), expected.method, expected.path) {
			t.Fatalf("route %s %s missing: %#v", expected.method, expected.path, engine.Routes())
		}
	}

	for _, test := range []struct {
		mode    Mode
		ginMode string
	}{
		{ModeTest, gin.TestMode},
		{ModeProd, gin.ReleaseMode},
	} {
		engine := gin.New()
		application := &Application{Mode: test.mode, StaticPath: map[string]string{"/assets": "public"}}
		application.Init(engine)
		if gin.Mode() != test.ginMode {
			t.Fatalf("mode %q selected gin mode %q", test.mode, gin.Mode())
		}
		if len(engine.Routes()) != 0 {
			t.Fatalf("mode %q registered static routes: %#v", test.mode, engine.Routes())
		}
	}
}

func TestUIServerInit(t *testing.T) {
	if runnable := (&UIServer{}).Init(); runnable != nil {
		t.Fatalf("disabled UI returned runnable %T", runnable)
	}

	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "index.html"),
		[]byte("<!doctype html><html><body>{{.}}</body></html>"),
		0o644,
	); err != nil {
		t.Fatalf("write UI index: %v", err)
	}
	ui := &UIServer{
		Enabled: true,
		Path:    root,
		Listen: frameworkconfig.Listen{
			Name:    "admin-ui",
			Addr:    "127.0.0.1:0",
			Timeout: 2,
		},
	}
	runnable := ui.Init()
	if runnable == nil {
		t.Fatal("enabled UI returned nil runnable")
	}
	if runnable.String() != "admin-ui" {
		t.Fatalf("runnable name = %q", runnable.String())
	}
}

func applicationRouteExists(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
