package middleware

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/sessioncache"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

// setupAuthSessionTest wires the bits of middleware.Init that
// validateSessionFromClaims depends on without booting the whole admin server:
// a sqlite DB exposed via gormdb.DB (single-tenant center.GetDB reads it),
// a miniredis-backed Cache injected into SessionService. A temporary SQLite
// file is used instead of :memory: so the asynchronous last-seen writer and
// the assertion connection observe the same database when GORM opens more
// than one physical connection.
func setupAuthSessionTest(t *testing.T) (*gorm.DB, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	databasePath := filepath.Join(t.TempDir(), "auth-session.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.UserSession{}))

	prevDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = prevDB })

	mr, err := miniredis.Run()
	assert.NoError(t, err)
	t.Cleanup(mr.Close)

	cli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	prevCache := service.Session
	service.Session = service.NewSessionService(sessioncache.New(cli))
	t.Cleanup(func() { service.Session = prevCache })

	// Registered last so it runs before the service, Redis, DB, and temporary
	// directory cleanups registered above.
	t.Cleanup(waitForSessionTouches)

	return db, mr
}

func newTestGinCtx() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c
}

func authSessionPrincipal(userID, roleID string) *models.User {
	principal := &models.User{}
	principal.ID = userID
	principal.RoleID = roleID
	return principal
}

// TestValidateSessionFromClaims_MissingSid proves that an obsolete JWT without
// the mandatory V6 server-session identifier is rejected.
func TestValidateSessionFromClaims_MissingSid(t *testing.T) {
	setupAuthSessionTest(t)
	c := newTestGinCtx()

	ok := validateSessionFromClaims(c, jwt.MapClaims{"verifier": "{}"}, authSessionPrincipal("u1", "r1"))
	assert.False(t, ok, "missing sid claim must be rejected")
}

// TestValidateSessionFromClaims_ActiveSession covers the happy path: a sid
// pointing at an active row must let the request through.
func TestValidateSessionFromClaims_ActiveSession(t *testing.T) {
	db, _ := setupAuthSessionTest(t)
	sid, err := service.Session.Create(context.Background(), db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	var before models.UserSession
	assert.NoError(t, db.First(&before, "id = ?", sid).Error)

	c := newTestGinCtx()
	assert.True(t, validateSessionFromClaims(c, jwt.MapClaims{"sid": sid}, authSessionPrincipal("u1", "r1")),
		"active session must be accepted")
	waitForSessionTouch(t, db, sid, before.LastSeenAt)
}

func waitForSessionTouch(t *testing.T, db *gorm.DB, sid string, previous time.Time) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var current models.UserSession
		if err := db.First(&current, "id = ?", sid).Error; err != nil {
			t.Fatalf("read touched session: %v", err)
		}
		if current.LastSeenAt.After(previous) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("session %q was not touched before test cleanup", sid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestValidateSessionFromClaims_RevokedRejected is the integration regression
// for the security guarantee of this PR: a revoked session must not be able
// to keep authenticating, regardless of cache state.
func TestValidateSessionFromClaims_RevokedRejected(t *testing.T) {
	db, _ := setupAuthSessionTest(t)
	ctx := context.Background()
	sid, err := service.Session.Create(ctx, db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	_, err = service.Session.RevokeBySID(ctx, db, sid, "admin", models.SessionRevokeForceBySession)
	assert.NoError(t, err)

	c := newTestGinCtx()
	assert.False(t, validateSessionFromClaims(c, jwt.MapClaims{"sid": sid}, authSessionPrincipal("u1", "r1")),
		"revoked session must be rejected")
}

// TestValidateSessionFromClaims_RevokedInDBOnlyRejected protects the
// authoritative-DB path even when the cache has not been updated yet
// (mirrors the regression in service/session_test.go but at the middleware
// boundary).
func TestValidateSessionFromClaims_RevokedInDBOnlyRejected(t *testing.T) {
	db, _ := setupAuthSessionTest(t)
	ctx := context.Background()
	sid, err := service.Session.Create(ctx, db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	// Bypass RevokeBySID — only DB is updated, cache still has active entry.
	assert.NoError(t, db.Model(&models.UserSession{}).Where("id = ?", sid).
		Updates(map[string]any{"revoked": true, "revoked_at": time.Now(), "revoked_by": "ops"}).Error)

	c := newTestGinCtx()
	assert.False(t, validateSessionFromClaims(c, jwt.MapClaims{"sid": sid}, authSessionPrincipal("u1", "r1")),
		"DB-revoked session must be rejected even when cache says active")
}

// TestValidateSessionFromClaims_ExpiredRejected covers natural expiry.
func TestValidateSessionFromClaims_ExpiredRejected(t *testing.T) {
	db, mr := setupAuthSessionTest(t)
	ctx := context.Background()
	sid, err := service.Session.Create(ctx, db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	// Force expiry in DB and drop cache so Lookup goes straight to DB.
	assert.NoError(t, db.Model(&models.UserSession{}).Where("id = ?", sid).
		Update("expired_at", time.Now().Add(-time.Minute)).Error)
	mr.FlushAll()

	c := newTestGinCtx()
	assert.False(t, validateSessionFromClaims(c, jwt.MapClaims{"sid": sid}, authSessionPrincipal("u1", "r1")),
		"expired session must be rejected")
}

// TestValidateSessionFromClaims_MissingRowRejected covers the case where the
// sid is present in the JWT but the underlying row has been hard-deleted
// (e.g. by the cleanup cron after long-term retention).
func TestValidateSessionFromClaims_MissingRowRejected(t *testing.T) {
	setupAuthSessionTest(t)
	c := newTestGinCtx()
	assert.False(t, validateSessionFromClaims(c, jwt.MapClaims{"sid": "ghost"}, authSessionPrincipal("u1", "r1")),
		"unknown sid must be rejected")
}

func TestValidateSessionFromClaims_IdentityMismatchRejected(t *testing.T) {
	db, _ := setupAuthSessionTest(t)
	sid, err := service.Session.Create(context.Background(), db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	for _, principal := range []*models.User{
		authSessionPrincipal("other-user", "r1"),
		authSessionPrincipal("u1", "other-role"),
	} {
		assert.False(t, validateSessionFromClaims(newTestGinCtx(), jwt.MapClaims{"sid": sid}, principal))
	}
	assert.False(t, validateSessionFromClaims(newTestGinCtx(), jwt.MapClaims{"sid": sid}, nil))
}
