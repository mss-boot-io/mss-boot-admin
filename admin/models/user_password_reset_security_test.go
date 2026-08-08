package models

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPasswordResetAtomicallyRevokesSessionsAndPersonalAccessTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "password-reset-security.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open password reset security database: %v", err)
	}
	if err := db.AutoMigrate(&Role{}, &User{}, &UserSession{}, &UserAuthToken{}); err != nil {
		t.Fatalf("migrate password reset security schema: %v", err)
	}
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	role := &Role{Name: "member", Status: enum.Enabled}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	createUser := func(id string) *User {
		user := &User{UserLogin: UserLogin{
			RoleID:   role.ID,
			Username: id,
			Password: "old-password",
			Status:   enum.Enabled,
		}}
		user.ID = id
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %q: %v", id, err)
		}
		return user
	}
	user := createUser("reset-user")
	other := createUser("other-user")

	createSession := func(id, userID string) {
		row := &UserSession{
			UserID:     userID,
			RoleID:     role.ID,
			LoginAt:    time.Now(),
			LastSeenAt: time.Now(),
			ExpiredAt:  time.Now().Add(time.Hour),
		}
		row.ID = id
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create session %q: %v", id, err)
		}
	}
	createSession("reset-session", user.ID)
	createSession("other-session", other.ID)

	createPAT := func(id, userID, raw string) {
		digest := HashUserAuthToken(raw)
		row := &UserAuthToken{
			UserID:      userID,
			TokenHash:   digest,
			Fingerprint: UserAuthTokenFingerprint(digest),
			ExpiredAt:   time.Now().Add(time.Hour),
		}
		row.ID = id
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create PAT %q: %v", id, err)
		}
	}
	createPAT("reset-pat", user.ID, "reset-user-raw-token")
	createPAT("other-pat", other.ID, "other-user-raw-token")

	if err := PasswordReset(context.Background(), user.ID, "replacement-password"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	var resetSession UserSession
	if err := db.First(&resetSession, "id = ?", "reset-session").Error; err != nil {
		t.Fatalf("load reset session: %v", err)
	}
	if !resetSession.Revoked || resetSession.RevokedAt == nil ||
		resetSession.RevokeReason != SessionRevokeForceByUser {
		t.Fatalf("reset session revocation = %#v", resetSession)
	}
	var resetPAT UserAuthToken
	if err := db.First(&resetPAT, "id = ?", "reset-pat").Error; err != nil {
		t.Fatalf("load reset PAT: %v", err)
	}
	if !resetPAT.Revoked {
		t.Fatal("password reset left the old PAT active")
	}

	var otherSession UserSession
	if err := db.First(&otherSession, "id = ?", "other-session").Error; err != nil {
		t.Fatalf("load other session: %v", err)
	}
	var otherPAT UserAuthToken
	if err := db.First(&otherPAT, "id = ?", "other-pat").Error; err != nil {
		t.Fatalf("load other PAT: %v", err)
	}
	if otherSession.Revoked || otherPAT.Revoked {
		t.Fatal("password reset revoked another user's credentials")
	}
}
