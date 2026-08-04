package compatibility

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminrouter "github.com/mss-boot-io/mss-boot-admin/admin/router"
	frameworkserver "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
)

func TestAdminModelsUseOwnedFrameworkDatabaseHandle(t *testing.T) {
	handle, err := (&gormdb.Database{
		Driver: "sqlite",
		Source: "file:admin-framework-compatibility?mode=memory&cache=shared",
	}).Open(context.Background())
	if err != nil {
		t.Fatalf("open framework database handle: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })

	if err := handle.DB.AutoMigrate(&models.Post{}); err != nil {
		t.Fatalf("migrate Admin model: %v", err)
	}
	post := &models.Post{Name: "Operator", Code: "operator", Sort: 10}
	post.ID = "operator"
	if err := handle.DB.Omit("Children").Create(post).Error; err != nil {
		t.Fatalf("create Admin model: %v", err)
	}
	var stored models.Post
	if err := handle.DB.First(&stored, "id = ?", "operator").Error; err != nil {
		t.Fatalf("read Admin model: %v", err)
	}
	if stored.Code != "operator" || stored.TableName() != "mss_boot_posts" {
		t.Fatalf("stored Admin model = %#v", stored)
	}
}

func TestAdminRouterUsesCurrentFrameworkControllerContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousMode := config.Cfg.Application.Mode
	config.Cfg.Application.Mode = config.ModeTest
	t.Cleanup(func() { config.Cfg.Application.Mode = previousMode })

	engine := gin.New()
	adminrouter.InitRouter(engine.Group("/admin"))
	if !containsRoute(engine.Routes(), http.MethodOptions, "/admin/api/*path") {
		t.Fatalf("Admin router did not register the framework OPTIONS contract: %#v", engine.Routes())
	}
	if len(engine.Routes()) < 2 {
		t.Fatalf("Admin controllers did not register through framework response contracts: %#v", engine.Routes())
	}
}

func TestFrameworkManagerOwnsAdminRunnableLifecycle(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	runnable := &adminRunnable{
		name: "admin-compatibility",
		run: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		},
	}
	manager := frameworkserver.New(
		frameworkserver.WithoutSignalHandling(),
		frameworkserver.WithGracefulShutdownTimeout(time.Second),
	)
	manager.Add(runnable)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- manager.Start(ctx) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Admin runnable did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("manager cancellation error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("framework manager did not stop Admin runnable")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("Admin runnable did not observe cancellation")
	}
}

type adminRunnable struct {
	name string
	run  func(context.Context) error
}

func (r *adminRunnable) String() string { return r.name }

func (r *adminRunnable) Start(ctx context.Context) error {
	if r.run == nil {
		return errors.New("Admin runnable is not configured")
	}
	return r.run(ctx)
}

func containsRoute(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
