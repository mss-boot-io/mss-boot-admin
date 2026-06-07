package service

import (
	"context"
	"errors"
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
	_ = s.cache.Set(ctx, sid, sessioncache.Entry{
		UserID:  in.UserID,
		RoleID:  in.RoleID,
		ExpUnix: row.ExpiredAt.Unix(),
	}, in.TTL)
	return sid, nil
}

func (s *SessionService) Lookup(ctx context.Context, db *gorm.DB, sid string) (LookupResult, error) {
	if sid == "" {
		return LookupResult{Status: LookupMissing}, nil
	}
	if entry, ok, _ := s.cache.Get(ctx, sid); ok {
		if entry.ExpUnix > 0 && time.Unix(entry.ExpUnix, 0).Before(time.Now()) {
			return LookupResult{Status: LookupExpired}, nil
		}
		return LookupResult{Status: LookupActive, Entry: entry}, nil
	}
	var row models.UserSession
	err := db.WithContext(ctx).Where("id = ?", sid).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LookupResult{Status: LookupMissing}, nil
	}
	if err != nil {
		return LookupResult{}, err
	}
	now := time.Now()
	switch {
	case row.Revoked:
		return LookupResult{Status: LookupRevoked}, nil
	case row.ExpiredAt.Before(now):
		return LookupResult{Status: LookupExpired}, nil
	}
	entry := sessioncache.Entry{UserID: row.UserID, RoleID: row.RoleID, ExpUnix: row.ExpiredAt.Unix()}
	_ = s.cache.Set(ctx, sid, entry, time.Until(row.ExpiredAt))
	return LookupResult{Status: LookupActive, Entry: entry}, nil
}

func (s *SessionService) Touch(ctx context.Context, db *gorm.DB, sid string) error {
	ok, err := s.cache.TryTouch(ctx, sid)
	if err != nil || !ok {
		return err
	}
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
		_ = s.cache.Del(ctx, sid)
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
	_ = s.cache.Del(ctx, sid)
	return &row, nil
}

func (s *SessionService) RevokeByUserID(ctx context.Context, db *gorm.DB, userID, actor string, reason models.SessionRevokeReason) (int64, error) {
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
	_ = s.cache.DelByUser(ctx, userID)
	return res.RowsAffected, nil
}
