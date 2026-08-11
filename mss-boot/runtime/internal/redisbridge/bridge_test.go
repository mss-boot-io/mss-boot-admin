package redisbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/internal/redisbridge"
	"gopkg.in/yaml.v3"
)

func TestOpaqueAtomicGroupUsesOneServerDerivedSlot(t *testing.T) {
	const identity = "person@example.test"
	group, err := redisbridge.NewAtomicGroup("challenge-subject", []byte(identity))
	if err != nil {
		t.Fatalf("NewAtomicGroup: %v", err)
	}
	first, err := group.Key("state")
	if err != nil {
		t.Fatalf("state Key: %v", err)
	}
	second, err := group.Key("quota")
	if err != nil {
		t.Fatalf("quota Key: %v", err)
	}
	for _, formatted := range []string{
		fmt.Sprint(group), fmt.Sprintf("%#v", group),
		fmt.Sprint(first), fmt.Sprintf("%#v", first),
		fmt.Sprint(redisbridge.ChallengeBeginScript()), fmt.Sprintf("%#v", redisbridge.ChallengeBeginScript()),
	} {
		if strings.Contains(formatted, identity) || strings.Contains(formatted, "state") || strings.Contains(formatted, "redis.call") || strings.Contains(formatted, "{") {
			t.Fatalf("capability formatting leaked implementation material: %q", formatted)
		}
	}

	executor := &recordingExecutor{result: "OK"}
	driver := &recordingDriver{executor: executor}
	source := fakeSource{driver: driver}
	var reply redisbridge.Reply
	err = redisbridge.Use(context.Background(), source, group, func(lease redisbridge.Lease) error {
		var runErr error
		reply, runErr = lease.Run(context.Background(), redisbridge.ChallengeBeginScript(), []redisbridge.Key{first, second}, "opaque-argument")
		return runErr
	})
	if err != nil {
		t.Fatalf("Use/Run: %v", err)
	}
	if driver.calls != 1 || executor.calls != 1 || len(executor.keys) != 2 {
		t.Fatalf("calls driver=%d executor=%d keys=%d; want 1,1,2", driver.calls, executor.calls, len(executor.keys))
	}
	if slot(executor.keys[0]) == "" || slot(executor.keys[0]) != slot(executor.keys[1]) {
		t.Fatalf("physical keys are not same-slot: %v", executor.keys)
	}
	for _, key := range executor.keys {
		if strings.Contains(key, identity) {
			t.Fatalf("physical key leaked group identity: %q", key)
		}
	}
	for _, value := range []any{group, first, redisbridge.ChallengeBeginScript(), reply} {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			formatted := fmt.Sprintf(format, value)
			if strings.Contains(formatted, identity) || strings.Contains(formatted, "opaque-argument") || strings.Contains(formatted, "redis.call") {
				t.Fatalf("capability formatting leaked implementation material: %q", formatted)
			}
		}
	}
	for _, formatted := range driver.requestFormats {
		if strings.Contains(formatted, identity) || strings.Contains(formatted, "opaque-argument") || strings.Contains(formatted, "redis.call") {
			t.Fatalf("request formatting leaked implementation material: %q", formatted)
		}
	}
	wantRequest := "RedisFixedRequest<opaque>"
	wantFormats := []string{wantRequest, wantRequest, wantRequest, wantRequest, fmt.Sprintf("%q", wantRequest)}
	if fmt.Sprint(driver.requestFormats) != fmt.Sprint(wantFormats) {
		t.Fatalf("request formats = %q, want %q", driver.requestFormats, wantFormats)
	}
	if driver.diagnosticErr != nil {
		t.Fatalf("request diagnostics: %v", driver.diagnosticErr)
	}
	wantJSON, _ := json.Marshal(wantRequest)
	wantYAML, _ := yaml.Marshal(wantRequest)
	wantTextLog, wantJSONLog := renderBridgeLogs(wantRequest)
	if !bytes.Equal(driver.requestJSON, wantJSON) || !bytes.Equal(driver.requestYAML, wantYAML) ||
		driver.requestTextLog != wantTextLog || driver.requestJSONLog != wantJSONLog {
		t.Fatalf("request diagnostics json=%s yaml=%q text=%q jsonlog=%q", driver.requestJSON, driver.requestYAML, driver.requestTextLog, driver.requestJSONLog)
	}
}

func TestCrossGroupAndInvalidScriptRejectBeforeDriver(t *testing.T) {
	one, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("one"))
	two, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("two"))
	oneKey, _ := one.Key("state")
	twoKey, _ := two.Key("state")

	for name, run := range map[string]func(redisbridge.Lease) error{
		"cross-group": func(lease redisbridge.Lease) error {
			_, err := lease.Run(context.Background(), redisbridge.ChallengeBeginScript(), []redisbridge.Key{oneKey, twoKey})
			if !errors.Is(err, redisbridge.ErrCrossGroup) {
				t.Fatalf("Run error = %v, want ErrCrossGroup", err)
			}
			return nil
		},
		"invalid-script": func(lease redisbridge.Lease) error {
			_, err := lease.Run(context.Background(), redisbridge.Script{}, []redisbridge.Key{oneKey})
			if !errors.Is(err, redisbridge.ErrInvalidScript) {
				t.Fatalf("Run error = %v, want ErrInvalidScript", err)
			}
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			driver := &recordingDriver{executor: &recordingExecutor{result: "OK"}}
			if err := redisbridge.Use(context.Background(), fakeSource{driver: driver}, one, run); err != nil {
				t.Fatalf("Use: %v", err)
			}
			if driver.calls != 0 || driver.executor.calls != 0 {
				t.Fatalf("rejected request reached I/O: driver=%d executor=%d", driver.calls, driver.executor.calls)
			}
		})
	}
}

type fakeSource struct{ driver redisbridge.Driver }

func (s fakeSource) RedisBridgeUse(_ context.Context, callback func(redisbridge.Driver) error) error {
	return callback(s.driver)
}

type recordingDriver struct {
	calls          int
	executor       *recordingExecutor
	requestFormats []string
	requestJSON    []byte
	requestYAML    []byte
	requestTextLog string
	requestJSONLog string
	diagnosticErr  error
}

func (d *recordingDriver) RedisBridgeRun(ctx context.Context, request redisbridge.Request) (redisbridge.Reply, error) {
	d.calls++
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		d.requestFormats = append(d.requestFormats, fmt.Sprintf(format, request))
	}
	d.requestJSON, d.diagnosticErr = json.Marshal(request)
	if d.diagnosticErr == nil {
		d.requestYAML, d.diagnosticErr = yaml.Marshal(request)
	}
	d.requestTextLog, d.requestJSONLog = renderBridgeLogs(request)
	return request.Execute(ctx, "resource:scope", d.executor)
}

type recordingExecutor struct {
	calls  int
	keys   []string
	source string
	result any
}

func (e *recordingExecutor) RedisBridgeEvalFixed(_ context.Context, source string, keys []string, _ ...any) (any, error) {
	e.calls++
	e.source = source
	e.keys = append([]string(nil), keys...)
	return e.result, nil
}

func slot(key string) string {
	start := strings.IndexByte(key, '{')
	end := strings.IndexByte(key, '}')
	if start < 0 || end <= start+1 {
		return ""
	}
	return key[start+1 : end]
}

func renderBridgeLogs(value any) (string, string) {
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
