package middleware

import (
	"path/filepath"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/sessioncache"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthenticateVerifierReloadsActiveDatabasePrincipal(t *testing.T) {
	db, role, user := setupAuthPrincipalTest(t)

	login := func() error {
		principal, err := AuthenticateVerifier(newTestGinCtx(), &models.User{UserLogin: models.UserLogin{
			Username: user.Username,
			Password: "correct-password",
		}})
		loginContext.Clear()
		if err == nil {
			current, ok := principal.(*models.User)
			if !ok {
				t.Fatalf("principal type = %T, want *models.User", principal)
			}
			if current.ID != user.ID || current.RoleID != role.ID || current.Role == nil {
				t.Fatalf("current principal = %#v", current)
			}
			if current.PasswordHash != "" || current.Salt != "" || current.Password != "" || current.Email != "" {
				t.Fatalf("authorization principal retained credentials/profile data: %#v", current.UserLogin)
			}
		}
		return err
	}

	if err := login(); err != nil {
		t.Fatalf("active login rejected: %v", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("status", enum.Disabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	if err := login(); err == nil {
		t.Fatal("disabled user logged in")
	}
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("status", enum.Enabled).Error; err != nil {
		t.Fatalf("restore user: %v", err)
	}
	if err := db.Model(&models.Role{}).Where("id = ?", role.ID).
		Update("status", enum.Disabled).Error; err != nil {
		t.Fatalf("disable role: %v", err)
	}
	if err := login(); err == nil {
		t.Fatal("user with a disabled role logged in")
	}
}

func TestCurrentPrincipalFromClaimsRejectsRoleDriftAndIgnoresJWTAuthority(t *testing.T) {
	db, role, user := setupAuthPrincipalTest(t)
	ctx := newTestGinCtx()

	principal, err := currentPrincipalFromClaims(ctx, jwt.MapClaims{"uid": user.ID, "rid": role.ID})
	if err != nil {
		t.Fatalf("load current claims principal: %v", err)
	}
	if principal.Root() {
		t.Fatal("non-root database role became root")
	}

	if _, err := currentPrincipalFromClaims(ctx, jwt.MapClaims{"verifier": `{"id":"legacy"}`}); err == nil {
		t.Fatal("retired verifier claim was accepted")
	}

	newRole := &models.Role{Name: "new-role", Status: enum.Enabled}
	if err := db.Create(newRole).Error; err != nil {
		t.Fatalf("create changed role: %v", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("role_id", newRole.ID).Error; err != nil {
		t.Fatalf("change user role: %v", err)
	}
	if _, err := currentPrincipalFromClaims(ctx, jwt.MapClaims{"uid": user.ID, "rid": role.ID}); err == nil {
		t.Fatal("JWT issued for an old role remained valid")
	}
	if _, err := currentPrincipalFromClaims(ctx, jwt.MapClaims{"uid": user.ID, "rid": newRole.ID}); err != nil {
		t.Fatalf("current role snapshot rejected: %v", err)
	}
	if _, err := currentPrincipalFromClaims(ctx, jwt.MapClaims{"uid": user.ID}); err == nil {
		t.Fatal("incomplete identity claims were accepted")
	}
}

func setupAuthPrincipalTest(t *testing.T) (*gorm.DB, *models.Role, *models.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth-principal.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open auth principal database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.UserOAuth2{},
		&models.AuditLog{},
		&models.UserSession{},
	); err != nil {
		t.Fatalf("migrate auth principal schema: %v", err)
	}

	previousDB := gormdb.DB
	previousVerifier := Verifier
	previousTimeout := config.Cfg.Auth.Timeout
	previousSessions := service.Session
	gormdb.DB = db
	Verifier = &models.User{}
	config.Cfg.Auth.Timeout = time.Hour
	service.Session = service.NewSessionService(sessioncache.New(nil))
	t.Cleanup(func() {
		gormdb.DB = previousDB
		Verifier = previousVerifier
		config.Cfg.Auth.Timeout = previousTimeout
		service.Session = previousSessions
	})

	role := &models.Role{Name: "member", Status: enum.Enabled}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create auth role: %v", err)
	}
	user := &models.User{UserLogin: models.UserLogin{
		Username: "principal-user",
		Password: "correct-password",
		RoleID:   role.ID,
		Status:   enum.Enabled,
	}}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create auth user: %v", err)
	}
	return db, role, user
}
