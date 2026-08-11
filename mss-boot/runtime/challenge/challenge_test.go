package challenge

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/internal/redisbridge"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

var (
	testSubjectKey = []byte("0123456789abcdefGHIJKLMNOPQRSTUV")
	testPepperV1   = []byte("abcdefghijklmnopqrstuvwxyzABCDEF")
	testPepperV2   = []byte("ZYXWVUTSRQPONMLKjihgfedcba987654")
)

const (
	testSubject = "Person+Case@example.test"
	testPurpose = Purpose("login")
)

type publicChallengeContract interface {
	Ready(context.Context) error
	BeginIssue(context.Context, BeginRequest) (BeginOutcome, error)
	Commit(context.Context, *Reservation) error
	Abort(context.Context, *Reservation) error
	Verify(context.Context, VerifyRequest) (VerifyOutcome, error)
}

var _ publicChallengeContract = (*Redis)(nil)

func TestIssueCommitVerifyAndReservationRedaction(t *testing.T) {
	store, fixture := newTestStore(t, Options{})
	ctx := context.Background()
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	outcome, err := store.BeginIssue(ctx, BeginRequest{Caller: "caller-one", Subject: testSubject, Purpose: testPurpose})
	if err != nil {
		t.Fatalf("BeginIssue: %v", err)
	}
	reservation, ok := outcome.Reservation()
	if !ok || outcome.State() != BeginReserved {
		t.Fatalf("BeginIssue outcome = %#v, want reserved", outcome)
	}
	code := reservation.Code()
	if !validCode(code) {
		t.Fatalf("Code = %q, want six digits", code)
	}
	for _, formatted := range []string{fmt.Sprint(reservation), fmt.Sprintf("%#v", reservation)} {
		if strings.Contains(formatted, code) || strings.Contains(formatted, testSubject) || strings.Contains(formatted, "version") {
			t.Fatalf("Reservation formatting leaked material: %q", formatted)
		}
	}
	for _, formatted := range []string{fmt.Sprint(outcome), fmt.Sprintf("%#v", outcome)} {
		if formatted != "ChallengeBeginOutcome{reserved}" || strings.Contains(formatted, code) || strings.Contains(formatted, testSubject) {
			t.Fatalf("BeginOutcome formatting = %q; want fixed redacted value", formatted)
		}
	}
	if VerifyVerified.String() != "verified" || VerifyRejected.String() != "rejected" || VerifyOutcome(255).String() != "rejected" {
		t.Fatal("VerifyOutcome formatting exposed a non-fixed diagnostic")
	}
	for _, value := range []any{Reservation{}, BeginOutcome{}} {
		typeOf := reflect.TypeOf(value)
		for index := range typeOf.NumField() {
			if typeOf.Field(index).IsExported() {
				t.Fatalf("%s exposes field %q", typeOf.Name(), typeOf.Field(index).Name)
			}
		}
	}
	if err = store.Commit(ctx, reservation); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = store.Commit(ctx, reservation); err != nil {
		t.Fatalf("idempotent Commit: %v", err)
	}
	if verified, verifyErr := store.Verify(ctx, VerifyRequest{Subject: testSubject, Purpose: testPurpose, Code: code}); verifyErr != nil || verified != VerifyVerified {
		t.Fatalf("Verify = %v, %v; want verified, nil", verified, verifyErr)
	}
	if verified, verifyErr := store.Verify(ctx, VerifyRequest{Subject: testSubject, Purpose: testPurpose, Code: code}); verifyErr != nil || verified != VerifyRejected {
		t.Fatalf("replay Verify = %v, %v; want rejected, nil", verified, verifyErr)
	}
	assertRedisMaterialRedacted(t, fixture.server, testSubject, code)
}

func TestPublicFormattingAndErrorChainsRedactAllMaterial(t *testing.T) {
	store, fixture := newTestStore(t, Options{})
	providerEndpoint := fixture.server.Addr()
	request := BeginRequest{Caller: "private-caller", Subject: testSubject, Purpose: testPurpose}
	outcome, err := store.BeginIssue(context.Background(), request)
	if err != nil {
		t.Fatalf("BeginIssue: %v", err)
	}
	reservation, ok := outcome.Reservation()
	if !ok {
		t.Fatalf("outcome = %#v, want reservation", outcome)
	}
	verifyRequest := VerifyRequest{Subject: testSubject, Purpose: testPurpose, Code: reservation.Code()}
	options := testOptions(Options{})
	cases := []struct {
		value    any
		redacted string
	}{
		{options.Peppers[0], "ChallengePepper<redacted>"},
		{&options.Peppers[0], "ChallengePepper<redacted>"},
		{options, "ChallengeOptions<redacted>"},
		{&options, "ChallengeOptions<redacted>"},
		{request, "ChallengeBeginRequest<redacted>"},
		{&request, "ChallengeBeginRequest<redacted>"},
		{verifyRequest, "ChallengeVerifyRequest<redacted>"},
		{&verifyRequest, "ChallengeVerifyRequest<redacted>"},
		{store, "RedisChallenge<redacted>"},
		{reservation, "ChallengeReservation<opaque>"},
		{outcome, "ChallengeBeginOutcome{reserved}"},
	}
	for _, test := range cases {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			formatted := fmt.Sprintf(format, test.value)
			want := test.redacted
			if format == "%q" {
				want = fmt.Sprintf("%q", test.redacted)
			}
			if formatted != want {
				t.Fatalf("fmt %q on %T = %q, want %q", format, test.value, formatted, want)
			}
			assertNoMaterial(t, formatted,
				testSubject, "private-caller", reservation.Code(), providerEndpoint,
				string(options.SubjectKey), string(options.Peppers[0].Secret),
			)
		}
		jsonValue, jsonErr := json.Marshal(test.value)
		if jsonErr != nil {
			t.Fatalf("json.Marshal(%T): %v", test.value, jsonErr)
		}
		wantJSON, _ := json.Marshal(test.redacted)
		if !bytes.Equal(jsonValue, wantJSON) {
			t.Fatalf("json.Marshal(%T) = %s, want %s", test.value, jsonValue, wantJSON)
		}
		yamlValue, yamlErr := yaml.Marshal(test.value)
		if yamlErr != nil {
			t.Fatalf("yaml.Marshal(%T): %v", test.value, yamlErr)
		}
		wantYAML, _ := yaml.Marshal(test.redacted)
		if !bytes.Equal(yamlValue, wantYAML) {
			t.Fatalf("yaml.Marshal(%T) = %q, want %q", test.value, yamlValue, wantYAML)
		}
		textLog, jsonLog := renderChallengeLogs(test.value)
		wantTextLog, wantJSONLog := renderChallengeLogs(test.redacted)
		if textLog != wantTextLog || jsonLog != wantJSONLog {
			t.Fatalf("slog(%T) text=%q json=%q; want text=%q json=%q", test.value, textLog, jsonLog, wantTextLog, wantJSONLog)
		}
		for _, serialized := range []string{string(jsonValue), string(yamlValue), textLog, jsonLog} {
			assertNoMaterial(t, serialized,
				testSubject, "private-caller", reservation.Code(), providerEndpoint,
				string(options.SubjectKey), string(options.Peppers[0].Secret),
			)
		}
	}

	fixture.server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	outageErr := store.Ready(ctx)
	if !errors.Is(outageErr, ErrUnavailable) {
		t.Fatalf("Ready outage = %v", outageErr)
	}
	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	canceledErr := store.Ready(canceled)
	if !errors.Is(canceledErr, ErrUnavailable) || !errors.Is(canceledErr, context.Canceled) {
		t.Fatalf("Ready canceled = %v, want unavailable+canceled", canceledErr)
	}
	for _, publicErr := range []error{outageErr, canceledErr} {
		var lifecycle *redisresource.LifecycleError
		var network *net.OpError
		if errors.As(publicErr, &lifecycle) || errors.As(publicErr, &network) {
			t.Fatalf("challenge error chain exposed provider/resource error: %T", publicErr)
		}
		for _, current := range challengeErrorTree(publicErr, 0) {
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
				assertNoMaterial(t, fmt.Sprintf(format, current),
					testSubject, "private-caller", reservation.Code(), providerEndpoint,
					string(options.SubjectKey), string(options.Peppers[0].Secret),
				)
			}
		}
	}
}

func TestPendingAbortStaleRecoveryCooldownAndQuota(t *testing.T) {
	store, fixture := newTestStore(t, Options{
		PendingTTL:  5 * time.Second,
		Cooldown:    10 * time.Second,
		MaxIssues:   3,
		QuotaWindow: time.Hour,
		CallerLimit: 100,
		GlobalLimit: 100,
	})
	ctx := context.Background()
	firstOutcome, err := store.BeginIssue(ctx, BeginRequest{Caller: "caller-a", Subject: testSubject, Purpose: testPurpose})
	if err != nil {
		t.Fatalf("first BeginIssue: %v", err)
	}
	first, ok := firstOutcome.Reservation()
	if !ok {
		t.Fatalf("first outcome = %#v, want reservation", firstOutcome)
	}
	if next, pendingErr := store.BeginIssue(ctx, BeginRequest{Caller: "caller-b", Subject: testSubject, Purpose: testPurpose}); pendingErr != nil || next.State() != BeginPending {
		t.Fatalf("concurrent BeginIssue = %#v, %v; want pending outcome", next, pendingErr)
	}
	if err = store.Abort(ctx, first); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if err = store.Abort(ctx, first); err != nil {
		t.Fatalf("idempotent Abort: %v", err)
	}
	secondOutcome, err := store.BeginIssue(ctx, BeginRequest{Caller: "caller-c", Subject: testSubject, Purpose: testPurpose})
	if err != nil {
		t.Fatalf("post-abort BeginIssue: %v", err)
	}
	second, ok := secondOutcome.Reservation()
	if !ok {
		t.Fatalf("second outcome = %#v", secondOutcome)
	}
	fixture.advance(5*time.Second + time.Millisecond)
	replacementOutcome, err := store.BeginIssue(ctx, BeginRequest{Caller: "caller-d", Subject: testSubject, Purpose: testPurpose})
	if err != nil {
		t.Fatalf("post-lease BeginIssue: %v", err)
	}
	replacement, ok := replacementOutcome.Reservation()
	if !ok {
		t.Fatalf("replacement outcome = %#v", replacementOutcome)
	}
	if err = store.Commit(ctx, second); !errors.Is(err, ErrStale) {
		t.Fatalf("stale Commit error = %v, want ErrStale", err)
	}
	if err = store.Abort(ctx, second); !errors.Is(err, ErrStale) {
		t.Fatalf("stale Abort error = %v, want ErrStale", err)
	}
	if err = store.Commit(ctx, replacement); err != nil {
		t.Fatalf("replacement Commit: %v", err)
	}
	if next, cooldownErr := store.BeginIssue(ctx, BeginRequest{Caller: "caller-e", Subject: testSubject, Purpose: testPurpose}); cooldownErr != nil || next.State() != BeginCooldown {
		t.Fatalf("cooldown BeginIssue = %#v, %v; want cooldown", next, cooldownErr)
	}
	fixture.advance(10*time.Second + time.Millisecond)
	if next, quotaErr := store.BeginIssue(ctx, BeginRequest{Caller: "caller-f", Subject: testSubject, Purpose: testPurpose}); quotaErr != nil || next.State() != BeginQuota {
		t.Fatalf("quota BeginIssue = %#v, %v; want quota", next, quotaErr)
	}
}

func TestPendingAbortAndReclaimPreserveActiveVerifier(t *testing.T) {
	options := Options{
		CodeTTL: 30 * time.Second, PendingTTL: 2 * time.Second,
		Cooldown: 5 * time.Second, MaxIssues: 10,
		CallerLimit: 100, GlobalLimit: 100,
	}
	t.Run("abort", func(t *testing.T) {
		store, fixture := newTestStore(t, options)
		old := mustCommitted(t, store, "preserve-abort@example.test")
		stateKey := activeStateKey(t, fixture.server)
		originalExpiry := fixture.server.HGet(stateKey, "active_expires_at")
		fixture.advance(5*time.Second + time.Millisecond)
		expectedPendingCode := setNextIssueCode(store, old.Code())
		outcome, err := store.BeginIssue(context.Background(), BeginRequest{
			Caller: "pending-abort", Subject: "preserve-abort@example.test", Purpose: testPurpose,
		})
		store.random = cryptorand.Reader
		if err != nil {
			t.Fatalf("pending BeginIssue: %v", err)
		}
		pending, ok := outcome.Reservation()
		if !ok || pending.Code() != expectedPendingCode {
			t.Fatalf("pending outcome = %#v code=%q", outcome, pending.Code())
		}
		if got := fixture.server.HGet(stateKey, "active_expires_at"); got != originalExpiry {
			t.Fatalf("pending changed active expiry: got %q want %q", got, originalExpiry)
		}
		if verified, verifyErr := store.Verify(context.Background(), VerifyRequest{
			Subject: "preserve-abort@example.test", Purpose: testPurpose, Code: pending.Code(),
		}); verifyErr != nil || verified != VerifyRejected {
			t.Fatalf("pending code Verify = %v, %v", verified, verifyErr)
		}
		if err = store.Abort(context.Background(), pending); err != nil {
			t.Fatalf("Abort: %v", err)
		}
		if got := fixture.server.HGet(stateKey, "active_expires_at"); got != originalExpiry {
			t.Fatalf("Abort changed active expiry: got %q want %q", got, originalExpiry)
		}
		if verified, verifyErr := store.Verify(context.Background(), VerifyRequest{
			Subject: "preserve-abort@example.test", Purpose: testPurpose, Code: old.Code(),
		}); verifyErr != nil || verified != VerifyVerified {
			t.Fatalf("old code after Abort = %v, %v", verified, verifyErr)
		}
	})

	t.Run("expired reclaim", func(t *testing.T) {
		store, fixture := newTestStore(t, options)
		old := mustCommitted(t, store, "preserve-reclaim@example.test")
		stateKey := activeStateKey(t, fixture.server)
		originalExpiry := fixture.server.HGet(stateKey, "active_expires_at")
		fixture.advance(5*time.Second + time.Millisecond)
		orphanOutcome, err := store.BeginIssue(context.Background(), BeginRequest{
			Caller: "orphan", Subject: "preserve-reclaim@example.test", Purpose: testPurpose,
		})
		if err != nil {
			t.Fatalf("orphan BeginIssue: %v", err)
		}
		orphan, ok := orphanOutcome.Reservation()
		if !ok {
			t.Fatalf("orphan outcome = %#v", orphanOutcome)
		}
		fixture.advance(2*time.Second + time.Millisecond)
		replacementOutcome, err := store.BeginIssue(context.Background(), BeginRequest{
			Caller: "replacement", Subject: "preserve-reclaim@example.test", Purpose: testPurpose,
		})
		if err != nil {
			t.Fatalf("replacement BeginIssue: %v", err)
		}
		replacement, ok := replacementOutcome.Reservation()
		if !ok {
			t.Fatalf("replacement outcome = %#v", replacementOutcome)
		}
		if got := fixture.server.HGet(stateKey, "active_expires_at"); got != originalExpiry {
			t.Fatalf("reclaim changed active expiry: got %q want %q", got, originalExpiry)
		}
		if err = store.Commit(context.Background(), orphan); !errors.Is(err, ErrStale) {
			t.Fatalf("orphan Commit = %v", err)
		}
		if err = store.Abort(context.Background(), orphan); !errors.Is(err, ErrStale) {
			t.Fatalf("orphan Abort = %v", err)
		}
		if err = store.Abort(context.Background(), replacement); err != nil {
			t.Fatalf("replacement Abort = %v", err)
		}
		if verified, verifyErr := store.Verify(context.Background(), VerifyRequest{
			Subject: "preserve-reclaim@example.test", Purpose: testPurpose, Code: old.Code(),
		}); verifyErr != nil || verified != VerifyVerified {
			t.Fatalf("old code after reclaim = %v, %v", verified, verifyErr)
		}
	})
}

func TestSameSubjectPurposesAreIsolated(t *testing.T) {
	store, _ := newTestStore(t, Options{CallerLimit: 100, GlobalLimit: 100})
	const subject = "multi-purpose@example.test"
	login := mustCommitted(t, store, subject)
	expectedRegisterCode := setNextIssueCode(store, login.Code())
	registerOutcome, err := store.BeginIssue(context.Background(), BeginRequest{
		Caller: "register-caller", Subject: subject, Purpose: Purpose("register"),
	})
	store.random = cryptorand.Reader
	if err != nil {
		t.Fatalf("register BeginIssue: %v", err)
	}
	register, ok := registerOutcome.Reservation()
	if !ok || register.Code() != expectedRegisterCode {
		t.Fatalf("register outcome = %#v", registerOutcome)
	}
	if err = store.Commit(context.Background(), register); err != nil {
		t.Fatalf("register Commit: %v", err)
	}
	for _, request := range []VerifyRequest{
		{Subject: subject, Purpose: testPurpose, Code: register.Code()},
		{Subject: subject, Purpose: Purpose("register"), Code: login.Code()},
	} {
		if outcome, verifyErr := store.Verify(context.Background(), request); verifyErr != nil || outcome != VerifyRejected {
			t.Fatalf("cross-purpose Verify = %v, %v", outcome, verifyErr)
		}
	}
	if outcome, verifyErr := store.Verify(context.Background(), VerifyRequest{Subject: subject, Purpose: testPurpose, Code: login.Code()}); verifyErr != nil || outcome != VerifyVerified {
		t.Fatalf("login Verify = %v, %v", outcome, verifyErr)
	}
	if outcome, verifyErr := store.Verify(context.Background(), VerifyRequest{Subject: subject, Purpose: Purpose("register"), Code: register.Code()}); verifyErr != nil || outcome != VerifyVerified {
		t.Fatalf("register Verify = %v, %v", outcome, verifyErr)
	}
}

func TestCallerAndGlobalLimitsAreAtomicAcrossSubjects(t *testing.T) {
	store, _ := newTestStore(t, Options{
		CallerLimit: 2, GlobalLimit: 3,
		CallerWindow: time.Hour, GlobalWindow: time.Hour,
		Cooldown: time.Millisecond, MaxIssues: 100,
	})
	ctx := context.Background()
	begin := func(caller, subject string) (BeginState, error) {
		outcome, err := store.BeginIssue(ctx, BeginRequest{Caller: caller, Subject: subject, Purpose: testPurpose})
		if err != nil {
			return 0, err
		}
		if outcome.State() == BeginQuota {
			return BeginQuota, nil
		}
		reservation, ok := outcome.Reservation()
		if !ok {
			return outcome.State(), fmt.Errorf("unexpected outcome %v", outcome.State())
		}
		return outcome.State(), store.Abort(ctx, reservation)
	}
	if state, err := begin("one", "first@example.test"); err != nil || state != BeginReserved {
		t.Fatalf("first = %v, %v", state, err)
	}
	if state, err := begin("one", "second@example.test"); err != nil || state != BeginReserved {
		t.Fatalf("second = %v, %v", state, err)
	}
	if state, err := begin("one", "rotated@example.test"); err != nil || state != BeginQuota {
		t.Fatalf("caller quota = %v, %v", state, err)
	}
	if state, err := begin("two", "third@example.test"); err != nil || state != BeginReserved {
		t.Fatalf("global third = %v, %v", state, err)
	}
	if state, err := begin("three", "fourth@example.test"); err != nil || state != BeginQuota {
		t.Fatalf("global quota = %v, %v", state, err)
	}
}

func TestRateScriptReplayAtLimitIsIdempotent(t *testing.T) {
	store, _ := newTestStore(t, Options{
		CallerLimit: 1, GlobalLimit: 1,
		CallerWindow: time.Hour, GlobalWindow: time.Hour,
	})
	stream := make([]byte, 0, 72)
	stream = append(stream, make([]byte, 24)...)
	stream = append(stream, make([]byte, 24)...)
	stream = append(stream, bytes.Repeat([]byte{1}, 24)...)
	store.random = bytes.NewReader(stream)
	if err := store.limitIssue(context.Background(), "replayed-caller"); err != nil {
		t.Fatalf("first limitIssue: %v", err)
	}
	if err := store.limitIssue(context.Background(), "replayed-caller"); err != nil {
		t.Fatalf("same-operation replay at limit: %v", err)
	}
	if err := store.limitIssue(context.Background(), "replayed-caller"); !errors.Is(err, errQuota) {
		t.Fatalf("different operation at limit = %v, want quota", err)
	}

	partialStore, fixture := newTestStore(t, Options{
		CallerLimit: 2, GlobalLimit: 2,
		CallerWindow: time.Hour, GlobalWindow: time.Hour,
	})
	partialStore.random = bytes.NewReader(make([]byte, 24))
	if err := partialStore.limitIssue(context.Background(), "partial-caller"); err != nil {
		t.Fatalf("seed partial limitIssue: %v", err)
	}
	keys := fixture.server.Keys()
	if len(keys) != 2 {
		t.Fatalf("rate keys = %v, want two", keys)
	}
	testClient := redis.NewClient(&redis.Options{Addr: fixture.server.Addr()})
	t.Cleanup(func() { _ = testClient.Close() })
	operationID := base64.RawURLEncoding.EncodeToString(make([]byte, 24))
	if removed, err := testClient.ZRem(context.Background(), keys[0], operationID).Result(); err != nil || removed != 1 {
		t.Fatalf("remove one replay member = %d, %v", removed, err)
	}
	partialStore.random = bytes.NewReader(make([]byte, 24))
	if err := partialStore.limitIssue(context.Background(), "partial-caller"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("partial replay = %v, want unavailable", err)
	}
}

func TestVerifyExactlyOnceAndBoundedAttempts(t *testing.T) {
	store, _ := newTestStore(t, Options{MaxAttempts: 5, CallerLimit: 100, GlobalLimit: 100})
	ctx := context.Background()
	reservation := mustCommitted(t, store, "exact@example.test")
	var successes atomic.Int64
	var failures atomic.Int64
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			outcome, err := store.Verify(ctx, VerifyRequest{Subject: "exact@example.test", Purpose: testPurpose, Code: reservation.Code()})
			if err != nil {
				failures.Add(1)
				return
			}
			if outcome == VerifyVerified {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()
	if failures.Load() != 0 || successes.Load() != 1 {
		t.Fatalf("parallel Verify errors=%d successes=%d; want 0,1", failures.Load(), successes.Load())
	}

	locked := mustCommitted(t, store, "locked@example.test")
	wrong := "000000"
	if wrong == locked.Code() {
		wrong = "000001"
	}
	for range 5 {
		if outcome, err := store.Verify(ctx, VerifyRequest{Subject: "locked@example.test", Purpose: testPurpose, Code: wrong}); err != nil || outcome != VerifyRejected {
			t.Fatalf("wrong Verify = %v, %v", outcome, err)
		}
	}
	if outcome, err := store.Verify(ctx, VerifyRequest{Subject: "locked@example.test", Purpose: testPurpose, Code: locked.Code()}); err != nil || outcome != VerifyRejected {
		t.Fatalf("locked correct Verify = %v, %v; want rejected,nil", outcome, err)
	}
}

func TestParallelRateLimitAndCodeExpiry(t *testing.T) {
	store, _ := newTestStore(t, Options{
		CallerLimit: 10, GlobalLimit: 1000,
		CallerWindow: time.Hour, GlobalWindow: time.Hour,
		CodeTTL: 5 * time.Second, PendingTTL: time.Second,
		Cooldown: time.Millisecond, MaxIssues: 100,
	})
	var reserved atomic.Int64
	var quota atomic.Int64
	var failures atomic.Int64
	var wait sync.WaitGroup
	for index := range 100 {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			outcome, err := store.BeginIssue(context.Background(), BeginRequest{
				Caller: "same-caller", Subject: fmt.Sprintf("parallel-%d@example.test", index), Purpose: testPurpose,
			})
			if err != nil {
				failures.Add(1)
				return
			}
			switch outcome.State() {
			case BeginReserved:
				reservation, ok := outcome.Reservation()
				if !ok || store.Commit(context.Background(), reservation) != nil {
					failures.Add(1)
					return
				}
				reserved.Add(1)
			case BeginQuota:
				quota.Add(1)
			default:
				failures.Add(1)
			}
		}(index)
	}
	wait.Wait()
	if failures.Load() != 0 || reserved.Load() != 10 || quota.Load() != 90 {
		t.Fatalf("parallel rate failures=%d reserved=%d quota=%d; want 0,10,90", failures.Load(), reserved.Load(), quota.Load())
	}
	expiryStore, expiryFixture := newTestStore(t, Options{
		CodeTTL: 5 * time.Second, PendingTTL: time.Second,
		CallerLimit: 100, GlobalLimit: 100,
	})
	reservation := mustCommitted(t, expiryStore, "expiry@example.test")
	expiryFixture.advance(5*time.Second + time.Millisecond)
	if outcome, err := expiryStore.Verify(context.Background(), VerifyRequest{Subject: "expiry@example.test", Purpose: testPurpose, Code: reservation.Code()}); err != nil || outcome != VerifyRejected {
		t.Fatalf("expired Verify = %v, %v", outcome, err)
	}
}

func TestPepperRotationAndAntiEnumeration(t *testing.T) {
	store, fixture := newTestStore(t, Options{CallerLimit: 100, GlobalLimit: 100})
	old := mustCommitted(t, store, "rotate@example.test")
	rotated, err := NewRedis(fixture.scope, testOptions(Options{Peppers: []Pepper{
		{Version: "v2", Secret: testPepperV2},
		{Version: "v1", Secret: testPepperV1},
	}, CallerLimit: 100, GlobalLimit: 100}))
	if err != nil {
		t.Fatalf("NewRedis rotated: %v", err)
	}
	if outcome, verifyErr := rotated.Verify(context.Background(), VerifyRequest{Subject: "rotate@example.test", Purpose: testPurpose, Code: old.Code()}); verifyErr != nil || outcome != VerifyVerified {
		t.Fatalf("previous pepper Verify = %v, %v", outcome, verifyErr)
	}
	for _, candidate := range []struct {
		subject string
		purpose Purpose
		code    string
	}{
		{"missing@example.test", testPurpose, "123456"},
		{"", testPurpose, "123456"},
		{"missing@example.test", Purpose("INVALID PURPOSE"), "123456"},
		{"missing@example.test", testPurpose, "12345x"},
	} {
		if outcome, verifyErr := rotated.Verify(context.Background(), VerifyRequest{Subject: candidate.subject, Purpose: candidate.purpose, Code: candidate.code}); verifyErr != nil || outcome != VerifyRejected {
			t.Fatalf("anti-enumerating Verify(%q) = %v, %v", candidate.subject, outcome, verifyErr)
		}
	}
}

func TestValidVerifyPathsUseEqualFixedScriptCount(t *testing.T) {
	store, _ := newTestStore(t, Options{CallerLimit: 100, GlobalLimit: 100})
	ctx := context.Background()
	counter := &countingVerifyExecutor{next: store.verifier}
	store.verifier = counter
	measure := func(request VerifyRequest) (VerifyOutcome, error, int, int) {
		beforeReads, beforeCompletes := counter.reads, counter.completes
		outcome, err := store.Verify(ctx, request)
		return outcome, err, counter.reads - beforeReads, counter.completes - beforeCompletes
	}
	missingOutcome, missingErr, missingReads, missingCompletes := measure(VerifyRequest{
		Subject: "missing@example.test", Purpose: testPurpose, Code: "123456",
	})
	wrongReservation := mustCommitted(t, store, "wrong-path@example.test")
	wrongCode := "000000"
	if wrongCode == wrongReservation.Code() {
		wrongCode = "000001"
	}
	wrongOutcome, wrongErr, wrongReads, wrongCompletes := measure(VerifyRequest{
		Subject: "wrong-path@example.test", Purpose: testPurpose, Code: wrongCode,
	})
	successReservation := mustCommitted(t, store, "success-path@example.test")
	successOutcome, successErr, successReads, successCompletes := measure(VerifyRequest{
		Subject: "success-path@example.test", Purpose: testPurpose, Code: successReservation.Code(),
	})
	if missingErr != nil || wrongErr != nil || successErr != nil ||
		missingOutcome != VerifyRejected || wrongOutcome != VerifyRejected || successOutcome != VerifyVerified {
		t.Fatalf("Verify outcomes missing=%v/%v wrong=%v/%v success=%v/%v",
			missingOutcome, missingErr, wrongOutcome, wrongErr, successOutcome, successErr)
	}
	if missingReads != 1 || wrongReads != 1 || successReads != 1 ||
		missingCompletes != 1 || wrongCompletes != 1 || successCompletes != 1 {
		t.Fatalf("Verify fixed scripts missing=%d/%d wrong=%d/%d success=%d/%d; want one read and one complete each",
			missingReads, missingCompletes, wrongReads, wrongCompletes, successReads, successCompletes)
	}
}

func TestConstructorIsPureClusterPortableAndOwnsNoClose(t *testing.T) {
	server := miniredis.RunT(t)
	profile := normalizedTestProfile(t, "challenge-cluster", runtimeconfig.RedisCluster, []string{server.Addr()})
	resource, err := redisresource.Build(profile)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	scope, err := resource.Scope("challenge")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	store, err := NewRedis(scope, testOptions(Options{}))
	if err != nil || store == nil {
		t.Fatalf("NewRedis = %#v, %v", store, err)
	}
	if keys := server.Keys(); len(keys) != 0 {
		t.Fatalf("constructor performed Redis I/O: %v", keys)
	}
	if _, exists := reflect.TypeOf(store).MethodByName("Close"); exists {
		t.Fatal("challenge capability unexpectedly owns Close")
	}
	if err = store.Ready(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Ready before resource Start = %v, want unavailable", err)
	}
}

func TestConstructorCopiesSecrets(t *testing.T) {
	_, fixture := newTestStore(t, Options{})
	options := testOptions(Options{})
	store, err := NewRedis(fixture.scope, options)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	originalSubject := append([]byte(nil), store.subjectKey...)
	originalPepper := append([]byte(nil), store.peppers["v1"]...)
	options.SubjectKey[0] ^= 0xff
	options.Peppers[0].Secret[0] ^= 0xff
	if !reflect.DeepEqual(store.subjectKey, originalSubject) || !reflect.DeepEqual(store.peppers["v1"], originalPepper) {
		t.Fatal("NewRedis retained mutable caller secret slices")
	}
}

func TestOutageContextRandomAndConfigurationFailClosed(t *testing.T) {
	store, fixture := newTestStore(t, Options{})
	fixture.server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := store.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Ready outage = %v", err)
	}
	if outcome, err := store.BeginIssue(ctx, BeginRequest{Caller: "caller", Subject: testSubject, Purpose: testPurpose}); outcome.State() != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("BeginIssue outage = %#v, %v", outcome, err)
	}

	fresh, _ := newTestStore(t, Options{})
	fresh.random = errorReader{}
	if outcome, err := fresh.BeginIssue(context.Background(), BeginRequest{Caller: "caller", Subject: testSubject, Purpose: testPurpose}); outcome.State() != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("random failure = %#v, %v", outcome, err)
	}

	base := testOptions(Options{})
	invalid := []Options{
		{SubjectKey: make([]byte, 32), Peppers: base.Peppers},
		{SubjectKey: base.SubjectKey, Peppers: []Pepper{{Version: "v1", Secret: make([]byte, 32)}}},
		{SubjectKey: base.SubjectKey, Peppers: []Pepper{{Version: "v1", Secret: base.SubjectKey}}},
		{SubjectKey: base.SubjectKey, Peppers: base.Peppers, CodeTTL: time.Microsecond},
	}
	for index, options := range invalid {
		if result, err := NewRedis(fresh.scope, options); result != nil || !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid[%d] = %#v, %v", index, result, err)
		}
	}
}

func mustCommitted(t *testing.T, store *Redis, subject string) *Reservation {
	t.Helper()
	outcome, err := store.BeginIssue(context.Background(), BeginRequest{Caller: "helper-" + subject, Subject: subject, Purpose: testPurpose})
	if err != nil {
		t.Fatalf("BeginIssue(%q): %v", subject, err)
	}
	reservation, ok := outcome.Reservation()
	if !ok {
		t.Fatalf("BeginIssue(%q) outcome = %#v", subject, outcome)
	}
	if err = store.Commit(context.Background(), reservation); err != nil {
		t.Fatalf("Commit(%q): %v", subject, err)
	}
	return reservation
}

type testFixture struct {
	server   *miniredis.Miniredis
	resource *redisresource.Resource
	scope    *redisresource.Scope
	now      time.Time
}

func newTestStore(t *testing.T, overrides Options) (*Redis, *testFixture) {
	t.Helper()
	server := miniredis.RunT(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	server.SetTime(now)
	profile := normalizedTestProfile(t, "challenge-cache", runtimeconfig.RedisStandalone, []string{server.Addr()})
	resource, err := redisresource.Build(profile)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	scope, err := resource.Scope("challenge")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	if err = resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	store, err := NewRedis(scope, testOptions(overrides))
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	return store, &testFixture{server: server, resource: resource, scope: scope, now: now}
}

func (f *testFixture) advance(duration time.Duration) {
	f.server.FastForward(duration)
	f.now = f.now.Add(duration)
	f.server.SetTime(f.now)
}

func normalizedTestProfile(t *testing.T, name string, mode runtimeconfig.RedisMode, endpoints []string) runtimeconfig.ResourceProfile {
	t.Helper()
	redisConfig := &runtimeconfig.RedisConfig{
		Mode: mode,
		Credentials: runtimeconfig.RedisCredentialsConfig{
			Kind:      runtimeconfig.RedisCredentialsAnonymous,
			Anonymous: &runtimeconfig.RedisAnonymousCredentialsConfig{},
		},
	}
	if mode == runtimeconfig.RedisCluster {
		redisConfig.Cluster = &runtimeconfig.RedisClusterConfig{Endpoints: endpoints}
	} else {
		redisConfig.Standalone = &runtimeconfig.RedisStandaloneConfig{Endpoint: endpoints[0]}
	}
	configuration := runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
		name: {Provider: runtimeconfig.ProviderConfig{Kind: runtimeconfig.ProviderRedis, Redis: redisConfig}},
	}}
	snapshot, err := configuration.Normalize(context.Background(), runtimeconfig.SecretResolverFunc(func(context.Context, runtimeconfig.SecretRef) (string, error) {
		return "", errors.New("unexpected secret resolution")
	}))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	profile, ok := snapshot.Resource(name)
	if !ok {
		t.Fatal("normalized profile missing")
	}
	return profile
}

func testOptions(overrides Options) Options {
	result := Options{
		SubjectKey: append([]byte(nil), testSubjectKey...),
		Peppers:    []Pepper{{Version: "v1", Secret: append([]byte(nil), testPepperV1...)}},
		CodeTTL:    5 * time.Minute, PendingTTL: 5 * time.Second,
		Cooldown: time.Minute, QuotaWindow: time.Hour,
		MaxIssues: 10, MaxAttempts: 5, IdempotencyTTL: 2 * time.Minute,
		CallerWindow: 10 * time.Minute, CallerLimit: 10,
		GlobalWindow: 10 * time.Minute, GlobalLimit: 1000,
	}
	if overrides.SubjectKey != nil {
		result.SubjectKey = overrides.SubjectKey
	}
	if overrides.Peppers != nil {
		result.Peppers = overrides.Peppers
	}
	if overrides.CodeTTL != 0 {
		result.CodeTTL = overrides.CodeTTL
	}
	if overrides.PendingTTL != 0 {
		result.PendingTTL = overrides.PendingTTL
	}
	if overrides.Cooldown != 0 {
		result.Cooldown = overrides.Cooldown
	}
	if overrides.QuotaWindow != 0 {
		result.QuotaWindow = overrides.QuotaWindow
	}
	if overrides.MaxIssues != 0 {
		result.MaxIssues = overrides.MaxIssues
	}
	if overrides.MaxAttempts != 0 {
		result.MaxAttempts = overrides.MaxAttempts
	}
	if overrides.IdempotencyTTL != 0 {
		result.IdempotencyTTL = overrides.IdempotencyTTL
	}
	if overrides.CallerWindow != 0 {
		result.CallerWindow = overrides.CallerWindow
	}
	if overrides.CallerLimit != 0 {
		result.CallerLimit = overrides.CallerLimit
	}
	if overrides.GlobalWindow != 0 {
		result.GlobalWindow = overrides.GlobalWindow
	}
	if overrides.GlobalLimit != 0 {
		result.GlobalLimit = overrides.GlobalLimit
	}
	return result
}

func activeStateKey(t *testing.T, server *miniredis.Miniredis) string {
	t.Helper()
	for _, key := range server.Keys() {
		if server.Type(key) == "hash" && server.HGet(key, "active_id") != "" {
			return key
		}
	}
	t.Fatal("active challenge state key missing")
	return ""
}

func setNextIssueCode(store *Redis, avoid string) string {
	target := byte(1)
	if avoid == "000001" {
		target = 2
	}
	stream := make([]byte, 0, 51)
	stream = append(stream, bytes.Repeat([]byte{0x71}, 24)...)
	stream = append(stream, 0, 0, target)
	stream = append(stream, bytes.Repeat([]byte{0x72}, 24)...)
	store.random = bytes.NewReader(stream)
	return fmt.Sprintf("%06d", target)
}

func assertRedisMaterialRedacted(t *testing.T, server *miniredis.Miniredis, subject, code string) {
	t.Helper()
	for _, key := range server.Keys() {
		if strings.Contains(strings.ToLower(key), strings.ToLower(subject)) || strings.Contains(key, code) {
			t.Fatalf("Redis key leaked material: %q", key)
		}
		if server.Type(key) == "hash" {
			fields, err := server.HKeys(key)
			if err != nil {
				t.Fatalf("HKeys(%q): %v", key, err)
			}
			for _, field := range fields {
				value := server.HGet(key, field)
				joined := field + "=" + value
				if strings.Contains(strings.ToLower(joined), strings.ToLower(subject)) || strings.Contains(joined, code) {
					t.Fatalf("Redis hash leaked material: %q", joined)
				}
			}
		}
	}
}

func assertNoMaterial(t *testing.T, formatted string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		if value != "" && strings.Contains(formatted, value) {
			t.Fatalf("formatted value leaked %q: %q", value, formatted)
		}
	}
}

func renderChallengeLogs(value any) (string, string) {
	options := &slog.HandlerOptions{ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
		if attribute.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return attribute
	}}
	var textLog bytes.Buffer
	var jsonLog bytes.Buffer
	slog.New(slog.NewTextHandler(&textLog, options)).Info("canary", "value", value)
	slog.New(slog.NewJSONHandler(&jsonLog, options)).Info("canary", "value", value)
	return textLog.String(), jsonLog.String()
}

func challengeErrorTree(err error, depth int) []error {
	if err == nil || depth > 16 {
		return nil
	}
	result := []error{err}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range many.Unwrap() {
			result = append(result, challengeErrorTree(child, depth+1)...)
		}
		return result
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		result = append(result, challengeErrorTree(one.Unwrap(), depth+1)...)
	}
	return result
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type countingVerifyExecutor struct {
	next      verifyExecutor
	reads     int
	completes int
}

func (e *countingVerifyExecutor) Read(
	ctx context.Context,
	store *Redis,
	group redisbridge.AtomicGroup,
	stateKey redisbridge.Key,
) ([]string, error) {
	e.reads++
	return e.next.Read(ctx, store, group, stateKey)
}

func (e *countingVerifyExecutor) Complete(
	ctx context.Context,
	store *Redis,
	group redisbridge.AtomicGroup,
	stateKey, opsKey redisbridge.Key,
	expectedID, expectedDigest string,
	matched bool,
) (string, error) {
	e.completes++
	return e.next.Complete(ctx, store, group, stateKey, opsKey, expectedID, expectedDigest, matched)
}
