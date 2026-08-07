package models

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	standardjwt "github.com/golang-jwt/jwt/v4"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserAuthTokenDigestContract(t *testing.T) {
	raw := "signed-high-entropy-token"
	digest := HashUserAuthToken(raw)
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
		t.Fatalf("digest = %q, want versioned SHA-256", digest)
	}
	if !IsValidUserAuthTokenHash(digest) {
		t.Fatal("generated digest should be valid")
	}
	if !VerifyUserAuthToken(raw, digest) {
		t.Fatal("raw token should match its digest")
	}
	if VerifyUserAuthToken(raw+"-other", digest) {
		t.Fatal("different token must not match digest")
	}
	if VerifyUserAuthToken(raw, "sha256:not-hex") {
		t.Fatal("malformed digest must fail closed")
	}
	uppercaseDigest := userAuthTokenHashPrefix + strings.ToUpper(strings.TrimPrefix(digest, userAuthTokenHashPrefix))
	if IsValidUserAuthTokenHash(uppercaseDigest) {
		t.Fatal("non-canonical uppercase digest was accepted")
	}
	if VerifyUserAuthToken(raw, uppercaseDigest) {
		t.Fatal("non-canonical uppercase digest authenticated")
	}
	if got := UserAuthTokenFingerprint(digest); len(got) != userAuthTokenFingerprintLength {
		t.Fatalf("fingerprint length = %d, want %d", len(got), userAuthTokenFingerprintLength)
	}
}

func TestGenerateUserAuthTokenPersistsDigestAndRestoresVerifier(t *testing.T) {
	db := prepareUserAuthTokenModelTestDB(t)
	auth := newUserAuthTokenTestMiddleware(t)
	verifier := &User{}
	verifier.ID = "user-1"
	ctx, _ := gin.CreateTestContext(nil)

	record, raw, err := GenerateUserAuthToken(ctx, auth, verifier, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if raw == "" {
		t.Fatal("create response secret is empty")
	}
	if record.LegacyToken != "" {
		t.Fatal("generated model retained plaintext")
	}
	if !VerifyUserAuthToken(raw, record.TokenHash) {
		t.Fatal("generated secret does not match persisted digest")
	}
	if record.Fingerprint != UserAuthTokenFingerprint(record.TokenHash) {
		t.Fatal("generated fingerprint does not match digest")
	}
	if verifier.GetRefreshTokenDisable() || verifier.GetPersonAccessToken() != "" {
		t.Fatal("generation mutated the interactive verifier")
	}

	stored := &UserAuthToken{}
	if err = db.First(stored, "id = ?", record.ID).Error; err != nil {
		t.Fatalf("load stored token: %v", err)
	}
	if stored.LegacyToken != "" || stored.TokenHash != record.TokenHash {
		t.Fatalf("stored secret contract mismatch: legacy=%q hashMatches=%v", stored.LegacyToken, stored.TokenHash == record.TokenHash)
	}

	parsed, err := auth.ParseTokenString(raw)
	if err != nil {
		t.Fatalf("parse generated token: %v", err)
	}
	claims, ok := parsed.Claims.(standardjwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T", parsed.Claims)
	}
	if claims["refreshTokenDisabled"] != true {
		t.Fatalf("refreshTokenDisabled = %#v, want true", claims["refreshTokenDisabled"])
	}
	if claims["personAccessToken"] != record.ID {
		t.Fatalf("personAccessToken = %#v, want %q", claims["personAccessToken"], record.ID)
	}
	if claims["jti"] == "" {
		t.Fatal("generated PAT has no uniqueness claim")
	}
}

func TestGenerateUserAuthTokenForcesSecurityClaims(t *testing.T) {
	testCases := []struct {
		name        string
		payloadFunc func(any) jwt.MapClaims
	}{
		{name: "nil payload"},
		{
			name: "conflicting custom payload",
			payloadFunc: func(any) jwt.MapClaims {
				return jwt.MapClaims{
					"refreshTokenDisabled": false,
					"personAccessToken":    "wrong-selector",
					"jti":                  "reused-id",
					"custom":               "preserved",
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			prepareUserAuthTokenModelTestDB(t)
			auth := newUserAuthTokenTestMiddleware(t)
			auth.PayloadFunc = testCase.payloadFunc
			verifier := &User{}
			verifier.ID = "user-claims"
			ctx, _ := gin.CreateTestContext(nil)

			record, raw, err := GenerateUserAuthToken(ctx, auth, verifier, time.Hour)
			if err != nil {
				t.Fatalf("generate PAT: %v", err)
			}
			parsed, err := auth.ParseTokenString(raw)
			if err != nil {
				t.Fatalf("parse PAT: %v", err)
			}
			claims, ok := parsed.Claims.(standardjwt.MapClaims)
			if !ok {
				t.Fatalf("claims type = %T", parsed.Claims)
			}
			if claims["refreshTokenDisabled"] != true {
				t.Fatalf("refreshTokenDisabled = %#v, want true", claims["refreshTokenDisabled"])
			}
			if claims["personAccessToken"] != record.ID {
				t.Fatalf("personAccessToken = %#v, want %q", claims["personAccessToken"], record.ID)
			}
			if claims["jti"] == "" || claims["jti"] == "reused-id" {
				t.Fatalf("jti = %#v, want a fresh forced value", claims["jti"])
			}
			if testCase.payloadFunc != nil && claims["custom"] != "preserved" {
				t.Fatalf("custom claim = %#v, want preserved", claims["custom"])
			}
			if verifier.GetRefreshTokenDisable() || verifier.GetPersonAccessToken() != "" {
				t.Fatal("generation did not restore verifier state")
			}
		})
	}
}

func TestUserCheckTokenBindsRawBearerDigest(t *testing.T) {
	db := prepareUserAuthTokenModelTestDB(t)
	user := &User{}
	user.ID = "user-1"
	user.Username = "owner"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	raw := "raw-bearer-token"
	record := &UserAuthToken{
		UserID:      user.ID,
		TokenHash:   HashUserAuthToken(raw),
		Fingerprint: UserAuthTokenFingerprint(HashUserAuthToken(raw)),
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	record.ID = "pat-record-1"
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	principal := func() *User {
		value := &User{}
		value.PersonAccessToken = record.ID
		return value
	}
	if err := principal().CheckToken(context.Background(), raw); err != nil {
		t.Fatalf("valid bearer rejected: %v", err)
	}
	if err := principal().CheckToken(context.Background(), raw+"-wrong"); err == nil {
		t.Fatal("wrong bearer accepted for a valid signed selector")
	}
	if err := principal().CheckToken(context.Background(), record.TokenHash); err == nil {
		t.Fatal("persisted digest was accepted as a bearer token")
	}
	if err := db.Model(record).Update("expired_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire token: %v", err)
	}
	if err := principal().CheckToken(context.Background(), raw); err == nil {
		t.Fatal("expired token accepted")
	}
	if err := db.Model(record).Update("expired_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("restore token expiry: %v", err)
	}

	if err := db.Model(record).Update("token_hash", "sha256:not-hex").Error; err != nil {
		t.Fatalf("corrupt digest: %v", err)
	}
	if err := principal().CheckToken(context.Background(), raw); err == nil {
		t.Fatal("malformed digest did not fail closed")
	}

	if err := db.Model(record).Updates(map[string]any{
		"token_hash": HashUserAuthToken(raw),
		"revoked":    true,
	}).Error; err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if err := principal().CheckToken(context.Background(), raw); err == nil {
		t.Fatal("revoked token accepted")
	}
}

func TestUserAuthTokenRotationCASAllowsOnlyOneSnapshot(t *testing.T) {
	db := prepareUserAuthTokenModelTestDB(t)
	oldRaw := "old-rotation-token"
	oldHash := HashUserAuthToken(oldRaw)
	record := &UserAuthToken{
		UserID:      "user-1",
		TokenHash:   oldHash,
		Fingerprint: UserAuthTokenFingerprint(oldHash),
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	record.ID = "pat-cas-1"
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("create CAS token: %v", err)
	}

	firstHash := HashUserAuthToken("first-rotated-token")
	firstWon, err := compareAndSwapUserAuthTokenDigest(
		db,
		record.ID,
		record.UserID,
		oldHash,
		firstHash,
		UserAuthTokenFingerprint(firstHash),
		time.Now().Add(2*time.Hour),
		time.Now(),
	)
	if err != nil || !firstWon {
		t.Fatalf("first rotation result = (%v, %v), want success", firstWon, err)
	}

	secondHash := HashUserAuthToken("second-rotated-token")
	secondWon, err := compareAndSwapUserAuthTokenDigest(
		db,
		record.ID,
		record.UserID,
		oldHash,
		secondHash,
		UserAuthTokenFingerprint(secondHash),
		time.Now().Add(2*time.Hour),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("second rotation CAS: %v", err)
	}
	if secondWon {
		t.Fatal("two rotations from the same digest snapshot both succeeded")
	}

	stored := &UserAuthToken{}
	if err = db.First(stored, "id = ?", record.ID).Error; err != nil {
		t.Fatalf("load CAS token: %v", err)
	}
	if stored.TokenHash != firstHash {
		t.Fatal("losing rotation replaced the winning digest")
	}

	if err = db.Model(&UserAuthToken{}).
		Where("id = ? AND revoked = ?", record.ID, false).
		Update("revoked", true).Error; err != nil {
		t.Fatalf("revoke CAS token: %v", err)
	}
	rotatedAfterRevoke, err := compareAndSwapUserAuthTokenDigest(
		db,
		record.ID,
		record.UserID,
		firstHash,
		HashUserAuthToken("after-revoke"),
		UserAuthTokenFingerprint(HashUserAuthToken("after-revoke")),
		time.Now().Add(2*time.Hour),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("rotation after revoke: %v", err)
	}
	if rotatedAfterRevoke {
		t.Fatal("rotation won after revocation linearized")
	}
}

func TestRotateUserAuthTokenConcurrentRequestsReturnOneSecret(t *testing.T) {
	db := prepareUserAuthTokenModelTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	// Default update transactions acquire the only SQLite connection before
	// callbacks run, preventing both requests from reaching the barrier. The
	// production CAS is a single statement, so disabling that wrapper here
	// preserves its atomicity while making the interleaving deterministic.
	gormdb.DB = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	oldRaw := "concurrent-old-token"
	oldHash := HashUserAuthToken(oldRaw)
	record := &UserAuthToken{
		UserID:      "user-concurrent",
		TokenHash:   oldHash,
		Fingerprint: UserAuthTokenFingerprint(oldHash),
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	record.ID = "pat-concurrent-rotate"
	if err = db.Create(record).Error; err != nil {
		t.Fatalf("create concurrent token: %v", err)
	}

	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	callbackName := "test:pat-concurrent-rotate"
	if err = db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != record.TableName() {
			return
		}
		arrived <- struct{}{}
		<-release
	}); err != nil {
		t.Fatalf("register update barrier: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })

	type rotationResult struct {
		record *UserAuthToken
		raw    string
		err    error
	}
	results := make(chan rotationResult, 2)
	start := make(chan struct{})
	auth := newUserAuthTokenTestMiddleware(t)
	for range 2 {
		verifier := &User{}
		verifier.ID = record.UserID
		ctx, _ := gin.CreateTestContext(nil)
		go func() {
			<-start
			rotated, raw, rotateErr := RotateUserAuthToken(ctx, auth, verifier, record.ID)
			results <- rotationResult{record: rotated, raw: raw, err: rotateErr}
		}()
	}
	close(start)
	for range 2 {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("concurrent rotations did not reach the CAS barrier")
		}
	}
	close(release)

	successes := 0
	conflicts := 0
	winningRaw := ""
	for range 2 {
		select {
		case result := <-results:
			switch {
			case result.err == nil:
				successes++
				winningRaw = result.raw
				if result.record == nil || result.raw == "" {
					t.Fatal("successful rotation did not return its one-time secret")
				}
			case errors.Is(result.err, ErrUserAuthTokenRotationConflict):
				conflicts++
				if result.record != nil || result.raw != "" {
					t.Fatal("losing rotation exposed a generated secret")
				}
			default:
				t.Fatalf("unexpected rotation error: %v", result.err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for concurrent rotation result")
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("rotation outcomes = %d success/%d conflict, want 1/1", successes, conflicts)
	}

	stored := &UserAuthToken{}
	if err = db.First(stored, "id = ?", record.ID).Error; err != nil {
		t.Fatalf("load concurrently rotated token: %v", err)
	}
	if !VerifyUserAuthToken(winningRaw, stored.TokenHash) {
		t.Fatal("returned winning secret does not match the persisted digest")
	}
	if VerifyUserAuthToken(oldRaw, stored.TokenHash) {
		t.Fatal("old secret remained usable after rotation")
	}
}

func TestRotateUserAuthTokenLosesToConcurrentRevoke(t *testing.T) {
	db := prepareUserAuthTokenModelTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	gormdb.DB = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	oldRaw := "rotate-revoke-old-token"
	oldHash := HashUserAuthToken(oldRaw)
	record := &UserAuthToken{
		UserID:      "user-rotate-revoke",
		TokenHash:   oldHash,
		Fingerprint: UserAuthTokenFingerprint(oldHash),
		ExpiredAt:   time.Now().Add(time.Hour),
	}
	record.ID = "pat-rotate-revoke"
	if err = db.Create(record).Error; err != nil {
		t.Fatalf("create rotate/revoke token: %v", err)
	}

	arrived := make(chan struct{})
	release := make(chan struct{})
	var blocked atomic.Bool
	callbackName := "test:pat-rotate-revoke"
	if err = db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != record.TableName() || !blocked.CompareAndSwap(false, true) {
			return
		}
		close(arrived)
		<-release
	}); err != nil {
		t.Fatalf("register rotate/revoke barrier: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })

	verifier := &User{}
	verifier.ID = record.UserID
	ctx, _ := gin.CreateTestContext(nil)
	auth := newUserAuthTokenTestMiddleware(t)
	type rotationResult struct {
		raw string
		err error
	}
	result := make(chan rotationResult, 1)
	go func() {
		_, raw, rotateErr := RotateUserAuthToken(ctx, auth, verifier, record.ID)
		result <- rotationResult{raw: raw, err: rotateErr}
	}()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("rotation did not reach the CAS barrier")
	}
	if err = db.Model(&UserAuthToken{}).
		Where("id = ? AND user_id = ? AND revoked = ?", record.ID, record.UserID, false).
		Updates(map[string]any{"revoked": true, "token": ""}).Error; err != nil {
		close(release)
		t.Fatalf("concurrent revoke: %v", err)
	}
	close(release)

	select {
	case rotation := <-result:
		if !errors.Is(rotation.err, ErrUserAuthTokenRotationConflict) {
			t.Fatalf("rotation error = %v, want conflict", rotation.err)
		}
		if rotation.raw != "" {
			t.Fatal("rotation that lost to revoke exposed a secret")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for rotate/revoke result")
	}

	stored := &UserAuthToken{}
	if err = db.First(stored, "id = ?", record.ID).Error; err != nil {
		t.Fatalf("load revoked token: %v", err)
	}
	if !stored.Revoked {
		t.Fatal("rotate/revoke race did not preserve revocation")
	}
	principal := &User{}
	principal.PersonAccessToken = record.ID
	if err = principal.CheckToken(context.Background(), oldRaw); err == nil {
		t.Fatal("old bearer authenticated after revoke won the race")
	}
}

func prepareUserAuthTokenModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&Role{}, &User{}, &UserAuthToken{}); err != nil {
		t.Fatalf("migrate PAT test schema: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	return db
}

func newUserAuthTokenTestMiddleware(t *testing.T) *jwt.GinJWTMiddleware {
	t.Helper()
	auth := &jwt.GinJWTMiddleware{
		Key:     []byte("user-auth-token-test-signing-key"),
		Timeout: time.Hour,
		PayloadFunc: func(data any) jwt.MapClaims {
			verifier := data.(security.Verifier)
			return jwt.MapClaims{
				"refreshTokenDisabled": verifier.GetRefreshTokenDisable(),
				"personAccessToken":    verifier.GetPersonAccessToken(),
			}
		},
	}
	if err := auth.MiddlewareInit(); err != nil {
		t.Fatalf("initialize JWT middleware: %v", err)
	}
	return auth
}
