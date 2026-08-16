package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RecentAuthenticationTTL       = 5 * time.Minute
	reauthenticationFailureWindow = 15 * time.Minute
	reauthenticationLockTTL       = 5 * time.Minute
	maxReauthenticationFailures   = 5
	minimumPasswordRunes          = 8
	maximumPasswordRunes          = 128
)

var (
	ErrSecuritySessionUnavailable   = errors.New("interactive security session is unavailable")
	ErrRecentAuthenticationRequired = errors.New("recent authentication is required")
	ErrReauthenticationLocked       = errors.New("reauthentication is temporarily locked")
	ErrInvalidCurrentPassword       = errors.New("current password is invalid")
	ErrLocalPasswordUnavailable     = errors.New("local password authentication is unavailable")
	ErrPasswordPolicy               = errors.New("new password does not meet policy")
	ErrPasswordUnchanged            = errors.New("new password must differ from the current password")
	ErrOAuthBindingNotFound         = errors.New("oauth binding was not found")
	ErrFinalLoginMethod             = errors.New("the final verified login method cannot be removed")
)

type RecentAuthenticationStatus struct {
	Recent      bool
	ExpiresAt   *time.Time
	LockedUntil *time.Time
}

// ValidateUserPassword applies the bounded password policy used by V6
// self-service. The password remains user-owned and is never normalized or
// logged; this function only checks length and character classes.
func ValidateUserPassword(password string) error {
	runes := utf8.RuneCountInString(password)
	if runes < minimumPasswordRunes || runes > maximumPasswordRunes {
		return ErrPasswordPolicy
	}
	var hasLetter, hasNumber bool
	for _, value := range password {
		hasLetter = hasLetter || unicode.IsLetter(value)
		hasNumber = hasNumber || unicode.IsNumber(value)
	}
	if !hasLetter || !hasNumber {
		return ErrPasswordPolicy
	}
	return nil
}

func (s *SessionService) RecentAuthenticationStatus(
	ctx context.Context,
	db *gorm.DB,
	sid, userID string,
) (RecentAuthenticationStatus, error) {
	if db == nil || strings.TrimSpace(sid) == "" || strings.TrimSpace(userID) == "" {
		return RecentAuthenticationStatus{}, ErrSecuritySessionUnavailable
	}
	var row models.UserSession
	if err := db.WithContext(ctx).
		Where("id = ? AND user_id = ?", sid, userID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RecentAuthenticationStatus{}, ErrSecuritySessionUnavailable
		}
		return RecentAuthenticationStatus{}, err
	}
	now := time.Now()
	if row.Revoked || !now.Before(row.ExpiredAt) {
		return RecentAuthenticationStatus{}, ErrSecuritySessionUnavailable
	}
	status := recentAuthenticationStatus(&row, now)
	return status, nil
}

func recentAuthenticationStatus(row *models.UserSession, now time.Time) RecentAuthenticationStatus {
	status := RecentAuthenticationStatus{}
	if row == nil {
		return status
	}
	if row.ReauthLockedUntil != nil && now.Before(*row.ReauthLockedUntil) {
		lockedUntil := row.ReauthLockedUntil.UTC()
		status.LockedUntil = &lockedUntil
	}
	if row.ReauthenticatedAt != nil {
		expiresAt := row.ReauthenticatedAt.Add(RecentAuthenticationTTL).UTC()
		status.ExpiresAt = &expiresAt
		status.Recent = now.Before(expiresAt)
	}
	return status
}

func loadLockedSecuritySession(
	ctx context.Context,
	tx *gorm.DB,
	sid, userID string,
	now time.Time,
) (*models.UserSession, error) {
	if tx == nil || strings.TrimSpace(sid) == "" || strings.TrimSpace(userID) == "" {
		return nil, ErrSecuritySessionUnavailable
	}
	row := &models.UserSession{}
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", sid, userID).
		First(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSecuritySessionUnavailable
	}
	if err != nil {
		return nil, err
	}
	if row.Revoked || !now.Before(row.ExpiredAt) {
		return nil, ErrSecuritySessionUnavailable
	}
	return row, nil
}

func requireRecentAuthentication(row *models.UserSession, now time.Time) error {
	if !recentAuthenticationStatus(row, now).Recent {
		return ErrRecentAuthenticationRequired
	}
	return nil
}

// ReauthenticateWithPassword verifies the current one-way password verifier
// and records proof only on the current durable server session. Failed proof
// state is committed even when authentication fails so brute-force attempts
// cannot be reset by rolling the request transaction back.
func (s *SessionService) ReauthenticateWithPassword(
	ctx context.Context,
	db *gorm.DB,
	sid, userID, password string,
) error {
	if db == nil || password == "" {
		return ErrInvalidCurrentPassword
	}
	var outcome error
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		session, err := loadLockedSecuritySession(ctx, tx, sid, userID, now)
		if err != nil {
			return err
		}
		if session.ReauthLockedUntil != nil && now.Before(*session.ReauthLockedUntil) {
			outcome = ErrReauthenticationLocked
			return nil
		}
		if session.ReauthFailedAt == nil || now.Sub(*session.ReauthFailedAt) > reauthenticationFailureWindow {
			session.ReauthFailedAttempts = 0
			session.ReauthLockedUntil = nil
		}

		user := &models.User{}
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "local_password_disabled", "password_hash", "salt").
			Where("id = ?", userID).
			First(user).Error; err != nil {
			return err
		}
		if user.LocalPasswordDisabled || user.PasswordHash == "" || user.Salt == "" {
			outcome = ErrLocalPasswordUnavailable
			return nil
		}
		verified, err := security.VerifyPassword(password, user.Salt, user.PasswordHash)
		if err != nil {
			return err
		}
		if !verified {
			session.ReauthFailedAttempts++
			session.ReauthFailedAt = &now
			if session.ReauthFailedAttempts >= maxReauthenticationFailures {
				lockedUntil := now.Add(reauthenticationLockTTL)
				session.ReauthLockedUntil = &lockedUntil
				outcome = ErrReauthenticationLocked
			} else {
				outcome = ErrInvalidCurrentPassword
			}
			return tx.Model(session).Updates(map[string]any{
				"reauth_failed_attempts": session.ReauthFailedAttempts,
				"reauth_failed_at":       session.ReauthFailedAt,
				"reauth_locked_until":    session.ReauthLockedUntil,
			}).Error
		}

		session.ReauthenticatedAt = &now
		session.ReauthFailedAttempts = 0
		session.ReauthFailedAt = nil
		session.ReauthLockedUntil = nil
		outcome = nil
		return tx.Model(session).Updates(map[string]any{
			"reauthenticated_at":     session.ReauthenticatedAt,
			"reauth_failed_attempts": 0,
			"reauth_failed_at":       nil,
			"reauth_locked_until":    nil,
		}).Error
	})
	if err != nil {
		return err
	}
	return outcome
}

// MarkRecentlyAuthenticated is used only after an OAuth callback has proved
// that the provider identity is already bound to this same user and session.
func (s *SessionService) MarkRecentlyAuthenticated(
	ctx context.Context,
	db *gorm.DB,
	sid, userID string,
) error {
	if db == nil {
		return ErrSecuritySessionUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		session, err := loadLockedSecuritySession(ctx, tx, sid, userID, now)
		if err != nil {
			return err
		}
		return tx.Model(session).Updates(map[string]any{
			"reauthenticated_at":     now,
			"reauth_failed_attempts": 0,
			"reauth_failed_at":       nil,
			"reauth_locked_until":    nil,
		}).Error
	})
}

// ChangePassword holds the current session and user through the recent-proof
// check and password/session/PAT rotation, so revocation cannot race the
// credential boundary after proof has been accepted.
func (s *SessionService) ChangePassword(
	ctx context.Context,
	db *gorm.DB,
	sid, userID, password string,
) error {
	if err := ValidateUserPassword(password); err != nil {
		return err
	}
	if db == nil {
		return ErrSecuritySessionUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		session, err := loadLockedSecuritySession(ctx, tx, sid, userID, now)
		if err != nil {
			return err
		}
		if err := requireRecentAuthentication(session, now); err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "local_password_disabled", "password_hash", "salt").
			Where("id = ?", userID).
			First(&user).Error; err != nil {
			return err
		}
		if !user.LocalPasswordDisabled && user.PasswordHash != "" && user.Salt != "" {
			same, verifyErr := security.VerifyPassword(password, user.Salt, user.PasswordHash)
			if verifyErr != nil {
				return verifyErr
			}
			if same {
				return ErrPasswordUnchanged
			}
		}
		return models.PasswordResetWithDB(ctx, tx, userID, password)
	})
}

// DisconnectOAuth removes one verified provider identity while holding both
// the user and binding rows. The current session must be recently proved and
// at least one other verified login method must remain after deletion.
func (s *SessionService) DisconnectOAuth(
	ctx context.Context,
	db *gorm.DB,
	sid, userID string,
	provider pkg.LoginProvider,
) error {
	if db == nil || (provider != pkg.GithubLoginProvider && provider != pkg.LarkLoginProvider) {
		return ErrOAuthBindingNotFound
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		session, err := loadLockedSecuritySession(ctx, tx, sid, userID, now)
		if err != nil {
			return err
		}
		if err := requireRecentAuthentication(session, now); err != nil {
			return err
		}

		user := &models.User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "local_password_disabled", "password_hash", "salt").
			Where("id = ?", userID).
			First(user).Error; err != nil {
			return err
		}

		var bindings []models.UserOAuth2
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Order("id").
			Find(&bindings).Error; err != nil {
			return err
		}
		target := -1
		for index := range bindings {
			if bindings[index].Provider == provider {
				target = index
				break
			}
		}
		if target < 0 {
			return ErrOAuthBindingNotFound
		}
		localPasswordAvailable := !user.LocalPasswordDisabled && user.PasswordHash != "" && user.Salt != ""
		if !localPasswordAvailable && len(bindings) == 1 {
			return ErrFinalLoginMethod
		}
		return tx.Delete(&bindings[target]).Error
	})
}
