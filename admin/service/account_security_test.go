package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPasswordReauthenticationIsSessionBoundAndRateLimited(t *testing.T) {
	db := openAccountSecurityTestDB(t)
	createAccountSecurityUser(t, db, "user-1", "current-password1", false)
	createAccountSecuritySession(t, db, "session-1", "user-1", nil)

	for attempt := 1; attempt <= maxReauthenticationFailures; attempt++ {
		err := Session.ReauthenticateWithPassword(
			t.Context(), db, "session-1", "user-1", "wrong-password1",
		)
		if attempt < maxReauthenticationFailures && !errors.Is(err, ErrInvalidCurrentPassword) {
			t.Fatalf("attempt %d error = %v, want invalid password", attempt, err)
		}
		if attempt == maxReauthenticationFailures && !errors.Is(err, ErrReauthenticationLocked) {
			t.Fatalf("attempt %d error = %v, want locked", attempt, err)
		}
	}
	if err := Session.ReauthenticateWithPassword(
		t.Context(), db, "session-1", "user-1", "current-password1",
	); !errors.Is(err, ErrReauthenticationLocked) {
		t.Fatalf("correct password during lock error = %v, want locked", err)
	}

	past := time.Now().Add(-time.Minute)
	if err := db.Model(&models.UserSession{}).Where("id = ?", "session-1").Updates(map[string]any{
		"reauth_locked_until":    past,
		"reauth_failed_at":       time.Now().Add(-reauthenticationFailureWindow - time.Minute),
		"reauth_failed_attempts": maxReauthenticationFailures,
	}).Error; err != nil {
		t.Fatalf("expire reauthentication lock: %v", err)
	}
	if err := Session.ReauthenticateWithPassword(
		t.Context(), db, "session-1", "user-1", "current-password1",
	); err != nil {
		t.Fatalf("reauthenticate after lock expiry: %v", err)
	}
	status, err := Session.RecentAuthenticationStatus(t.Context(), db, "session-1", "user-1")
	if err != nil || !status.Recent || status.ExpiresAt == nil || status.LockedUntil != nil {
		t.Fatalf("recent status = %#v, err = %v", status, err)
	}
	if _, err := Session.RecentAuthenticationStatus(t.Context(), db, "session-1", "other-user"); !errors.Is(err, ErrSecuritySessionUnavailable) {
		t.Fatalf("cross-user session status error = %v", err)
	}
}

func TestChangePasswordRequiresRecentProofAndRevokesEveryCredential(t *testing.T) {
	db := openAccountSecurityTestDB(t)
	createAccountSecurityUser(t, db, "user-1", "current-password1", false)
	createAccountSecuritySession(t, db, "session-1", "user-1", nil)
	token := &models.UserAuthToken{
		UserID:      "user-1",
		TokenHash:   models.HashUserAuthToken("personal-access-token"),
		Fingerprint: "fingerprint",
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	token.ID = "token-1"
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create PAT: %v", err)
	}

	err := Session.ChangePassword(t.Context(), db, "session-1", "user-1", "replacement-password2")
	if !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("change without recent proof error = %v", err)
	}
	if err := Session.MarkRecentlyAuthenticated(t.Context(), db, "session-1", "user-1"); err != nil {
		t.Fatalf("mark recent authentication: %v", err)
	}
	if err := Session.ChangePassword(t.Context(), db, "session-1", "user-1", "current-password1"); !errors.Is(err, ErrPasswordUnchanged) {
		t.Fatalf("unchanged password error = %v", err)
	}
	if err := Session.ChangePassword(t.Context(), db, "session-1", "user-1", "replacement-password2"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var user models.User
	if err := db.Where("id = ?", "user-1").First(&user).Error; err != nil {
		t.Fatalf("load changed user: %v", err)
	}
	verified, err := security.VerifyPassword("replacement-password2", user.Salt, user.PasswordHash)
	if err != nil || !verified || user.LocalPasswordDisabled {
		t.Fatalf("replacement password state invalid: verified=%v disabled=%v err=%v", verified, user.LocalPasswordDisabled, err)
	}
	var session models.UserSession
	if err := db.Where("id = ?", "session-1").First(&session).Error; err != nil {
		t.Fatalf("load revoked session: %v", err)
	}
	if !session.Revoked {
		t.Fatal("password change did not revoke the initiating session")
	}
	if err := db.Where("id = ?", "token-1").First(token).Error; err != nil {
		t.Fatalf("load revoked PAT: %v", err)
	}
	if !token.Revoked {
		t.Fatal("password change did not revoke the PAT")
	}
}

func TestOAuthDisconnectCannotRemoveFinalVerifiedLoginMethod(t *testing.T) {
	db := openAccountSecurityTestDB(t)
	createAccountSecurityUser(t, db, "oauth-user", "internal-password1", true)
	now := time.Now()
	createAccountSecuritySession(t, db, "oauth-session", "oauth-user", &now)
	createOAuthBinding(t, db, "github-binding", "oauth-user", pkg.GithubLoginProvider, "42", "")

	err := Session.DisconnectOAuth(
		t.Context(), db, "oauth-session", "oauth-user", pkg.GithubLoginProvider,
	)
	if !errors.Is(err, ErrFinalLoginMethod) {
		t.Fatalf("disconnect final provider error = %v", err)
	}
	createOAuthBinding(t, db, "lark-binding", "oauth-user", pkg.LarkLoginProvider, "", "union-1")
	if err := Session.DisconnectOAuth(
		t.Context(), db, "oauth-session", "oauth-user", pkg.GithubLoginProvider,
	); err != nil {
		t.Fatalf("disconnect provider with fallback: %v", err)
	}
	var active int64
	if err := db.Model(&models.UserOAuth2{}).Where("user_id = ?", "oauth-user").Count(&active).Error; err != nil {
		t.Fatalf("count active bindings: %v", err)
	}
	if active != 1 {
		t.Fatalf("active OAuth bindings = %d, want 1", active)
	}
}

func openAccountSecurityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "account-security.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open account security database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get account security database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserSession{},
		&models.UserOAuth2{},
		&models.UserAuthToken{},
	); err != nil {
		t.Fatalf("migrate account security schema: %v", err)
	}
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func createAccountSecurityUser(t *testing.T, db *gorm.DB, id, password string, disabled bool) {
	t.Helper()
	user := &models.User{UserLogin: models.UserLogin{
		Username:              id,
		Password:              password,
		LocalPasswordDisabled: disabled,
	}}
	user.ID = id
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user %s: %v", id, err)
	}
}

func createAccountSecuritySession(
	t *testing.T,
	db *gorm.DB,
	id, userID string,
	reauthenticatedAt *time.Time,
) {
	t.Helper()
	now := time.Now()
	row := &models.UserSession{
		UserID:            userID,
		LoginAt:           now,
		LastSeenAt:        now,
		ReauthenticatedAt: reauthenticatedAt,
		ExpiredAt:         now.Add(time.Hour),
	}
	row.ID = id
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create session %s: %v", id, err)
	}
}

func createOAuthBinding(
	t *testing.T,
	db *gorm.DB,
	id, userID string,
	provider pkg.LoginProvider,
	openID, unionID string,
) {
	t.Helper()
	binding := &models.UserOAuth2{
		UserID:   userID,
		Provider: provider,
		OpenID:   openID,
		UnionID:  unionID,
	}
	binding.ID = id
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("create OAuth binding %s: %v", id, err)
	}
}
