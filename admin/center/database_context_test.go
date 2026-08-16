package center

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRequestDatabaseBindingFeedsSingleTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open request database: %v", err)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	if err := BindRequestDatabase(ctx, db); err != nil {
		t.Fatalf("bind request database: %v", err)
	}

	pinned, ok := RequestDatabase(ctx.Request.Context())
	if !ok || pinned != db {
		t.Fatal("request context did not preserve the leased database")
	}
	resolved := (&SingleTenant{}).GetDB(ctx, nil)
	if resolved == nil || resolved.Statement.ConnPool != db.Statement.ConnPool {
		t.Fatal("single tenant did not resolve the request-pinned connection pool")
	}
	if resolved.Statement.Context != ctx.Request.Context() {
		t.Fatal("single tenant database did not inherit the request context")
	}
}

func TestRequestDatabaseBindingRejectsIncompleteInputs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open request database: %v", err)
	}
	if err := BindRequestDatabase(nil, db); err == nil {
		t.Fatal("nil Gin context was accepted")
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if err := BindRequestDatabase(ctx, db); err == nil {
		t.Fatal("Gin context without an HTTP request was accepted")
	}
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	if err := BindRequestDatabase(ctx, nil); err == nil {
		t.Fatal("nil database was accepted")
	}
	if _, ok := RequestDatabase(context.Background()); ok {
		t.Fatal("database unexpectedly resolved from an unrelated context")
	}
}
