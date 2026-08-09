package apis

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/notice/email"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	adminChallengeSubjectKey = []byte("admin-challenge-subject-key-material-at-least-32-bytes")
	adminChallengePepper     = []byte("admin-challenge-pepper-material-at-least-32-bytes")
)

func TestEmailChallengePurposeIsolation(t *testing.T) {
	store, _, _ := installAdminChallenge(t, storagecache.ChallengeOptions{MaxAttempts: 10})
	var mu sync.Mutex
	codes := make([]string, 0, 3)
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, code, _ string) error {
		mu.Lock()
		codes = append(codes, code)
		mu.Unlock()
		return nil
	}}

	requests := []struct {
		useBy   string
		purpose storagecache.ChallengePurpose
	}{
		{email.LoginSender.String(), pkg.EmailLoginChallengePurpose},
		{email.RegisterSender.String(), pkg.EmailRegisterChallengePurpose},
		{email.ResetPasswordSender.String(), pkg.PasswordResetChallengePurpose},
	}
	for _, request := range requests {
		response := executeEmailChallengeRequest(t, handler, testChallengeEmail, request.useBy)
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s issue status = %d, body=%s", request.useBy, response.Code, response.Body.String())
		}
	}
	if len(codes) != len(requests) {
		t.Fatalf("delivered code count = %d, want %d", len(codes), len(requests))
	}
	for sourceIndex, code := range codes {
		for targetIndex, request := range requests {
			if sourceIndex == targetIndex {
				continue
			}
			ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, request.purpose, code)
			if err != nil || ok {
				t.Fatalf("code for %s verified as %s: ok=%v err=%v", requests[sourceIndex].useBy, request.useBy, ok, err)
			}
		}
	}
	for index, request := range requests {
		ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, request.purpose, codes[index])
		if err != nil || !ok {
			t.Fatalf("correct %s verification = %v, %v; want true, nil", request.useBy, ok, err)
		}
	}

	// Prove the three existing consumers request the same fixed purposes used by
	// issuance; no email-only fallback remains in models or password recovery.
	recorder := &recordingAdminChallenge{}
	center.SetChallenge(recorder)
	t.Cleanup(func() { center.SetChallenge(store) })
	prepareEmailRegistrationRole(t)
	gin.SetMode(gin.TestMode)
	loginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	loginContext.Request = httptest.NewRequest(http.MethodPost, "/admin/api/user/login", nil)
	for _, provider := range []pkg.LoginProvider{pkg.EmailLoginProvider, pkg.EmailRegisterProvider} {
		login := &models.UserLogin{Provider: provider, Email: testChallengeEmail, Captcha: "123456"}
		ok, principal, err := login.Verify(loginContext)
		if err != nil || ok || principal != nil {
			t.Fatalf("%s provisional Verify = %v, %#v, %v; want false, nil, nil", provider, ok, principal, err)
		}
	}
	reset := executePasswordReset(t, nil, `{"email":"person@example.com","captcha":"123456","password":"replacement-password"}`)
	if reset.Code != http.StatusForbidden {
		t.Fatalf("password reset invalid challenge status = %d, body=%s", reset.Code, reset.Body.String())
	}
	wantPurposes := []storagecache.ChallengePurpose{
		pkg.EmailLoginChallengePurpose,
		pkg.EmailRegisterChallengePurpose,
		pkg.PasswordResetChallengePurpose,
	}
	if got := recorder.snapshot(); len(got) != len(wantPurposes) {
		t.Fatalf("consumer challenge purposes = %v, want %v", got, wantPurposes)
	} else {
		for index := range wantPurposes {
			if got[index] != wantPurposes[index] {
				t.Fatalf("consumer challenge purposes = %v, want %v", got, wantPurposes)
			}
		}
	}
}

func TestEmailChallengeResponseDoesNotEnumerateAccount(t *testing.T) {
	_, _, _ = installAdminChallenge(t, storagecache.ChallengeOptions{})
	var deliveries atomic.Int64
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, _, _ string) error {
		deliveries.Add(1)
		return nil
	}}

	for _, useBy := range []string{email.LoginSender.String(), email.ResetPasswordSender.String()} {
		existing := executeEmailChallengeRequest(t, handler, "existing@example.com", useBy)
		missing := executeEmailChallengeRequest(t, handler, "missing@example.com", useBy)
		if existing.Code != http.StatusAccepted || missing.Code != http.StatusAccepted {
			t.Fatalf("%s statuses existing=%d missing=%d", useBy, existing.Code, missing.Code)
		}
		if existing.Body.String() != missing.Body.String() {
			t.Fatalf("%s response enumerates account: existing=%q missing=%q", useBy, existing.Body.String(), missing.Body.String())
		}
		if strings.Contains(existing.Body.String(), "example.com") || strings.Contains(strings.ToLower(existing.Body.String()), "record not found") {
			t.Fatalf("%s response leaked account material: %s", useBy, existing.Body.String())
		}
	}
	if deliveries.Load() != 4 {
		t.Fatalf("delivery count = %d, want 4 identical delivery attempts", deliveries.Load())
	}
	invalid := executeEmailChallengeRawRequest(t, handler, `{"email":"person@example.com","useBy":"unknown"}`)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid purpose status = %d, body=%s, want 422", invalid.Code, invalid.Body.String())
	}
}

func TestEmailChallengeCanonicalEmailBinding(t *testing.T) {
	store, _, _ := installAdminChallenge(t, storagecache.ChallengeOptions{})
	var code, recipient string
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, to, delivered, _ string) error {
		recipient = to
		code = delivered
		return nil
	}}
	response := executeEmailChallengeRequest(t, handler, "Person@EXAMPLE.COM", email.LoginSender.String())
	if response.Code != http.StatusAccepted {
		t.Fatalf("issue status = %d, body=%s, want 202", response.Code, response.Body.String())
	}
	if recipient != "person@example.com" {
		t.Fatalf("canonical delivery recipient = %q, want person@example.com", recipient)
	}
	if ok, err := store.VerifyChallenge(context.Background(), "person@example.com", pkg.EmailLoginChallengePurpose, code); err != nil || !ok {
		t.Fatalf("canonical verification = %v, %v; want true, nil", ok, err)
	}
}

func TestEmailChallengeRotatedSubjectCallerLimit(t *testing.T) {
	_, _, _ = installAdminChallenge(t, storagecache.ChallengeOptions{
		CallerLimit:  2,
		CallerWindow: time.Hour,
	})
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, _, _ string) error {
		return nil
	}}
	for index, address := range []string{"rotated-one@example.com", "rotated-two@example.com"} {
		response := executeEmailChallengeRequestWithNetworkIdentity(
			t,
			handler,
			address,
			email.LoginSender.String(),
			"198.51.100.10:4321",
			"203.0.113."+strconv.Itoa(index+1),
		)
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s status = %d, body=%s, want 202", address, response.Code, response.Body.String())
		}
	}
	response := executeEmailChallengeRequestWithNetworkIdentity(
		t,
		handler,
		"rotated-three@example.com",
		email.LoginSender.String(),
		"198.51.100.10:4321",
		"203.0.113.99",
	)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rotated-subject limit status = %d, body=%s, want 429", response.Code, response.Body.String())
	}
}

func TestEmailChallengeCallerUsesExplicitTrustedProxyPolicy(t *testing.T) {
	_, _, _ = installAdminChallenge(t, storagecache.ChallengeOptions{
		CallerLimit:  1,
		CallerWindow: time.Hour,
	})
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, _, _ string) error {
		return nil
	}}
	trustedProxies := []string{"10.0.0.0/8"}
	first := executeEmailChallengeRequestWithProxyPolicy(
		t,
		handler,
		"proxied-one@example.com",
		email.LoginSender.String(),
		"10.0.0.5:4321",
		"203.0.113.10",
		trustedProxies,
	)
	second := executeEmailChallengeRequestWithProxyPolicy(
		t,
		handler,
		"proxied-two@example.com",
		email.LoginSender.String(),
		"10.0.0.5:4321",
		"203.0.113.11",
		trustedProxies,
	)
	if first.Code != http.StatusAccepted || second.Code != http.StatusAccepted {
		t.Fatalf("trusted proxy callers statuses = %d/%d, want 202/202", first.Code, second.Code)
	}
}

func TestEmailChallengeRegistrationDisabledBeforeIssue(t *testing.T) {
	_, server, _ := installAdminChallenge(t, storagecache.ChallengeOptions{})
	center.SetChallenge(nil)
	center.SetAppConfig(emailChallengeAppConfig{
		"security:registerEnabled": "false",
		"security:emailEnabled":    "true",
		"email:smtpHost":           "smtp.example.test",
		"email:smtpPort":           "587",
		"email:username":           "mailer@example.test",
		"email:password":           "test-password",
	})
	var deliveries atomic.Int64
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, _, _ string) error {
		deliveries.Add(1)
		return nil
	}}
	response := executeEmailChallengeRequest(t, handler, "disabled-register@example.com", email.RegisterSender.String())
	if response.Code != http.StatusForbidden {
		t.Fatalf("disabled registration issue status = %d, body=%s, want 403", response.Code, response.Body.String())
	}
	if deliveries.Load() != 0 || len(server.Keys()) != 0 {
		t.Fatalf("disabled registration caused delivery/state: deliveries=%d keys=%v", deliveries.Load(), server.Keys())
	}
}

func TestEmailChallengeEmailCapabilityDisabledBeforeIssue(t *testing.T) {
	_, server, _ := installAdminChallenge(t, storagecache.ChallengeOptions{})
	center.SetChallenge(nil)
	center.SetAppConfig(emailChallengeAppConfig{
		"security:emailEnabled":    "false",
		"security:registerEnabled": "true",
		"email:smtpHost":           "smtp.example.test",
		"email:smtpPort":           "587",
		"email:username":           "mailer@example.test",
		"email:password":           "test-password",
	})
	var deliveries atomic.Int64
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, _, _ string) error {
		deliveries.Add(1)
		return nil
	}}
	response := executeEmailChallengeRequest(t, handler, "disabled-email@example.com", email.LoginSender.String())
	if response.Code != http.StatusForbidden {
		t.Fatalf("disabled email issue status = %d, body=%s, want 403", response.Code, response.Body.String())
	}
	if deliveries.Load() != 0 || len(server.Keys()) != 0 {
		t.Fatalf("disabled email caused delivery/state: deliveries=%d keys=%v", deliveries.Load(), server.Keys())
	}
}

func TestEmailChallengeCapabilityDisableRejectsEveryActiveCodeConsumer(t *testing.T) {
	store, _, _ := installAdminChallenge(t, storagecache.ChallengeOptions{MaxAttempts: 10})
	codes := make(map[storagecache.ChallengePurpose]string, 3)
	for _, purpose := range []storagecache.ChallengePurpose{
		pkg.EmailLoginChallengePurpose,
		pkg.EmailRegisterChallengePurpose,
		pkg.PasswordResetChallengePurpose,
	} {
		purpose := purpose
		if err := store.Issue(context.Background(), "capability-toggle", testChallengeEmail, purpose, func(_ context.Context, code string) error {
			codes[purpose] = code
			return nil
		}); err != nil {
			t.Fatalf("issue %s challenge: %v", purpose, err)
		}
	}
	center.SetAppConfig(emailChallengeAppConfig{
		"security:emailEnabled":    "false",
		"security:registerEnabled": "true",
	})

	loginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	loginContext.Request = httptest.NewRequest(http.MethodPost, "/admin/api/user/login", nil)
	for _, attempt := range []*models.UserLogin{
		{Provider: pkg.EmailLoginProvider, Email: testChallengeEmail, Captcha: codes[pkg.EmailLoginChallengePurpose]},
		{Provider: pkg.EmailRegisterProvider, Email: testChallengeEmail, Captcha: codes[pkg.EmailRegisterChallengePurpose]},
	} {
		ok, principal, err := attempt.Verify(loginContext)
		if err != nil || ok || principal != nil {
			t.Fatalf("disabled %s consumer = %v, %#v, %v; want false, nil, nil", attempt.Provider, ok, principal, err)
		}
	}
	reset := executePasswordReset(
		t,
		nil,
		`{"email":"person@example.com","captcha":"`+codes[pkg.PasswordResetChallengePurpose]+`","password":"replacement-password"}`,
	)
	if reset.Code != http.StatusForbidden {
		t.Fatalf("disabled password-reset consumer status = %d, body=%s, want 403", reset.Code, reset.Body.String())
	}

	// Capability disablement rejects before touching challenge state. Re-enable
	// direct verification to prove all three active codes were left unconsumed.
	for _, purpose := range []storagecache.ChallengePurpose{
		pkg.EmailLoginChallengePurpose,
		pkg.EmailRegisterChallengePurpose,
		pkg.PasswordResetChallengePurpose,
	} {
		ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, purpose, codes[purpose])
		if err != nil || !ok {
			t.Fatalf("disabled %s code state = %v, %v; want untouched active code", purpose, ok, err)
		}
	}
}

func TestEmailChallengeProviderOutageReturnsServiceUnavailable(t *testing.T) {
	_, _, _ = installAdminChallenge(t, storagecache.ChallengeOptions{})
	center.SetChallenge(nil)
	handler := &User{challengeSender: func(context.Context, string, string, string, string, string, string, string, string) error {
		return nil
	}}
	issue := executeEmailChallengeRequest(t, handler, testChallengeEmail, email.LoginSender.String())
	if issue.Code != http.StatusServiceUnavailable {
		t.Fatalf("challenge issue outage status = %d, body=%s, want 503", issue.Code, issue.Body.String())
	}
	reset := executePasswordReset(
		t,
		nil,
		`{"email":"person@example.com","captcha":"123456","password":"replacement-password"}`,
	)
	if reset.Code != http.StatusServiceUnavailable {
		t.Fatalf("password-reset outage status = %d, body=%s, want 503", reset.Code, reset.Body.String())
	}
}

func TestEmailChallengeHungSenderCancellationAndConcurrencyBound(t *testing.T) {
	_, _, _ = installAdminChallenge(t, storagecache.ChallengeOptions{})
	started := make(chan struct{})
	handler := &User{
		challengeSendSlots: make(chan struct{}, 1),
		challengeSender: func(ctx context.Context, _, _, _, _, _, _, _, _ string) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/api/user/fakeCaptcha", handler.FakeCaptcha)

	requestContext, cancel := context.WithCancel(context.Background())
	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/admin/api/user/fakeCaptcha",
		bytes.NewBufferString(`{"email":"hung-one@example.com","useBy":"login"}`),
	).WithContext(requestContext)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstResponse := httptest.NewRecorder()
	completed := make(chan struct{})
	go func() {
		router.ServeHTTP(firstResponse, firstRequest)
		close(completed)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SMTP sender")
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/admin/api/user/fakeCaptcha",
		bytes.NewBufferString(`{"email":"hung-two@example.com","useBy":"login"}`),
	)
	secondRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("concurrency-bound status = %d, body=%s, want 429", secondResponse.Code, secondResponse.Body.String())
	}

	cancel()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("canceled SMTP sender did not release the HTTP handler")
	}
	if firstResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("canceled delivery status = %d, body=%s, want 503", firstResponse.Code, firstResponse.Body.String())
	}
	if len(handler.challengeSendSlots) != 0 {
		t.Fatal("canceled delivery did not release its concurrency slot")
	}
}

func TestEmailChallengeSendFailureRotation(t *testing.T) {
	store, server, client := installAdminChallenge(t, storagecache.ChallengeOptions{MaxIssues: 3})
	var oldCode string
	if err := store.Issue(context.Background(), "direct-test", testChallengeEmail, pkg.EmailLoginChallengePurpose, func(_ context.Context, code string) error {
		oldCode = code
		return nil
	}); err != nil {
		t.Fatalf("issue prior active challenge: %v", err)
	}
	advanceAdminChallengeTime(t, client, server, time.Minute+time.Millisecond)

	var unsentCode string
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, code, _ string) error {
		unsentCode = code
		return errors.New("smtp detail for " + testChallengeEmail + " code=" + code)
	}}
	response := executeEmailChallengeRequest(t, handler, testChallengeEmail, email.LoginSender.String())
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("send failure status = %d, body=%s, want 503", response.Code, response.Body.String())
	}
	for _, secret := range []string{testChallengeEmail, oldCode, unsentCode, "smtp detail"} {
		if secret != "" && strings.Contains(response.Body.String(), secret) {
			t.Fatalf("send failure response leaked %q: %s", secret, response.Body.String())
		}
	}
	if ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, pkg.EmailLoginChallengePurpose, unsentCode); err != nil || ok {
		t.Fatalf("unsent code verification = %v, %v; want false, nil", ok, err)
	}
	if ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, pkg.EmailLoginChallengePurpose, oldCode); err != nil || !ok {
		t.Fatalf("prior active verification = %v, %v; want true, nil", ok, err)
	}

	// Abort does not start cooldown, so a new delivery can commit immediately.
	var replacementCode string
	if err := store.Issue(context.Background(), "direct-test", testChallengeEmail, pkg.EmailLoginChallengePurpose, func(_ context.Context, code string) error {
		replacementCode = code
		return nil
	}); err != nil {
		t.Fatalf("immediate replacement after abort: %v", err)
	}
	// The failed delivery remains charged: prior + failed + replacement exhausts
	// the configured rolling quota.
	advanceAdminChallengeTime(t, client, server, time.Minute+time.Millisecond)
	if err := store.Issue(context.Background(), "direct-test", testChallengeEmail, pkg.EmailLoginChallengePurpose, func(context.Context, string) error { return nil }); !errors.Is(err, storagecache.ErrChallengeQuota) {
		t.Fatalf("post-failure quota error = %v, want ErrChallengeQuota", err)
	}
	if ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, pkg.EmailLoginChallengePurpose, replacementCode); err != nil || !ok {
		t.Fatalf("replacement verification = %v, %v; want true, nil", ok, err)
	}
}

func TestEmailChallengeConcurrentSendFailureCAS(t *testing.T) {
	store, server, client := installAdminChallenge(t, storagecache.ChallengeOptions{})
	deliveryStarted := make(chan struct{})
	releaseDelivery := make(chan struct{})
	var oldCode string
	handler := &User{challengeSender: func(_ context.Context, _, _, _, _, _, _, code, _ string) error {
		oldCode = code
		close(deliveryStarted)
		<-releaseDelivery
		return errors.New("late transport failure")
	}}
	responseChannel := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		responseChannel <- executeEmailChallengeRequest(t, handler, testChallengeEmail, email.LoginSender.String())
	}()
	select {
	case <-deliveryStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first delivery")
	}
	advanceAdminChallengeTime(t, client, server, 30*time.Second+time.Millisecond)

	var currentCode string
	if err := store.Issue(context.Background(), "direct-test", testChallengeEmail, pkg.EmailLoginChallengePurpose, func(_ context.Context, code string) error {
		currentCode = code
		return nil
	}); err != nil {
		t.Fatalf("replacement delivery: %v", err)
	}
	close(releaseDelivery)
	var response *httptest.ResponseRecorder
	select {
	case response = <-responseChannel:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stale delivery completion")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("stale delivery status = %d, body=%s, want 503", response.Code, response.Body.String())
	}
	if ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, pkg.EmailLoginChallengePurpose, oldCode); err != nil || ok {
		t.Fatalf("stale delivery code verification = %v, %v; want false, nil", ok, err)
	}
	if ok, err := store.VerifyChallenge(context.Background(), testChallengeEmail, pkg.EmailLoginChallengePurpose, currentCode); err != nil || !ok {
		t.Fatalf("replacement code verification = %v, %v; want true, nil", ok, err)
	}
}

const testChallengeEmail = "person@example.com"

type emailChallengeAppConfig map[string]string

type recordingAdminChallenge struct {
	mu       sync.Mutex
	purposes []storagecache.ChallengePurpose
}

func (*recordingAdminChallenge) Ready(context.Context) error { return nil }

func (*recordingAdminChallenge) Issue(
	context.Context,
	string,
	string,
	storagecache.ChallengePurpose,
	func(context.Context, string) error,
) error {
	return errors.New("unexpected Issue call")
}

func (r *recordingAdminChallenge) VerifyChallenge(
	_ context.Context,
	_ string,
	purpose storagecache.ChallengePurpose,
	_ string,
) (bool, error) {
	r.mu.Lock()
	r.purposes = append(r.purposes, purpose)
	r.mu.Unlock()
	return false, nil
}

func (r *recordingAdminChallenge) snapshot() []storagecache.ChallengePurpose {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]storagecache.ChallengePurpose(nil), r.purposes...)
}

func (c emailChallengeAppConfig) SetAppConfig(*gin.Context, string, bool, string) error { return nil }

func (c emailChallengeAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := c[key]
	return value, ok
}

func installAdminChallenge(
	t *testing.T,
	overrides storagecache.ChallengeOptions,
) (*storagecache.RedisChallengeStore, *miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	server := miniredis.RunT(t)
	server.SetTime(time.Unix(1_800_000_000, 0).UTC())
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{server.Addr()}, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	options := storagecache.ChallengeOptions{
		Namespace:      "test:admin-challenge",
		SubjectKey:     adminChallengeSubjectKey,
		Peppers:        []storagecache.ChallengePepper{{Version: "v1", Secret: adminChallengePepper}},
		CodeTTL:        5 * time.Minute,
		PendingTTL:     30 * time.Second,
		Cooldown:       time.Minute,
		QuotaWindow:    time.Hour,
		MaxIssues:      10,
		MaxAttempts:    5,
		IdempotencyTTL: 2 * time.Minute,
		CallerWindow:   10 * time.Minute,
		CallerLimit:    10,
		GlobalWindow:   10 * time.Minute,
		GlobalLimit:    1000,
	}
	if overrides.MaxIssues != 0 {
		options.MaxIssues = overrides.MaxIssues
	}
	if overrides.MaxAttempts != 0 {
		options.MaxAttempts = overrides.MaxAttempts
	}
	if overrides.CallerWindow != 0 {
		options.CallerWindow = overrides.CallerWindow
	}
	if overrides.CallerLimit != 0 {
		options.CallerLimit = overrides.CallerLimit
	}
	if overrides.GlobalWindow != 0 {
		options.GlobalWindow = overrides.GlobalWindow
	}
	if overrides.GlobalLimit != 0 {
		options.GlobalLimit = overrides.GlobalLimit
	}
	store, err := storagecache.NewRedisChallengeStore(client, options)
	if err != nil {
		t.Fatalf("NewRedisChallengeStore: %v", err)
	}
	previousChallenge := center.GetChallenge()
	previousAppConfig := center.GetAppConfig()
	center.SetChallenge(store)
	center.SetAppConfig(emailChallengeAppConfig{
		"email:smtpHost":           "smtp.example.test",
		"email:smtpPort":           "587",
		"email:username":           "mailer@example.test",
		"email:password":           "test-password",
		"base:websiteName":         "MSS Test",
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	t.Cleanup(func() {
		center.SetChallenge(previousChallenge)
		center.SetAppConfig(previousAppConfig)
	})
	return store, server, client
}

func prepareEmailRegistrationRole(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open registration role database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open registration SQL handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&models.Role{}); err != nil {
		t.Fatalf("migrate registration role: %v", err)
	}
	if err = db.Exec(
		"INSERT INTO mss_boot_roles (id, name, root, `default`, status) VALUES (?, ?, ?, ?, ?)",
		"role-default", "Default", false, true, enum.Enabled,
	).Error; err != nil {
		t.Fatalf("seed registration role: %v", err)
	}
	previous := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previous })
}

func executeEmailChallengeRequest(t *testing.T, handler *User, address, useBy string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"email":"` + address + `","useBy":"` + useBy + `"}`
	return executeEmailChallengeRawRequest(t, handler, body)
}

func executeEmailChallengeRequestWithNetworkIdentity(
	t *testing.T,
	handler *User,
	address string,
	useBy string,
	remoteAddr string,
	forwardedFor string,
) *httptest.ResponseRecorder {
	return executeEmailChallengeRequestWithProxyPolicy(
		t,
		handler,
		address,
		useBy,
		remoteAddr,
		forwardedFor,
		nil,
	)
}

func executeEmailChallengeRequestWithProxyPolicy(
	t *testing.T,
	handler *User,
	address string,
	useBy string,
	remoteAddr string,
	forwardedFor string,
	trustedProxies []string,
) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"email":"` + address + `","useBy":"` + useBy + `"}`
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		t.Fatalf("configure trusted proxies: %v", err)
	}
	router.POST("/admin/api/user/fakeCaptcha", handler.FakeCaptcha)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/fakeCaptcha", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", forwardedFor)
	request.RemoteAddr = remoteAddr
	router.ServeHTTP(response, request)
	return response
}

func executeEmailChallengeRawRequest(t *testing.T, handler *User, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		t.Fatalf("disable trusted proxies: %v", err)
	}
	router.POST("/admin/api/user/fakeCaptcha", handler.FakeCaptcha)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/user/fakeCaptcha", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}

func advanceAdminChallengeTime(
	t *testing.T,
	client redis.UniversalClient,
	server *miniredis.Miniredis,
	duration time.Duration,
) {
	t.Helper()
	now, err := client.Time(context.Background()).Result()
	if err != nil {
		t.Fatalf("read Redis test clock: %v", err)
	}
	server.FastForward(duration)
	server.SetTime(now.Add(duration))
}
