package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthenticatePersonalAccessTokenUsesRawBearerDigest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&models.Role{}, &models.User{}, &models.UserAuthToken{}); err != nil {
		t.Fatalf("migrate PAT middleware schema: %v", err)
	}

	user := &models.User{}
	user.ID = "user-1"
	user.Username = "owner"
	if err = db.Session(&gorm.Session{SkipHooks: true}).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	rawToken := "raw-token-from-jwt-middleware-context"
	record := &models.UserAuthToken{
		UserID:      user.ID,
		TokenHash:   models.HashUserAuthToken(rawToken),
		Fingerprint: models.UserAuthTokenFingerprint(models.HashUserAuthToken(rawToken)),
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	record.ID = "pat-middleware-1"
	if err = db.Create(record).Error; err != nil {
		t.Fatalf("create PAT: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("JWT_TOKEN", rawToken)
	principal := &models.User{}
	verified := authenticatePersonalAccessToken(ctx, principal, record.ID)
	if verified == nil {
		t.Fatal("valid raw bearer digest was rejected")
	}
	if !principal.GetRefreshTokenDisable() || principal.GetPersonAccessToken() != record.ID {
		t.Fatal("verified identity was not marked as a PAT")
	}

	badCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	badCtx.Set("JWT_TOKEN", rawToken+"-wrong")
	if verified = authenticatePersonalAccessToken(badCtx, &models.User{}, record.ID); verified != nil {
		t.Fatal("wrong raw bearer was accepted for the signed PAT selector")
	}
}
