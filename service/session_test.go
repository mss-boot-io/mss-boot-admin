package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg/sessioncache"
)

func setupSessionEnv(t *testing.T) (*SessionService, *gorm.DB, *miniredis.Miniredis) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.UserSession{}, &models.AuditLog{}))
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	t.Cleanup(mr.Close)
	cli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewSessionService(sessioncache.New(func() *redis.Client { return cli }))
	return svc, db, mr
}

func TestSessionCreate(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	ctx := context.Background()

	sid, err := svc.Create(ctx, db, CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1",
		IP: "1.1.1.1", UserAgent: "ua", TTL: time.Hour,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, sid)

	var got models.UserSession
	assert.NoError(t, db.First(&got, "id = ?", sid).Error)
	assert.Equal(t, "u1", got.UserID)
	assert.False(t, got.Revoked)

	entry, ok, err := svc.cache.Get(ctx, sid)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "u1", entry.UserID)
}

func TestSessionLookupCacheHit(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	ctx := context.Background()
	sid, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})

	res, err := svc.Lookup(ctx, db, sid)
	assert.NoError(t, err)
	assert.Equal(t, LookupActive, res.Status)
	assert.Equal(t, "u1", res.Entry.UserID)
}

func TestSessionLookupCacheMissDBHit(t *testing.T) {
	svc, db, mr := setupSessionEnv(t)
	ctx := context.Background()
	sid, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})

	mr.FlushAll()

	res, err := svc.Lookup(ctx, db, sid)
	assert.NoError(t, err)
	assert.Equal(t, LookupActive, res.Status)

	_, ok, _ := svc.cache.Get(ctx, sid)
	assert.True(t, ok, "应该已回填到 Redis")
}

func TestSessionLookupRevoked(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	ctx := context.Background()
	sid, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})
	_, err := svc.RevokeBySID(ctx, db, sid, "admin", models.SessionRevokeForceBySession)
	assert.NoError(t, err)

	res, _ := svc.Lookup(ctx, db, sid)
	assert.Equal(t, LookupRevoked, res.Status)
}

func TestSessionLookupExpired(t *testing.T) {
	svc, db, mr := setupSessionEnv(t)
	ctx := context.Background()
	sid, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})
	assert.NoError(t, db.Model(&models.UserSession{}).Where("id = ?", sid).
		Update("expired_at", time.Now().Add(-time.Minute)).Error)
	mr.FlushAll()

	res, _ := svc.Lookup(ctx, db, sid)
	assert.Equal(t, LookupExpired, res.Status)
}

func TestSessionLookupMissing(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	res, err := svc.Lookup(context.Background(), db, "nope")
	assert.NoError(t, err)
	assert.Equal(t, LookupMissing, res.Status)
}

func TestRevokeByUser(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	ctx := context.Background()
	sid1, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})
	sid2, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u1", Username: "a", RoleID: "r1", TTL: time.Hour})
	sid3, _ := svc.Create(ctx, db, CreateSessionInput{UserID: "u2", Username: "b", RoleID: "r1", TTL: time.Hour})

	n, err := svc.RevokeByUserID(ctx, db, "u1", "admin", models.SessionRevokeForceByUser)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)

	r1, _ := svc.Lookup(ctx, db, sid1)
	r2, _ := svc.Lookup(ctx, db, sid2)
	r3, _ := svc.Lookup(ctx, db, sid3)
	assert.Equal(t, LookupRevoked, r1.Status)
	assert.Equal(t, LookupRevoked, r2.Status)
	assert.Equal(t, LookupActive, r3.Status)
}

func TestCleanupOldSessions(t *testing.T) {
	svc, db, _ := setupSessionEnv(t)
	ctx := context.Background()

	old := time.Now().Add(-40 * 24 * time.Hour)
	r := &models.UserSession{UserID: "u1", LoginAt: old, LastSeenAt: old, ExpiredAt: old, Revoked: true}
	r.ID = "old-1"
	rt := old
	r.RevokedAt = &rt
	assert.NoError(t, db.Create(r).Error)

	_, _ = svc.Create(ctx, db, CreateSessionInput{UserID: "u2", Username: "b", RoleID: "r1", TTL: time.Hour})

	n, err := svc.CleanupOlderThan(ctx, db, 30*24*time.Hour)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)

	var count int64
	db.Model(&models.UserSession{}).Count(&count)
	assert.EqualValues(t, 1, count)
}
