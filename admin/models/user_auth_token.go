package models

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/30 09:45:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/30 09:45:03
 */

type UserAuthToken struct {
	ModelGormTenant
	UserID      string    `gorm:"type:varchar(64);index;comment:用户ID" json:"userID"`
	LegacyToken string    `gorm:"column:token;type:text;comment:legacy plaintext token" json:"-"`
	TokenHash   string    `gorm:"type:varchar(80);not null;default:'';comment:versioned token digest" json:"-"`
	Fingerprint string    `gorm:"type:varchar(16);not null;default:'';comment:safe token fingerprint" json:"fingerprint"`
	ExpiredAt   time.Time `gorm:"index;comment:过期时间" json:"expiredAt"`
	Revoked     bool      `gorm:"index;comment:是否撤销" json:"revoked"`
}

const (
	userAuthTokenHashPrefix        = "sha256:"
	userAuthTokenFingerprintLength = 12
)

var (
	ErrUserAuthTokenRevoked          = errors.New("user auth token revoked")
	ErrUserAuthTokenInvalidDigest    = errors.New("user auth token digest is invalid")
	ErrUserAuthTokenRotationConflict = errors.New("user auth token rotation conflict")
)

func (*UserAuthToken) TableName() string {
	return "mss_boot_user_auth_token"
}

// HashUserAuthToken returns the versioned digest persisted for a high-entropy
// personal access token. Raw token values must never be persisted or logged.
func HashUserAuthToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return userAuthTokenHashPrefix + hex.EncodeToString(digest[:])
}

// IsValidUserAuthTokenHash reports whether digest uses the supported versioned
// SHA-256 representation.
func IsValidUserAuthTokenHash(digest string) bool {
	if !strings.HasPrefix(digest, userAuthTokenHashPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(digest, userAuthTokenHashPrefix)
	if len(encoded) != sha256.Size*2 {
		return false
	}
	if encoded != strings.ToLower(encoded) {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

// UserAuthTokenFingerprint returns a short non-secret identifier derived from
// a validated digest. It is for display only and must not be used for lookup.
func UserAuthTokenFingerprint(digest string) string {
	if !IsValidUserAuthTokenHash(digest) {
		return ""
	}
	encoded := strings.TrimPrefix(digest, userAuthTokenHashPrefix)
	return encoded[:userAuthTokenFingerprintLength]
}

// VerifyUserAuthToken compares a presented raw token with its persisted digest
// in constant time. Malformed or unsupported digests fail closed.
func VerifyUserAuthToken(token, digest string) bool {
	if !IsValidUserAuthTokenHash(digest) {
		return false
	}
	expected := HashUserAuthToken(token)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(digest)) == 1
}

// GenerateUserAuthToken generates a PAT and persists only its digest. The raw
// token is returned separately so callers can display it exactly once.
func GenerateUserAuthToken(ctx *gin.Context, authMiddleware *jwt.GinJWTMiddleware, verify security.Verifier, validityPeriod time.Duration) (*UserAuthToken, string, error) {
	if validityPeriod <= 0 {
		validityPeriod = 100 * 12 * 30 * 24 * time.Hour
	}
	userAuthToken := &UserAuthToken{
		UserID: verify.GetUserID(),
	}
	userAuthToken.ID = pkg.SimpleID()
	token, expiredAt, err := generateUserAuthTokenValue(authMiddleware, verify, userAuthToken.ID, validityPeriod)
	if err != nil {
		return nil, "", err
	}
	userAuthToken.TokenHash = HashUserAuthToken(token)
	userAuthToken.Fingerprint = UserAuthTokenFingerprint(userAuthToken.TokenHash)
	userAuthToken.ExpiredAt = expiredAt

	db := silentUserAuthTokenDB(center.GetDB(ctx, &UserAuthToken{}))
	err = db.Create(userAuthToken).Error
	if err != nil {
		return nil, "", err
	}
	return userAuthToken, token, nil
}

// RotateUserAuthToken replaces an owned token digest with an owner-scoped
// compare-and-swap update. A secret is returned only after exactly one row was
// updated, so concurrent rotations cannot both produce usable responses.
func RotateUserAuthToken(ctx *gin.Context, authMiddleware *jwt.GinJWTMiddleware, verify security.Verifier, id string) (*UserAuthToken, string, error) {
	db := silentUserAuthTokenDB(center.GetDB(ctx, &UserAuthToken{}))
	userAuthToken := &UserAuthToken{}
	err := db.
		Select("id", "user_id", "token_hash", "fingerprint", "expired_at", "revoked", "created_at", "updated_at").
		Where("id = ?", id).
		Where("user_id = ?", verify.GetUserID()).
		First(userAuthToken).Error
	if err != nil {
		return nil, "", err
	}
	if userAuthToken.Revoked {
		return nil, "", ErrUserAuthTokenRevoked
	}
	if !IsValidUserAuthTokenHash(userAuthToken.TokenHash) {
		return nil, "", ErrUserAuthTokenInvalidDigest
	}

	validityPeriod := authMiddleware.Timeout
	if authMiddleware.TimeoutFunc != nil {
		validityPeriod = authMiddleware.TimeoutFunc(verify)
	}
	if validityPeriod <= 0 {
		return nil, "", errors.New("user auth token validity period is not positive")
	}
	token, expiredAt, err := generateUserAuthTokenValue(authMiddleware, verify, userAuthToken.ID, validityPeriod)
	if err != nil {
		return nil, "", err
	}
	tokenHash := HashUserAuthToken(token)
	fingerprint := UserAuthTokenFingerprint(tokenHash)
	updatedAt := time.Now()
	rotated, err := compareAndSwapUserAuthTokenDigest(
		db,
		userAuthToken.ID,
		verify.GetUserID(),
		userAuthToken.TokenHash,
		tokenHash,
		fingerprint,
		expiredAt,
		updatedAt,
	)
	if err != nil {
		return nil, "", err
	}
	if !rotated {
		return nil, "", ErrUserAuthTokenRotationConflict
	}

	userAuthToken.LegacyToken = ""
	userAuthToken.TokenHash = tokenHash
	userAuthToken.Fingerprint = fingerprint
	userAuthToken.ExpiredAt = expiredAt
	userAuthToken.UpdatedAt = updatedAt
	return userAuthToken, token, nil
}

func compareAndSwapUserAuthTokenDigest(db *gorm.DB, id, userID, oldHash, newHash, fingerprint string, expiredAt, updatedAt time.Time) (bool, error) {
	result := db.Model(&UserAuthToken{}).
		Where("id = ?", id).
		Where("user_id = ?", userID).
		Where("revoked = ?", false).
		Where("token_hash = ?", oldHash).
		Updates(map[string]any{
			"token":       "",
			"token_hash":  newHash,
			"fingerprint": fingerprint,
			"expired_at":  expiredAt,
			"updated_at":  updatedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func generateUserAuthTokenValue(authMiddleware *jwt.GinJWTMiddleware, verify security.Verifier, id string, validityPeriod time.Duration) (string, time.Time, error) {
	auth := *authMiddleware
	auth.Timeout = validityPeriod
	auth.TimeoutFunc = func(_ any) time.Duration {
		return validityPeriod
	}

	basePayloadFunc := auth.PayloadFunc
	tokenID := pkg.SimpleID()
	auth.PayloadFunc = func(data any) jwt.MapClaims {
		claims := jwt.MapClaims{}
		if basePayloadFunc != nil {
			claims = basePayloadFunc(data)
		}
		if claims == nil {
			claims = jwt.MapClaims{}
		}
		// These claims define PAT semantics and must not depend on the
		// application PayloadFunc. A custom or nil PayloadFunc must never
		// downgrade a PAT into an interactive refreshable token.
		claims["refreshTokenDisabled"] = true
		claims["personAccessToken"] = id
		claims["jti"] = tokenID
		return claims
	}

	previousRefreshDisabled := verify.GetRefreshTokenDisable()
	previousPersonAccessToken := verify.GetPersonAccessToken()
	verify.SetRefreshTokenDisable(true)
	verify.SetPersonAccessToken(id)
	defer func() {
		verify.SetRefreshTokenDisable(previousRefreshDisabled)
		verify.SetPersonAccessToken(previousPersonAccessToken)
	}()

	return auth.TokenGenerator(verify)
}

func silentUserAuthTokenDB(db *gorm.DB) *gorm.DB {
	if db == nil || db.Logger == nil {
		return db
	}
	return db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)})
}
