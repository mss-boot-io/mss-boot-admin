package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/pkg/sessioncache"
)

type LookupStatus string

const (
	LookupActive  LookupStatus = "active"
	LookupRevoked LookupStatus = "revoked"
	LookupExpired LookupStatus = "expired"
	LookupMissing LookupStatus = "missing"
)

type LookupResult struct {
	Status LookupStatus
	Entry  sessioncache.Entry
}

type CreateSessionInput struct {
	UserID    string
	Username  string
	RoleID    string
	IP        string
	UserAgent string
	TTL       time.Duration
}

type SessionService struct {
	cache *sessioncache.Cache
}

var Session = &SessionService{}

func NewSessionService(c *sessioncache.Cache) *SessionService {
	return &SessionService{cache: c}
}

func (s *SessionService) SetCache(c *sessioncache.Cache) { s.cache = c }

func (s *SessionService) Create(ctx context.Context, db *gorm.DB, in CreateSessionInput) (string, error) {
	if in.TTL <= 0 {
		return "", errors.New("session ttl must be positive")
	}
	now := time.Now()
	sid := pkg.SimpleID()
	row := &models.UserSession{
		UserID:     in.UserID,
		Username:   in.Username,
		RoleID:     in.RoleID,
		LoginAt:    now,
		LastSeenAt: now,
		ExpiredAt:  now.Add(in.TTL),
		IP:         in.IP,
		UserAgent:  in.UserAgent,
	}
	row.ID = sid
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		return "", err
	}
	if err := s.cache.Set(ctx, sid, sessioncache.Entry{
		UserID:  in.UserID,
		RoleID:  in.RoleID,
		ExpUnix: row.ExpiredAt.Unix(),
	}, in.TTL); err != nil {
		slog.Warn("session cache write failed", "sid", sid, "err", err)
	}
	return sid, nil
}

func (s *SessionService) Lookup(ctx context.Context, db *gorm.DB, sid string) (LookupResult, error) {
	if sid == "" {
		return LookupResult{Status: LookupMissing}, nil
	}
	entry, ok, err := s.cache.Get(ctx, sid)
	if err != nil {
		slog.Warn("session cache lookup failed", "sid", sid, "err", err)
	}
	if ok {
		// Fast path: cache already records the revoke (RevokeBySID/ByUserID
		// flips this synchronously on every revoke that succeeds against DB).
		if entry.Revoked {
			return LookupResult{Status: LookupRevoked}, nil
		}
		if entry.ExpUnix > 0 && time.Unix(entry.ExpUnix, 0).Before(time.Now()) {
			return LookupResult{Status: LookupExpired}, nil
		}
		// Authoritative check: cache hit alone is not enough — a Set/Del that
		// failed during a previous revoke could leave a stale "active" entry.
		// Re-read the minimal columns from DB and trust them. The lookup is by
		// primary key so the cost is one indexed point query per request.
		var probe struct {
			Revoked   bool
			ExpiredAt time.Time
		}
		err := db.WithContext(ctx).Model(&models.UserSession{}).
			Select("revoked", "expired_at").
			Where("id = ?", sid).
			First(&probe).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Cache out of sync with DB; treat as missing.
			if delErr := s.cache.Del(ctx, sid); delErr != nil {
				slog.Warn("session cache cleanup after missing DB row failed", "sid", sid, "err", delErr)
			}
			return LookupResult{Status: LookupMissing}, nil
		}
		if err != nil {
			return LookupResult{}, err
		}
		if probe.Revoked {
			ttl := time.Until(probe.ExpiredAt)
			if ttl <= 0 {
				ttl = time.Minute
			}
			if rErr := s.cache.SetRevoked(ctx, sid, entry, ttl); rErr != nil {
				slog.Warn("session cache repair after stale active hit failed", "sid", sid, "err", rErr)
			}
			return LookupResult{Status: LookupRevoked}, nil
		}
		if probe.ExpiredAt.Before(time.Now()) {
			return LookupResult{Status: LookupExpired}, nil
		}
		return LookupResult{Status: LookupActive, Entry: entry}, nil
	}
	var row models.UserSession
	if err = db.WithContext(ctx).Where("id = ?", sid).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return LookupResult{Status: LookupMissing}, nil
	} else if err != nil {
		return LookupResult{}, err
	}
	now := time.Now()
	switch {
	case row.Revoked:
		return LookupResult{Status: LookupRevoked}, nil
	case row.ExpiredAt.Before(now):
		return LookupResult{Status: LookupExpired}, nil
	}
	freshEntry := sessioncache.Entry{UserID: row.UserID, RoleID: row.RoleID, ExpUnix: row.ExpiredAt.Unix()}
	if err := s.cache.Set(ctx, sid, freshEntry, time.Until(row.ExpiredAt)); err != nil {
		slog.Warn("session cache backfill failed", "sid", sid, "err", err)
	}
	return LookupResult{Status: LookupActive, Entry: freshEntry}, nil
}

// MarkLastSeen asks the cache whether this caller wins the throttle slot for
// updating last_seen_at. Callers are expected to call RecordLastSeen only when
// the result is true (typically in a background goroutine to keep the request
// path cheap).
func (s *SessionService) MarkLastSeen(ctx context.Context, sid string) (bool, error) {
	return s.cache.TryTouch(ctx, sid)
}

// RecordLastSeen writes the current time into last_seen_at for an active session.
func (s *SessionService) RecordLastSeen(ctx context.Context, db *gorm.DB, sid string) error {
	return db.WithContext(ctx).Model(&models.UserSession{}).
		Where("id = ? AND revoked = ?", sid, false).
		Update("last_seen_at", time.Now()).Error
}

func (s *SessionService) RevokeBySID(ctx context.Context, db *gorm.DB, sid, actor string, reason models.SessionRevokeReason) (*models.UserSession, error) {
	var row models.UserSession
	if err := db.WithContext(ctx).Where("id = ?", sid).First(&row).Error; err != nil {
		return nil, err
	}
	if row.Revoked {
		s.markCacheRevoked(ctx, &row)
		return &row, nil
	}
	now := time.Now()
	row.Revoked = true
	row.RevokedAt = &now
	row.RevokedBy = actor
	row.RevokeReason = reason
	if err := db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	s.markCacheRevoked(ctx, &row)
	return &row, nil
}

func (s *SessionService) RevokeByUserID(ctx context.Context, db *gorm.DB, userID, actor string, reason models.SessionRevokeReason) (int64, error) {
	var rows []models.UserSession
	if err := db.WithContext(ctx).
		Where("user_id = ? AND revoked = ?", userID, false).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	now := time.Now()
	res := db.WithContext(ctx).Model(&models.UserSession{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Updates(map[string]any{
			"revoked":       true,
			"revoked_at":    now,
			"revoked_by":    actor,
			"revoke_reason": reason,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	for i := range rows {
		rows[i].Revoked = true
		rows[i].RevokedAt = &now
		s.markCacheRevoked(ctx, &rows[i])
	}
	return res.RowsAffected, nil
}

// markCacheRevoked best-effort writes a revoked sentinel into the cache so
// Lookup's fast path returns LookupRevoked without DB. Failures are logged;
// the authoritative check in Lookup (DB SELECT) still catches the revoke.
func (s *SessionService) markCacheRevoked(ctx context.Context, row *models.UserSession) {
	ttl := time.Until(row.ExpiredAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	if err := s.cache.SetRevoked(ctx, row.ID, sessioncache.Entry{
		UserID:  row.UserID,
		RoleID:  row.RoleID,
		ExpUnix: row.ExpiredAt.Unix(),
	}, ttl); err != nil {
		slog.Warn("session cache mark revoked failed", "sid", row.ID, "err", err)
	}
}

func (s *SessionService) CleanupOlderThan(ctx context.Context, db *gorm.DB, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	res := db.WithContext(ctx).
		Where("(revoked = ? AND revoked_at < ?) OR (revoked = ? AND expired_at < ?)",
			true, cutoff, false, cutoff).
		Delete(&models.UserSession{})
	return res.RowsAffected, res.Error
}
