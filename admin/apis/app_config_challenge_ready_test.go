package apis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	runtimechallenge "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/challenge"
)

type appConfigReadinessChallenge struct {
	ready  atomic.Bool
	calls  atomic.Int32
	block  bool
	budget chan time.Duration
}

type appConfigReadinessSettings map[string]string

func (c appConfigReadinessSettings) SetAppConfig(*gin.Context, string, bool, string) error {
	return nil
}

func (c appConfigReadinessSettings) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := c[key]
	return value, ok
}

func installAppConfigReadinessSettings(t *testing.T, settings appConfigReadinessSettings) {
	t.Helper()
	previous := center.GetAppConfig()
	center.SetAppConfig(settings)
	t.Cleanup(func() { center.SetAppConfig(previous) })
}

func (c *appConfigReadinessChallenge) Ready(ctx context.Context) error {
	c.calls.Add(1)
	if c.block {
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("readiness context has no deadline")
		}
		select {
		case c.budget <- time.Until(deadline):
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if !c.ready.Load() {
		return errors.New("challenge unavailable")
	}
	return nil
}

func (*appConfigReadinessChallenge) BeginIssue(
	context.Context,
	runtimechallenge.BeginRequest,
) (runtimechallenge.BeginOutcome, error) {
	return runtimechallenge.BeginOutcome{}, errors.New("not implemented")
}

func (*appConfigReadinessChallenge) Commit(context.Context, *runtimechallenge.Reservation) error {
	return errors.New("not implemented")
}

func (*appConfigReadinessChallenge) Abort(context.Context, *runtimechallenge.Reservation) error {
	return errors.New("not implemented")
}

func (*appConfigReadinessChallenge) Verify(
	context.Context,
	runtimechallenge.VerifyRequest,
) (runtimechallenge.VerifyOutcome, error) {
	return runtimechallenge.VerifyRejected, errors.New("not implemented")
}

func appConfigProfileForReadinessTest(t *testing.T) map[string]map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/app-configs/profile", nil)
	(&AppConfig{}).Profile(ctx)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	profile := make(map[string]map[string]any)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &profile))
	return profile
}

func TestAppConfigProfileProjectsFreshEmailChallengeReadinessAfterCachedProfile(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.Create([]*models.AppConfig{
		{Group: "security", Name: "emailEnabled", Value: "true"},
		{Group: "security", Name: "registerEnabled", Value: "true"},
		{Group: "email", Name: "smtpHost", Value: "smtp.example.test"},
		{Group: "email", Name: "smtpPort", Value: "587"},
		{Group: "email", Name: "username", Value: "mailer@example.test"},
		{Group: "email", Name: "password", Value: "test-password"},
	}).Error)
	installAppConfigReadinessSettings(t, appConfigReadinessSettings{
		"email:smtpHost": "smtp.example.test",
		"email:smtpPort": "587",
		"email:username": "mailer@example.test",
		"email:password": "test-password",
	})

	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	profileCache, err := cacheconfig.NewRedis(client, nil)
	require.NoError(t, err)
	previousCache := center.GetCache()
	previousChallenge := center.GetRuntimeChallenge()
	center.SetCache(profileCache)
	challenge := &appConfigReadinessChallenge{}
	challenge.ready.Store(true)
	center.SetRuntimeChallenge(challenge)
	t.Cleanup(func() {
		center.SetRuntimeChallenge(previousChallenge)
		center.SetCache(previousCache)
		_ = profileCache.Close()
	})

	first := appConfigProfileForReadinessTest(t)
	require.Equal(t, true, first["security"]["emailEnabled"])
	require.Equal(t, true, first["security"]["emailChallengeReady"])

	// No configuration revision changes between requests, so the static profile
	// is served from its versioned cache. Readiness must still be re-evaluated.
	challenge.ready.Store(false)
	second := appConfigProfileForReadinessTest(t)
	require.Equal(t, false, second["security"]["emailChallengeReady"])
	require.EqualValues(t, 2, challenge.calls.Load())

	center.SetRuntimeChallenge(nil)
	third := appConfigProfileForReadinessTest(t)
	require.Equal(t, false, third["security"]["emailChallengeReady"])
	require.EqualValues(t, 2, challenge.calls.Load())
}

func TestAppConfigProfileChallengeReadinessUsesBoundedFailClosedCheck(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	require.NoError(t, db.Create([]*models.AppConfig{
		{Group: "email", Name: "smtpHost", Value: "smtp.example.test"},
		{Group: "email", Name: "smtpPort", Value: "587"},
		{Group: "email", Name: "username", Value: "mailer@example.test"},
		{Group: "email", Name: "password", Value: "test-password"},
	}).Error)
	installAppConfigReadinessSettings(t, appConfigReadinessSettings{
		"email:smtpHost": "smtp.example.test",
		"email:smtpPort": "587",
		"email:username": "mailer@example.test",
		"email:password": "test-password",
	})
	gin.SetMode(gin.TestMode)
	previousChallenge := center.GetRuntimeChallenge()
	challenge := &appConfigReadinessChallenge{
		block:  true,
		budget: make(chan time.Duration, 1),
	}
	center.SetRuntimeChallenge(challenge)
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })

	started := time.Now()
	profile := appConfigProfileForReadinessTest(t)
	elapsed := time.Since(started)
	require.Equal(t, false, profile["security"]["emailChallengeReady"])
	require.Less(t, elapsed, time.Second)

	budget := <-challenge.budget
	require.Positive(t, budget)
	require.LessOrEqual(t, budget, emailChallengeReadyLimit)
}

func TestAppConfigProfileChallengeReadinessRequiresCompleteSMTPConfig(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	require.NoError(t, db.Create([]*models.AppConfig{
		{Group: "security", Name: "emailEnabled", Value: "true"},
		{Group: "email", Name: "smtpHost", Value: "smtp.example.test"},
		{Group: "email", Name: "smtpPort", Value: "587"},
		{Group: "email", Name: "username", Value: "mailer@example.test"},
		// password deliberately missing
	}).Error)
	installAppConfigReadinessSettings(t, appConfigReadinessSettings{
		"email:smtpHost": "smtp.example.test",
		"email:smtpPort": "587",
		"email:username": "mailer@example.test",
		// password deliberately missing
	})
	previousChallenge := center.GetRuntimeChallenge()
	challenge := &appConfigReadinessChallenge{}
	challenge.ready.Store(true)
	center.SetRuntimeChallenge(challenge)
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })

	profile := appConfigProfileForReadinessTest(t)
	require.Equal(t, false, profile["security"]["emailChallengeReady"])
	require.Zero(t, challenge.calls.Load(), "dependency health must not run for guaranteed-invalid SMTP config")
}
