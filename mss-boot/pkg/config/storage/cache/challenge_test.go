package cache

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testChallengeSubject = "Person+Challenge@Example.COM"
	testLoginPurpose     = ChallengePurpose("email-login")
	testRegisterPurpose  = ChallengePurpose("email-registration")
)

var (
	testChallengeSubjectKey = []byte("subject-locator-key-material-32-bytes-minimum")
	testChallengePepperV1   = []byte("challenge-pepper-version-one-32-bytes-minimum")
	testChallengePepperV2   = []byte("challenge-pepper-version-two-32-bytes-minimum")
)

func TestChallengeIssueSendCommitAbortCAS(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{MaxIssues: 2})
	ctx := context.Background()
	oldCode := mustIssueChallenge(t, store, testChallengeSubject, testLoginPurpose)

	advanceChallengeTime(t, store.client, server, time.Minute+time.Millisecond)
	var unsentCode string
	err := store.Issue(ctx, "caller-a", testChallengeSubject, testLoginPurpose, func(_ context.Context, code string) error {
		unsentCode = code
		return errors.New("transport detail must not escape")
	})
	if !errors.Is(err, ErrChallengeDelivery) {
		t.Fatalf("failed delivery error = %v, want ErrChallengeDelivery", err)
	}
	if strings.Contains(err.Error(), "transport") {
		t.Fatalf("failed delivery leaked transport error: %v", err)
	}
	if unsentCode != oldCode {
		if ok, verifyErr := store.VerifyChallenge(ctx, testChallengeSubject, testLoginPurpose, unsentCode); verifyErr != nil || ok {
			t.Fatalf("unsent code verification = %v, %v; want false, nil", ok, verifyErr)
		}
	}
	if ok, verifyErr := store.VerifyChallenge(ctx, testChallengeSubject, testLoginPurpose, oldCode); verifyErr != nil || !ok {
		t.Fatalf("prior active code verification = %v, %v; want true, nil", ok, verifyErr)
	}
	if err = store.Issue(ctx, "caller-a", testChallengeSubject, testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengeQuota) {
		t.Fatalf("third issue error = %v, want ErrChallengeQuota", err)
	}

	// A completion from an expired delivery reservation cannot affect the next
	// reservation, even when it arrives after the newer Begin.
	const secondSubject = "second@example.com"
	first, err := store.beginIssue(ctx, secondSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("first BeginIssue: %v", err)
	}
	advanceChallengeTime(t, store.client, server, 5*time.Second+time.Millisecond)
	second, err := store.beginIssue(ctx, secondSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("reclaimed BeginIssue: %v", err)
	}
	if err = store.commitIssue(ctx, first); !errors.Is(err, ErrChallengeStale) {
		t.Fatalf("stale commit error = %v, want ErrChallengeStale", err)
	}
	if err = store.abortIssue(ctx, first); !errors.Is(err, ErrChallengeStale) {
		t.Fatalf("stale abort error = %v, want ErrChallengeStale", err)
	}
	if err = store.commitIssue(ctx, second); err != nil {
		t.Fatalf("new reservation commit: %v", err)
	}
	if ok, verifyErr := store.VerifyChallenge(ctx, secondSubject, testLoginPurpose, second.code); verifyErr != nil || !ok {
		t.Fatalf("new reservation verification = %v, %v; want true, nil", ok, verifyErr)
	}
}

func TestChallengeConcurrentIssueSinglePending(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{MaxIssues: 200})
	ctx := context.Background()

	// Exercise the public orchestration boundary: while one delivery callback
	// owns the pending lease, a second Issue cannot send another code.
	deliveryEntered := make(chan struct{})
	releaseDelivery := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- store.Issue(ctx, "caller-race", "public-race@example.com", testLoginPurpose, func(context.Context, string) error {
			close(deliveryEntered)
			<-releaseDelivery
			return nil
		})
	}()
	<-deliveryEntered
	if err := store.Issue(ctx, "caller-race", "public-race@example.com", testLoginPurpose, func(context.Context, string) error {
		return nil
	}); !errors.Is(err, ErrChallengePending) {
		t.Fatalf("second public Issue error = %v, want ErrChallengePending", err)
	}
	close(releaseDelivery)
	if err := <-firstResult; err != nil {
		t.Fatalf("first public Issue: %v", err)
	}

	advanceChallengeTime(t, store.client, server, time.Minute+time.Millisecond)
	const subject = "atomic-race@example.com"
	start := make(chan struct{})
	results := make(chan error, 100)
	issues := make(chan *challengeIssue, 100)
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			issue, err := store.beginIssue(ctx, subject, testLoginPurpose)
			if issue != nil {
				issues <- issue
			}
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(issues)

	successes := 0
	pendings := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrChallengePending):
			pendings++
		default:
			t.Fatalf("concurrent BeginIssue error = %v", err)
		}
	}
	if successes != 1 || pendings != 99 || len(issues) != 1 {
		t.Fatalf("concurrent BeginIssue successes=%d pending=%d issues=%d; want 1, 99, 1", successes, pendings, len(issues))
	}
	winning := <-issues
	if err := store.commitIssue(ctx, winning); err != nil {
		t.Fatalf("commit winning issue: %v", err)
	}
	if err := store.Issue(ctx, "caller-race", subject, testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengeCooldown) {
		t.Fatalf("post-commit Issue error = %v, want ErrChallengeCooldown", err)
	}
}

func TestChallengeCooldownActiveExpiryAndCanceledDelivery(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{})
	ctx := context.Background()
	code := mustIssueChallenge(t, store, "expiry@example.com", testLoginPurpose)
	if err := store.Issue(ctx, "caller-expiry", "expiry@example.com", testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengeCooldown) {
		t.Fatalf("cooldown Issue error = %v, want ErrChallengeCooldown", err)
	}
	advanceChallengeTime(t, store.client, server, 5*time.Minute+time.Millisecond)
	if ok, err := store.VerifyChallenge(ctx, "expiry@example.com", testLoginPurpose, code); err != nil || ok {
		t.Fatalf("expired verification = %v, %v; want false, nil", ok, err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	err := store.Issue(canceled, "caller-canceled", "canceled@example.com", testLoginPurpose, func(context.Context, string) error {
		cancel()
		return errors.New("delivery canceled")
	})
	if !errors.Is(err, ErrChallengeUnavailable) {
		t.Fatalf("canceled delivery error = %v, want ErrChallengeUnavailable", err)
	}
	if err = store.Issue(ctx, "caller-canceled", "canceled@example.com", testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengePending) {
		t.Fatalf("pre-reclaim Issue error = %v, want ErrChallengePending", err)
	}
	advanceChallengeTime(t, store.client, server, 5*time.Second+time.Millisecond)
	if err = store.Issue(ctx, "caller-canceled", "canceled@example.com", testLoginPurpose, func(context.Context, string) error { return nil }); err != nil {
		t.Fatalf("post-lease recovery Issue: %v", err)
	}
}

func TestChallengeCallerAndGlobalIssueLimitsAcrossSubjects(t *testing.T) {
	store, _ := newTestChallengeStore(t, ChallengeOptions{
		CallerLimit:  2,
		GlobalLimit:  3,
		CallerWindow: time.Hour,
		GlobalWindow: time.Hour,
	})
	ctx := context.Background()
	issue := func(caller, subject string) error {
		return store.Issue(ctx, caller, subject, testLoginPurpose, func(context.Context, string) error { return nil })
	}
	if err := issue("caller-one", "one@example.com"); err != nil {
		t.Fatalf("caller-one first Issue: %v", err)
	}
	if err := issue("caller-one", "two@example.com"); err != nil {
		t.Fatalf("caller-one second Issue: %v", err)
	}
	if err := issue("caller-one", "rotated@example.com"); !errors.Is(err, ErrChallengeQuota) {
		t.Fatalf("rotated-subject Issue error = %v, want quota", err)
	}
	if err := issue("caller-two", "three@example.com"); err != nil {
		t.Fatalf("caller-two Issue: %v", err)
	}
	if err := issue("caller-three", "four@example.com"); !errors.Is(err, ErrChallengeQuota) {
		t.Fatalf("global-limit Issue error = %v, want quota", err)
	}
}

func TestChallengeVerifyExactlyOnce(t *testing.T) {
	store, _ := newTestChallengeStore(t, ChallengeOptions{MaxAttempts: 5})
	ctx := context.Background()
	code := mustIssueChallenge(t, store, testChallengeSubject, testLoginPurpose)

	var successes atomic.Int64
	var failures atomic.Int64
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ok, err := store.VerifyChallenge(ctx, testChallengeSubject, testLoginPurpose, code)
			if err != nil {
				failures.Add(1)
				return
			}
			if ok {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()
	if failures.Load() != 0 || successes.Load() != 1 {
		t.Fatalf("parallel Verify errors=%d successes=%d, want 0 and 1", failures.Load(), successes.Load())
	}

	const boundedSubject = "bounded@example.com"
	boundedCode := mustIssueChallenge(t, store, boundedSubject, testLoginPurpose)
	for attempt := 0; attempt < 4; attempt++ {
		ok, err := store.VerifyChallenge(ctx, boundedSubject, testLoginPurpose, differentChallengeCode(boundedCode))
		if err != nil || ok {
			t.Fatalf("wrong attempt %d = %v, %v; want false, nil", attempt, ok, err)
		}
	}
	if ok, err := store.VerifyChallenge(ctx, boundedSubject, testLoginPurpose, boundedCode); err != nil || !ok {
		t.Fatalf("correct code before limit = %v, %v; want true, nil", ok, err)
	}

	const lockedSubject = "locked@example.com"
	lockedCode := mustIssueChallenge(t, store, lockedSubject, testLoginPurpose)
	for range 5 {
		_, _ = store.VerifyChallenge(ctx, lockedSubject, testLoginPurpose, differentChallengeCode(lockedCode))
	}
	if ok, err := store.VerifyChallenge(ctx, lockedSubject, testLoginPurpose, lockedCode); err != nil || ok {
		t.Fatalf("correct code after attempt limit = %v, %v; want false, nil", ok, err)
	}
}

func TestChallengeRedisOutageFailsClosed(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{})
	server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := store.Ready(ctx); !errors.Is(err, ErrChallengeUnavailable) {
		t.Fatalf("Ready error = %v, want ErrChallengeUnavailable", err)
	}
	if err := store.Issue(ctx, "caller-outage", testChallengeSubject, testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengeUnavailable) {
		t.Fatalf("Issue error = %v, want ErrChallengeUnavailable", err)
	}
	if ok, err := store.VerifyChallenge(ctx, testChallengeSubject, testLoginPurpose, "123456"); ok || !errors.Is(err, ErrChallengeUnavailable) {
		t.Fatalf("Verify during outage = %v, %v; want false, ErrChallengeUnavailable", ok, err)
	}

	client, freshServer := newMiniredisClient(t)
	failingStore, err := NewRedisChallengeStore(client, testChallengeOptions(ChallengeOptions{}))
	if err != nil {
		t.Fatalf("NewRedisChallengeStore: %v", err)
	}
	failingStore.random = errorReader{}
	if err = failingStore.Issue(context.Background(), "caller-random", testChallengeSubject, testLoginPurpose, func(context.Context, string) error { return nil }); !errors.Is(err, ErrChallengeUnavailable) {
		t.Fatalf("crypto failure error = %v, want ErrChallengeUnavailable", err)
	}
	if keys := freshServer.Keys(); len(keys) != 0 {
		t.Fatalf("crypto failure wrote Redis keys: %v", keys)
	}
}

func TestChallengePendingLeaseCrashRecovery(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{})
	ctx := context.Background()
	mustIssueChallenge(t, store, testChallengeSubject, testLoginPurpose)
	stateKey, _, _, _, err := store.keys(testChallengeSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("challenge keys: %v", err)
	}
	originalExpiry := store.client.HGet(ctx, stateKey, "active_expires_at").Val()
	advanceChallengeTime(t, store.client, server, time.Minute+time.Millisecond)

	orphan, err := store.beginIssue(ctx, testChallengeSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("orphan BeginIssue: %v", err)
	}
	if got := store.client.HGet(ctx, stateKey, "active_expires_at").Val(); got != originalExpiry {
		t.Fatalf("BeginIssue changed prior active expiry: got %q want %q", got, originalExpiry)
	}
	advanceChallengeTime(t, store.client, server, 5*time.Second+time.Millisecond)
	replacement, err := store.beginIssue(ctx, testChallengeSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("replacement BeginIssue: %v", err)
	}
	if got := store.client.HGet(ctx, stateKey, "active_expires_at").Val(); got != originalExpiry {
		t.Fatalf("pending reclaim changed prior active expiry: got %q want %q", got, originalExpiry)
	}
	if err = store.commitIssue(ctx, orphan); !errors.Is(err, ErrChallengeStale) {
		t.Fatalf("orphan commit error = %v, want ErrChallengeStale", err)
	}
	if err = store.abortIssue(ctx, orphan); !errors.Is(err, ErrChallengeStale) {
		t.Fatalf("orphan abort error = %v, want ErrChallengeStale", err)
	}
	if err = store.commitIssue(ctx, replacement); err != nil {
		t.Fatalf("replacement commit: %v", err)
	}
}

func TestChallengeGeneratedKeysShareRedisHashTag(t *testing.T) {
	store, _ := newTestChallengeStore(t, ChallengeOptions{})
	stateKey, quotaKey, opsKey, _, err := store.keys(testChallengeSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("challenge keys: %v", err)
	}
	if err = ensureSameRedisHashTag(stateKey, quotaKey, opsKey); err != nil {
		t.Fatalf("challenge keys do not share a Redis hash tag: %v", err)
	}
}

func TestChallengeCrossSlotPreflightRejected(t *testing.T) {
	if err := ensureSameRedisHashTag("challenge:{one}:state", "challenge:{two}:quota"); err == nil {
		t.Fatal("cross-slot challenge keys were accepted")
	}
	if err := ensureSameRedisHashTag("challenge:state", "challenge:quota"); err == nil {
		t.Fatal("untagged challenge keys were accepted")
	}
}

func TestChallengeRedisClusterFailsClosed(t *testing.T) {
	client := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{"127.0.0.1:1"}})
	t.Cleanup(func() { _ = client.Close() })
	for name, candidate := range map[string]redis.UniversalClient{
		"raw":     client,
		"wrapped": &Redis{UniversalClient: client},
	} {
		t.Run(name, func(t *testing.T) {
			store, err := NewRedisChallengeStore(candidate, testChallengeOptions(ChallengeOptions{}))
			if store != nil || !errors.Is(err, ErrChallengeUnavailable) {
				t.Fatalf("Cluster constructor = %#v, %v; want nil, unavailable", store, err)
			}
		})
	}
}

func TestChallengeVerifierPepperAndRotation(t *testing.T) {
	client, server := newMiniredisClient(t)
	v1, err := NewRedisChallengeStore(client, testChallengeOptions(ChallengeOptions{}))
	if err != nil {
		t.Fatalf("new v1 store: %v", err)
	}
	oldCode := mustIssueChallenge(t, v1, testChallengeSubject, testLoginPurpose)
	stateKey, _, _, _, err := v1.keys(testChallengeSubject, testLoginPurpose)
	if err != nil {
		t.Fatalf("challenge keys: %v", err)
	}
	storedDigest := client.HGet(context.Background(), stateKey, "active_digest").Val()
	if storedDigest == "" || storedDigest == oldCode {
		t.Fatalf("stored verifier = %q; want non-plaintext digest", storedDigest)
	}

	rotatedOptions := testChallengeOptions(ChallengeOptions{})
	rotatedOptions.Peppers = []ChallengePepper{
		{Version: "v2", Secret: testChallengePepperV2},
		{Version: "v1", Secret: testChallengePepperV1},
	}
	rotated, err := NewRedisChallengeStore(client, rotatedOptions)
	if err != nil {
		t.Fatalf("new rotated store: %v", err)
	}
	if ok, verifyErr := rotated.VerifyChallenge(context.Background(), testChallengeSubject, testLoginPurpose, oldCode); verifyErr != nil || !ok {
		t.Fatalf("previous-pepper verification = %v, %v; want true, nil", ok, verifyErr)
	}

	advanceChallengeTime(t, client, server, time.Minute+time.Millisecond)
	newCode := mustIssueChallenge(t, rotated, testChallengeSubject, testLoginPurpose)
	if ok, verifyErr := v1.VerifyChallenge(context.Background(), testChallengeSubject, testLoginPurpose, newCode); ok || !errors.Is(verifyErr, ErrChallengeUnavailable) {
		t.Fatalf("retired reader verification = %v, %v; want false, unavailable", ok, verifyErr)
	}
	if ok, verifyErr := rotated.VerifyChallenge(context.Background(), testChallengeSubject, testLoginPurpose, newCode); verifyErr != nil || !ok {
		t.Fatalf("current-pepper verification = %v, %v; want true, nil", ok, verifyErr)
	}
}

func TestChallengeVerifierUsesConstantTimeCompare(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filepath.Join(filepath.Dir(currentFile), "challenge.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse challenge implementation: %v", err)
	}
	var helperOK bool
	var liveCallOK bool
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.FuncDecl)
		if !ok || declaration.Body == nil {
			return true
		}
		switch declaration.Name.Name {
		case "constantTimeDigestEqual":
			if len(declaration.Body.List) != 1 {
				return true
			}
			statement, ok := declaration.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(statement.Results) != 1 {
				return true
			}
			call, ok := statement.Results[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			packageName, packageOK := selector.X.(*ast.Ident)
			helperOK = ok && packageOK && packageName.Name == "hmac" && selector.Sel.Name == "Equal"
		case "VerifyChallenge":
			ast.Inspect(declaration.Body, func(child ast.Node) bool {
				assignment, ok := child.(*ast.AssignStmt)
				if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
					return true
				}
				left, leftOK := assignment.Lhs[0].(*ast.Ident)
				call, callOK := assignment.Rhs[0].(*ast.CallExpr)
				function, functionOK := call.Fun.(*ast.Ident)
				if leftOK && callOK && functionOK && left.Name == "matched" && function.Name == "constantTimeDigestEqual" && len(call.Args) == 2 {
					first, firstOK := call.Args[0].(*ast.Ident)
					second, secondOK := call.Args[1].(*ast.Ident)
					liveCallOK = firstOK && secondOK && first.Name == "storedDigest" && second.Name == "candidate"
				}
				return true
			})
		}
		return true
	})
	if !helperOK || !liveCallOK {
		t.Fatalf("constant-time verifier AST helper=%v liveCall=%v; want both true", helperOK, liveCallOK)
	}
}

func TestChallengeConfigurationRejectsWeakSecretsAndSubMillisecondDurations(t *testing.T) {
	client, _ := newMiniredisClient(t)
	tests := []struct {
		name   string
		mutate func(*ChallengeOptions)
	}{
		{name: "weak locator", mutate: func(options *ChallengeOptions) { options.SubjectKey = make([]byte, 32) }},
		{name: "weak pepper", mutate: func(options *ChallengeOptions) { options.Peppers[0].Secret = make([]byte, 32) }},
		{name: "same locator and pepper", mutate: func(options *ChallengeOptions) {
			options.Peppers[0].Secret = append([]byte(nil), options.SubjectKey...)
		}},
		{name: "code ttl", mutate: func(options *ChallengeOptions) { options.CodeTTL = time.Microsecond }},
		{name: "pending ttl", mutate: func(options *ChallengeOptions) { options.PendingTTL = time.Microsecond }},
		{name: "cooldown", mutate: func(options *ChallengeOptions) { options.Cooldown = time.Microsecond }},
		{name: "quota window", mutate: func(options *ChallengeOptions) { options.QuotaWindow = time.Microsecond }},
		{name: "idempotency ttl", mutate: func(options *ChallengeOptions) { options.IdempotencyTTL = time.Microsecond }},
		{name: "caller window", mutate: func(options *ChallengeOptions) { options.CallerWindow = time.Microsecond }},
		{name: "global window", mutate: func(options *ChallengeOptions) { options.GlobalWindow = time.Microsecond }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := testChallengeOptions(ChallengeOptions{})
			test.mutate(&options)
			if store, err := NewRedisChallengeStore(client, options); err == nil || store != nil {
				t.Fatalf("invalid constructor = %#v, %v; want nil, error", store, err)
			}
		})
	}
}

func TestChallengeObservabilityRedactsCodeAndSubject(t *testing.T) {
	store, server := newTestChallengeStore(t, ChallengeOptions{})
	var code string
	err := store.Issue(context.Background(), "caller-redaction", testChallengeSubject, testRegisterPurpose, func(_ context.Context, delivered string) error {
		code = delivered
		return errors.New("smtp rejected " + testChallengeSubject + " with " + delivered)
	})
	if !errors.Is(err, ErrChallengeDelivery) {
		t.Fatalf("delivery error = %v, want ErrChallengeDelivery", err)
	}
	if strings.Contains(err.Error(), testChallengeSubject) || strings.Contains(err.Error(), code) {
		t.Fatalf("public error leaked challenge material: %v", err)
	}
	for _, key := range server.Keys() {
		if strings.Contains(strings.ToLower(key), strings.ToLower(testChallengeSubject)) || strings.Contains(key, code) {
			t.Fatalf("Redis key leaked challenge material: %q", key)
		}
		if server.Type(key) == "hash" {
			values, hashErr := store.client.HGetAll(context.Background(), key).Result()
			if hashErr != nil {
				t.Fatalf("read Redis hash: %v", hashErr)
			}
			for field, value := range values {
				joined := field + "=" + value
				if strings.Contains(strings.ToLower(joined), strings.ToLower(testChallengeSubject)) || strings.Contains(joined, code) {
					t.Fatalf("Redis hash leaked challenge material: %q", joined)
				}
			}
		}
	}
}

func TestLegacyVerifyCodeStoreIsDisabled(t *testing.T) {
	legacy := NewVerifyCode(nil)
	if code, err := legacy.GenerateCode(context.Background(), testChallengeSubject, time.Minute); code != "" ||
		!errors.Is(err, ErrLegacyVerifyCodeDisabled) {
		t.Fatalf("legacy GenerateCode = %q, %v; want empty, disabled", code, err)
	}
	if ok, err := legacy.VerifyCode(context.Background(), testChallengeSubject, "123456"); ok ||
		!errors.Is(err, ErrLegacyVerifyCodeDisabled) {
		t.Fatalf("legacy VerifyCode = %v, %v; want false, disabled", ok, err)
	}
}

func newTestChallengeStore(t *testing.T, overrides ChallengeOptions) (*RedisChallengeStore, *miniredis.Miniredis) {
	t.Helper()
	client, server := newMiniredisClient(t)
	store, err := NewRedisChallengeStore(client, testChallengeOptions(overrides))
	if err != nil {
		t.Fatalf("NewRedisChallengeStore: %v", err)
	}
	return store, server
}

func newMiniredisClient(t *testing.T) (redis.UniversalClient, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	server.SetTime(time.Unix(1_800_000_000, 0).UTC())
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{server.Addr()}, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return client, server
}

func advanceChallengeTime(
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

func testChallengeOptions(overrides ChallengeOptions) ChallengeOptions {
	options := ChallengeOptions{
		Namespace:      "test:challenge",
		SubjectKey:     testChallengeSubjectKey,
		Peppers:        []ChallengePepper{{Version: "v1", Secret: testChallengePepperV1}},
		CodeTTL:        5 * time.Minute,
		PendingTTL:     5 * time.Second,
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
	if overrides.Namespace != "" {
		options.Namespace = overrides.Namespace
	}
	if overrides.SubjectKey != nil {
		options.SubjectKey = overrides.SubjectKey
	}
	if overrides.Peppers != nil {
		options.Peppers = overrides.Peppers
	}
	if overrides.CodeTTL != 0 {
		options.CodeTTL = overrides.CodeTTL
	}
	if overrides.PendingTTL != 0 {
		options.PendingTTL = overrides.PendingTTL
	}
	if overrides.Cooldown != 0 {
		options.Cooldown = overrides.Cooldown
	}
	if overrides.QuotaWindow != 0 {
		options.QuotaWindow = overrides.QuotaWindow
	}
	if overrides.MaxIssues != 0 {
		options.MaxIssues = overrides.MaxIssues
	}
	if overrides.MaxAttempts != 0 {
		options.MaxAttempts = overrides.MaxAttempts
	}
	if overrides.IdempotencyTTL != 0 {
		options.IdempotencyTTL = overrides.IdempotencyTTL
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
	return options
}

func mustIssueChallenge(t *testing.T, store *RedisChallengeStore, subject string, purpose ChallengePurpose) string {
	t.Helper()
	var code string
	if err := store.Issue(context.Background(), "caller-helper", subject, purpose, func(_ context.Context, delivered string) error {
		code = delivered
		return nil
	}); err != nil {
		t.Fatalf("Issue challenge: %v", err)
	}
	if !validChallengeCode(code) {
		t.Fatalf("issued code = %q, want fixed-width digits", code)
	}
	return code
}

func differentChallengeCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
