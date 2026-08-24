package dev

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.json")
	want := ServiceState{
		ServiceID:         "backend",
		Generation:        "test-generation",
		PID:               12345,
		StartedAt:         time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC),
		ProcessStartToken: "linux-proc:123456",
		HealthHeader:      defaultLaunchHeader,
		HealthExpectation: digestHealthNonce("round-trip-nonce"),
		Command:           []string{"go", "run", ".", "server"},
		Directory:         "/workspace",
		LogPath:           "/workspace/.mss/logs/backend.log",
		Detached:          true,
		Environment: map[string]string{
			"MSS_DEV_MANAGED": "true",
		},
	}
	if err := writeState(path, want); err != nil {
		t.Fatalf("write state: %v", err)
	}
	got, err := readState(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state mismatch\n got: %#v\nwant: %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestReadStateRejectsInvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"serviceId":"backend","pid":0}`), 0o600); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	if _, err := readState(path); err == nil {
		t.Fatal("expected invalid state to fail")
	}
}

func TestWriteTailReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var output bytes.Buffer
	if err := writeTail(path, 2, &output); err != nil {
		t.Fatalf("tail log: %v", err)
	}
	if output.String() != "three\nfour\n" {
		t.Fatalf("tail output = %q", output.String())
	}
}

func TestDevelopmentStateLogAndLockRefuseFinalLinks(t *testing.T) {
	for _, name := range []string{"state", "log", "lock"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			service := helperService("sleep")
			config := testConfig(t, root, service)
			if err := ensureDirectories(config); err != nil {
				t.Fatalf("ensure directories: %v", err)
			}
			target := filepath.Join(t.TempDir(), "outside.txt")
			const sentinel = "outside-must-not-change\n"
			if err := os.WriteFile(target, []byte(sentinel), 0o600); err != nil {
				t.Fatalf("write outside target: %v", err)
			}
			var linkedPath string
			switch name {
			case "state":
				linkedPath = config.StatePath(service.ID)
			case "log":
				linkedPath = config.LogPath(service.ID)
			case "lock":
				linkedPath = filepath.Join(config.RuntimeDirectory, ".lifecycle.lock")
			}
			if err := os.Symlink(target, linkedPath); err != nil {
				t.Skipf("create symlink or reparse point: %v", err)
			}
			report, err := Start(context.Background(), config, StartOptions{Detach: true})
			if err == nil || report.Success {
				t.Fatalf("%s link unexpectedly accepted: report=%#v err=%v", name, report, err)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil || string(content) != sentinel {
				t.Fatalf("%s link modified outside target: content=%q err=%v", name, content, readErr)
			}
		})
	}
}

func TestMergeEnvironmentIsStableAndOverridesValues(t *testing.T) {
	result := mergeEnvironment(
		[]string{"PATH=/usr/bin", "HOME=/tmp/home", "DUPLICATE=old", initialAdminPasswordEnvironment + "=inherited-secret", healthNonceEnvironment + "=inherited-nonce"},
		map[string]string{"DUPLICATE": "new", "MSS_DEV_MANAGED": "true", strings.ToLower(initialAdminPasswordEnvironment): "configured-secret"},
	)
	joined := strings.Join(result, "\n")
	if strings.Contains(joined, "inherited-secret") || strings.Contains(joined, "configured-secret") || strings.Contains(joined, "inherited-nonce") || strings.Contains(strings.ToUpper(joined), initialAdminPasswordEnvironment+"=") {
		t.Fatalf("one-use administrator password leaked into service environment %#v", result)
	}
	for _, expected := range []string{
		"DUPLICATE=new",
		"HOME=/tmp/home",
		"MSS_DEV_MANAGED=true",
		"PATH=/usr/bin",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in environment %#v", expected, result)
		}
	}
	for index := 1; index < len(result); index++ {
		if result[index-1] > result[index] {
			t.Fatalf("environment is not sorted: %#v", result)
		}
	}
}

func TestStartStripsInitialAdministratorPasswordFromRealChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	output := filepath.Join(root, "child-environment.txt")
	service := helperService("capture-env", output)
	config := testConfig(t, root, service)
	const secret = "MustNeverReachDevService2026"
	t.Setenv(initialAdminPasswordEnvironment, secret)

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err != nil || !report.Success {
		t.Fatalf("start environment capture helper: report=%#v err=%v", report, err)
	}
	t.Cleanup(func() {
		_, _ = Stop(context.Background(), config, StopOptions{Force: true})
	})
	content := waitForFile(t, output)
	if content != "" {
		t.Fatalf("development child inherited one-use administrator password %q", content)
	}
	state, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read managed state: %v", err)
	}
	for key := range state.Environment {
		if strings.EqualFold(key, initialAdminPasswordEnvironment) {
			t.Fatalf("managed state persisted forbidden environment key %q", key)
		}
	}
}

func TestManagedHealthNonceIsHashedInStateAndRequiredByStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process-group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	address := unusedLoopbackAddress(t)
	root := t.TempDir()
	service := helperService("serve", address, filepath.Join(root, "launches.txt"))
	service.Health = &HealthSpec{URL: "http://" + address + "/healthz", Interval: "10ms", Timeout: "2s", SuccessStatus: []int{http.StatusOK}}
	config := testConfig(t, root, service)
	t.Cleanup(func() { _, _ = Stop(context.Background(), config, StopOptions{Force: true}) })

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err != nil || !report.Success {
		t.Fatalf("start nonce-bound health fixture: report=%#v err=%v", report, err)
	}
	response, err := http.Get(service.Health.URL)
	if err != nil {
		t.Fatalf("read nonce-bound health response: %v", err)
	}
	nonce := response.Header.Get(defaultLaunchHeader)
	_ = response.Body.Close()
	if nonce == "" {
		t.Fatal("managed service did not echo launch nonce")
	}
	state, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read nonce-bound state: %v", err)
	}
	if state.HealthHeader != defaultLaunchHeader || state.HealthExpectation != digestHealthNonce(nonce) {
		t.Fatalf("state health expectation mismatch: %#v", state)
	}
	stateData, err := os.ReadFile(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read raw state: %v", err)
	}
	if strings.Contains(string(stateData), nonce) || strings.Contains(string(stateData), healthNonceEnvironment) {
		t.Fatalf("raw health nonce leaked into managed state: %s", stateData)
	}
	status, err := Status(context.Background(), config, nil)
	if err != nil || !status.Success || status.Services[0].Healthy == nil || !*status.Services[0].Healthy {
		t.Fatalf("nonce-bound status failed: report=%#v err=%v", status, err)
	}
	state.HealthExpectation = digestHealthNonce("different-launch")
	if err := writeState(config.StatePath(service.ID), state); err != nil {
		t.Fatalf("write mismatched health expectation: %v", err)
	}
	mismatched, err := Status(context.Background(), config, nil)
	if err != nil {
		t.Fatalf("inspect mismatched health expectation: %v", err)
	}
	if mismatched.Success || mismatched.Services[0].Status != "degraded" || !strings.Contains(mismatched.Services[0].Detail, managedLaunchMismatchDetail) {
		t.Fatalf("status accepted response from a different launch: %#v", mismatched)
	}
}

func TestHealthServerWinningAfterPreflightCannotAuthorizeUnrelatedChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process-group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	address := unusedLoopbackAddress(t)
	root := t.TempDir()
	service := helperService("sleep")
	service.Health = &HealthSpec{URL: "http://" + address + "/healthz", Interval: "10ms", Timeout: "250ms", SuccessStatus: []int{http.StatusOK}}
	config := testConfig(t, root, service)
	config.StopTimeout = 100 * time.Millisecond
	var server *http.Server
	config.afterHealthPreflight = func(ServiceSpec) {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("start post-preflight unrelated server: %v", err)
		}
		server = &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})}
		go func() { _ = server.Serve(listener) }()
	}
	t.Cleanup(func() {
		if server != nil {
			_ = server.Close()
		}
	})

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success || len(report.Services) != 1 || report.Services[0].Status != "failed-health" {
		t.Fatalf("unrelated post-preflight server authorized sleeping child: report=%#v err=%v", report, err)
	}
	if !strings.Contains(report.Services[0].Detail, "did not match the managed launch identity") {
		t.Fatalf("readiness failure did not identify launch-header mismatch: %#v", report.Services[0])
	}
	if err := waitForProcessExit(report.Services[0].PID, time.Second); err != nil {
		t.Fatalf("failed nonce-bound readiness left child running: %v", err)
	}
	if err := waitForPathAbsent(config.StatePath(service.ID), time.Second); err != nil {
		t.Fatalf("failed nonce-bound readiness left state: %v", err)
	}
}

func TestStartRejectsPreexistingHealthyURLBeforeLaunchingAliveNonListener(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	unrelated := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer unrelated.Close()

	root := t.TempDir()
	service := helperService("sleep")
	service.Health = &HealthSpec{
		URL:           unrelated.URL,
		Interval:      "10ms",
		Timeout:       "1s",
		SuccessStatus: []int{http.StatusOK},
	}
	config := testConfig(t, root, service)
	var tokenReads atomic.Int32
	config.processStartTokenReader = func(pid int) (string, error) {
		tokenReads.Add(1)
		return processStartToken(pid)
	}
	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success {
		t.Fatalf("unrelated healthy endpoint authorized exited child: report=%#v err=%v", report, err)
	}
	if len(report.Services) != 1 || report.Services[0].Status != "health-already-active" {
		t.Fatalf("readiness failure did not reject the pre-existing healthy URL: %#v", report.Services)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); !os.IsNotExist(stateErr) {
		t.Fatalf("failed child left managed state: %v", stateErr)
	}
	if got := tokenReads.Load(); got != 0 {
		t.Fatalf("pre-existing health check launched a non-listening child; process identity reads = %d", got)
	}
}

func TestTokenCaptureFailureTerminatesNoOpSignaledChildAndReleasesLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real child cleanup is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	config.StopTimeout = 75 * time.Millisecond
	pid := make(chan int, 1)
	config.processStartTokenReader = func(processID int) (string, error) {
		select {
		case pid <- processID:
		default:
		}
		return "", errors.New("injected process identity failure")
	}
	config.processSignaler = func(int, bool) error { return nil }

	startedAt := time.Now()
	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success {
		t.Fatalf("token-reader failure unexpectedly succeeded: report=%#v err=%v", report, err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("token-reader cleanup was not bounded: %s", elapsed)
	}
	var childPID int
	select {
	case childPID = <-pid:
	default:
		t.Fatal("token reader did not observe launched child")
	}
	if err := waitForProcessExit(childPID, time.Second); err != nil {
		t.Fatalf("token-reader failure left child running: %v", err)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); !os.IsNotExist(stateErr) {
		t.Fatalf("token-reader failure left state: %v", stateErr)
	}
	guard, lockErr := lockLifecycle(context.Background(), config, 250*time.Millisecond)
	if lockErr != nil {
		t.Fatalf("token-reader failure retained lifecycle lock: %v", lockErr)
	}
	guard.Close()
}

func TestStateWriteFailureTerminatesNoOpSignaledChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real child cleanup is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	config.StopTimeout = 75 * time.Millisecond
	config.stateWriter = func(string, ServiceState) error { return errors.New("injected state write failure") }
	config.processSignaler = func(int, bool) error { return nil }

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success || len(report.Services) != 1 || report.Services[0].PID <= 1 {
		t.Fatalf("state-writer failure unexpectedly succeeded: report=%#v err=%v", report, err)
	}
	if err := waitForProcessExit(report.Services[0].PID, time.Second); err != nil {
		t.Fatalf("state-writer failure left child running: %v", err)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); !os.IsNotExist(stateErr) {
		t.Fatalf("state-writer failure left state: %v", stateErr)
	}
}

func TestReadinessCancellationRollsBackChildAndState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process-group rollback is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	service.Health = &HealthSpec{
		URL:           "http://127.0.0.1:1/healthz",
		Interval:      "20ms",
		Timeout:       "10s",
		SuccessStatus: []int{http.StatusOK},
	}
	config := testConfig(t, root, service)
	config.StopTimeout = 250 * time.Millisecond
	pid := make(chan int, 1)
	config.processStartTokenReader = func(processID int) (string, error) {
		select {
		case pid <- processID:
		default:
		}
		return processStartToken(processID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		report Report
		err    error
	}
	finished := make(chan outcome, 1)
	go func() {
		report, err := Start(ctx, config, StartOptions{Detach: true})
		finished <- outcome{report: report, err: err}
	}()
	var childPID int
	select {
	case childPID = <-pid:
	case <-time.After(3 * time.Second):
		t.Fatal("readiness fixture did not launch")
	}
	cancel()
	select {
	case result := <-finished:
		if result.err == nil || result.report.Success {
			t.Fatalf("readiness cancellation unexpectedly succeeded: report=%#v err=%v", result.report, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readiness cancellation did not return after bounded rollback")
	}
	if err := waitForProcessExit(childPID, time.Second); err != nil {
		t.Fatalf("readiness cancellation left child running: %v", err)
	}
	if err := waitForPathAbsent(config.StatePath(service.ID), time.Second); err != nil {
		t.Fatalf("readiness cancellation left state: %v", err)
	}
}

func TestConcurrentStartLaunchesOneRealService(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	address := unusedLoopbackAddress(t)
	root := t.TempDir()
	launches := filepath.Join(root, "launches.txt")
	service := helperService("serve", address, launches)
	service.Health = &HealthSpec{
		URL:           "http://" + address + "/healthz",
		Interval:      "20ms",
		Timeout:       "3s",
		SuccessStatus: []int{http.StatusOK},
	}
	config := testConfig(t, root, service)
	t.Cleanup(func() {
		_, _ = Stop(context.Background(), config, StopOptions{Force: true})
	})

	type outcome struct {
		report Report
		err    error
	}
	ready := make(chan struct{})
	outcomes := make(chan outcome, 2)
	var callers sync.WaitGroup
	for range 2 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-ready
			report, err := Start(context.Background(), config, StartOptions{Detach: true})
			outcomes <- outcome{report: report, err: err}
		}()
	}
	close(ready)
	callers.Wait()
	close(outcomes)
	statuses := make([]string, 0, 2)
	for result := range outcomes {
		if result.err != nil || !result.report.Success || len(result.report.Services) != 1 {
			t.Fatalf("concurrent start failed: report=%#v err=%v", result.report, result.err)
		}
		statuses = append(statuses, result.report.Services[0].Status)
	}
	if !reflect.DeepEqual(sortedStrings(statuses), []string{"already-running", "running"}) {
		t.Fatalf("concurrent start statuses = %#v", statuses)
	}
	lines := strings.Fields(waitForFile(t, launches))
	if len(lines) != 1 {
		t.Fatalf("concurrent start launched %d service processes, want 1: %q", len(lines), waitForFile(t, launches))
	}
}

func TestExitedForegroundGenerationCannotDeleteRestartedState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process-group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	address := unusedLoopbackAddress(t)
	root := t.TempDir()
	trigger := filepath.Join(root, "exit-trigger")
	launches := filepath.Join(root, "launches.txt")
	service := helperService("serve-until-file", address, launches, trigger)
	service.Health = &HealthSpec{URL: "http://" + address + "/healthz", Interval: "10ms", Timeout: "2s", SuccessStatus: []int{http.StatusOK}}
	config := testConfig(t, root, service)
	foregroundReady := make(chan struct{})
	var foregroundOnce sync.Once
	config.foregroundReady = func() { foregroundOnce.Do(func() { close(foregroundReady) }) }
	removeEntered := make(chan struct{})
	allowRemove := make(chan struct{})
	var blocked atomic.Bool
	config.beforeStateRemove = func(ServiceState) {
		if blocked.CompareAndSwap(false, true) {
			close(removeEntered)
			<-allowRemove
		}
	}
	type outcome struct {
		report Report
		err    error
	}
	foreground := make(chan outcome, 1)
	go func() {
		report, err := Start(context.Background(), config, StartOptions{})
		foreground <- outcome{report: report, err: err}
	}()
	select {
	case <-foregroundReady:
	case <-time.After(3 * time.Second):
		t.Fatal("foreground manager did not reach signal-ready state")
	}
	firstState, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read first generation: %v", err)
	}
	if err := os.WriteFile(trigger, []byte("exit\n"), 0o600); err != nil {
		t.Fatalf("trigger first generation exit: %v", err)
	}
	select {
	case <-removeEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("first generation did not reach guarded state cleanup")
	}
	if err := os.Remove(trigger); err != nil {
		t.Fatalf("clear exit trigger before restart: %v", err)
	}
	restarted := make(chan outcome, 1)
	go func() {
		report, err := Start(context.Background(), config, StartOptions{Detach: true})
		restarted <- outcome{report: report, err: err}
	}()
	select {
	case result := <-restarted:
		t.Fatalf("restart bypassed lifecycle cleanup lock: report=%#v err=%v", result.report, result.err)
	case <-time.After(75 * time.Millisecond):
	}
	close(allowRemove)
	var restartedResult outcome
	select {
	case restartedResult = <-restarted:
	case <-time.After(4 * time.Second):
		t.Fatal("restart did not finish after first generation cleanup")
	}
	if restartedResult.err != nil || !restartedResult.report.Success {
		t.Fatalf("restart failed: report=%#v err=%v", restartedResult.report, restartedResult.err)
	}
	secondState, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read restarted generation: %v", err)
	}
	if secondState.Generation == firstState.Generation {
		t.Fatalf("restart reused lifecycle generation %q", secondState.Generation)
	}
	select {
	case firstResult := <-foreground:
		if firstResult.err == nil {
			t.Fatalf("naturally exited foreground manager returned no exit error: %#v", firstResult.report)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("foreground manager did not return after first generation exit")
	}
	current, err := readState(config.StatePath(service.ID))
	if err != nil || current.Generation != secondState.Generation {
		t.Fatalf("old generation cleanup removed or replaced restarted state: state=%#v err=%v", current, err)
	}
	if _, err := processStartToken(secondState.PID); err != nil {
		t.Fatalf("old generation cleanup terminated restarted process: %v", err)
	}
	if report, err := Stop(context.Background(), config, StopOptions{Force: true}); err != nil || !report.Success {
		t.Fatalf("cleanup restarted generation: report=%#v err=%v", report, err)
	}
}

func TestForegroundManagerSIGTERMCleansChildAndState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM foreground manager behavior is Unix-specific")
	}
	root := t.TempDir()
	address := unusedLoopbackAddress(t)
	launches := filepath.Join(root, "child-launches.txt")
	ready := filepath.Join(root, "manager-ready")
	command := exec.Command(os.Args[0], "-test.run=^TestDevHelperProcess$", "--", "run-foreground-manager", root, address, launches, ready)
	command.Env = append(os.Environ(), "GO_WANT_DEV_HELPER=1")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatalf("start foreground manager helper: %v", err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	_ = waitForFile(t, ready)
	statePath := filepath.Join(root, ".mss", "run", "backend.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("read foreground-managed state: %v", err)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal foreground manager: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("foreground manager did not handle SIGTERM cleanly: %v output=%s", err, output.String())
		}
	case <-time.After(4 * time.Second):
		t.Fatalf("foreground manager hung after SIGTERM; output=%s", output.String())
	}
	if err := waitForProcessExit(state.PID, time.Second); err != nil {
		t.Fatalf("foreground SIGTERM left child running: %v", err)
	}
	if err := waitForPathAbsent(statePath, time.Second); err != nil {
		t.Fatalf("foreground SIGTERM left state: %v", err)
	}
}

func TestExistingUnhealthyManagedServiceFailsStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	service.Health = &HealthSpec{
		URL:           "http://127.0.0.1:1/healthz",
		Interval:      "10ms",
		Timeout:       "100ms",
		SuccessStatus: []int{http.StatusOK},
	}
	config := testConfig(t, root, service)
	state, command := launchStateFixture(t, config, service)
	defer cleanupFixtureProcess(t, command, config.StatePath(service.ID), state)

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success {
		t.Fatalf("existing unhealthy service was reported as successful: report=%#v err=%v", report, err)
	}
	if len(report.Services) != 1 || report.Services[0].Status != "degraded" {
		t.Fatalf("existing unhealthy service status = %#v", report.Services)
	}
	if token, tokenErr := processStartToken(state.PID); tokenErr != nil || token != state.ProcessStartToken {
		t.Fatalf("existing unhealthy service was not preserved: token=%q err=%v", token, tokenErr)
	}
}

func TestStopRefusesReusedPIDIdentityWithoutSignaling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process group lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	state, command := launchStateFixture(t, config, service)
	realToken := state.ProcessStartToken
	state.ProcessStartToken = "reused-pid-identity"
	if err := writeState(config.StatePath(service.ID), state); err != nil {
		t.Fatalf("write mismatched state: %v", err)
	}
	defer cleanupFixtureProcess(t, command, config.StatePath(service.ID), state)

	report, err := Stop(context.Background(), config, StopOptions{Force: true})
	if err == nil || report.Success {
		t.Fatalf("stop accepted reused PID identity: report=%#v err=%v", report, err)
	}
	if len(report.Services) != 1 || report.Services[0].Status != "identity-mismatch" {
		t.Fatalf("reused PID stop status = %#v", report.Services)
	}
	if token, tokenErr := processStartToken(state.PID); tokenErr != nil || token != realToken {
		t.Fatalf("identity mismatch stop signaled unrelated process: token=%q err=%v", token, tokenErr)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); stateErr != nil {
		t.Fatalf("identity mismatch evidence was removed: %v", stateErr)
	}
}

func TestStopRefusesProcessTokenFromDifferentLinuxBoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux boot identity is Linux-specific")
	}
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	state, command := launchStateFixture(t, config, service)
	defer cleanupFixtureProcess(t, command, config.StatePath(service.ID), state)
	parts := strings.Split(state.ProcessStartToken, ":")
	if len(parts) != 3 || parts[0] != "linux-proc" {
		t.Fatalf("unexpected Linux token %q", state.ProcessStartToken)
	}
	parts[1] = "00000000-0000-0000-0000-000000000000"
	state.ProcessStartToken = strings.Join(parts, ":")
	if err := writeState(config.StatePath(service.ID), state); err != nil {
		t.Fatalf("write old-boot state: %v", err)
	}

	report, err := Stop(context.Background(), config, StopOptions{Force: true})
	if err == nil || report.Success || report.Services[0].Status != "identity-mismatch" {
		t.Fatalf("stop accepted token from another Linux boot: report=%#v err=%v", report, err)
	}
	if _, tokenErr := processStartToken(state.PID); tokenErr != nil {
		t.Fatalf("old-boot token stop signaled live process: %v", tokenErr)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); stateErr != nil {
		t.Fatalf("old-boot identity evidence was removed: %v", stateErr)
	}
}

func TestStopEscalatesWhenServiceIgnoresSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM escalation is Unix-specific")
	}
	address := unusedLoopbackAddress(t)
	root := t.TempDir()
	launches := filepath.Join(root, "launches.txt")
	service := helperService("serve-ignore-term", address, launches)
	service.Health = &HealthSpec{
		URL:           "http://" + address + "/healthz",
		Interval:      "10ms",
		Timeout:       "2s",
		SuccessStatus: []int{http.StatusOK},
	}
	config := testConfig(t, root, service)
	config.StopTimeout = 100 * time.Millisecond
	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err != nil || !report.Success {
		t.Fatalf("start SIGTERM-resistant helper: report=%#v err=%v", report, err)
	}
	state, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read resistant helper state: %v", err)
	}

	stopReport, err := Stop(context.Background(), config, StopOptions{})
	if err != nil || !stopReport.Success || stopReport.Services[0].Status != "stopped" {
		t.Fatalf("stop did not escalate to forced termination: report=%#v err=%v", stopReport, err)
	}
	if err := waitForProcessExit(state.PID, time.Second); err != nil {
		t.Fatalf("SIGTERM-resistant helper survived escalation: %v", err)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); !os.IsNotExist(stateErr) {
		t.Fatalf("confirmed stop retained state: %v", stateErr)
	}
}

func TestStopSignalFailurePreservesStateEvidenceAndProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real child lifecycle is covered on Unix; Windows is cross-compiled separately")
	}
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err != nil || !report.Success {
		t.Fatalf("start signal-failure fixture: report=%#v err=%v", report, err)
	}
	state, err := readState(config.StatePath(service.ID))
	if err != nil {
		t.Fatalf("read signal-failure state: %v", err)
	}
	config.processSignaler = func(int, bool) error { return errors.New("injected signal failure") }

	stopReport, err := Stop(context.Background(), config, StopOptions{Force: true})
	if err == nil || stopReport.Success || stopReport.Services[0].Status != "stop-failed" {
		t.Fatalf("signal failure was hidden: report=%#v err=%v", stopReport, err)
	}
	if token, tokenErr := processStartToken(state.PID); tokenErr != nil || token != state.ProcessStartToken {
		t.Fatalf("signal failure did not preserve process identity: token=%q err=%v", token, tokenErr)
	}
	if _, stateErr := os.Stat(config.StatePath(service.ID)); stateErr != nil {
		t.Fatalf("signal failure removed state evidence: %v", stateErr)
	}

	config.processSignaler = nil
	if cleanupReport, cleanupErr := Stop(context.Background(), config, StopOptions{Force: true}); cleanupErr != nil || !cleanupReport.Success {
		t.Fatalf("cleanup signal-failure fixture: report=%#v err=%v", cleanupReport, cleanupErr)
	}
}

func TestPartialStartRollbackConfirmsStartedServiceExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real process-group rollback is covered on Unix; Windows is cross-compiled separately")
	}
	backendAddress := unusedLoopbackAddress(t)
	unrelated := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer unrelated.Close()
	root := t.TempDir()
	backend := helperService("serve-ignore-term", backendAddress, filepath.Join(root, "backend-launches.txt"))
	backend.ID = "backend"
	backend.Health = &HealthSpec{URL: "http://" + backendAddress + "/healthz", Interval: "10ms", Timeout: "2s", SuccessStatus: []int{http.StatusOK}}
	frontend := helperService("sleep")
	frontend.ID = "frontend"
	frontend.DependsOn = []string{"backend"}
	frontend.Health = &HealthSpec{URL: unrelated.URL, Interval: "10ms", Timeout: "1s", SuccessStatus: []int{http.StatusOK}}
	config := multiServiceTestConfig(t, root, backend, frontend)
	config.StopTimeout = 100 * time.Millisecond

	report, err := Start(context.Background(), config, StartOptions{Detach: true})
	if err == nil || report.Success || len(report.Services) != 2 || report.Services[0].PID <= 1 {
		t.Fatalf("partial start unexpectedly succeeded: report=%#v err=%v", report, err)
	}
	if err := waitForProcessExit(report.Services[0].PID, time.Second); err != nil {
		t.Fatalf("partial-start rollback left first service running: %v", err)
	}
	if _, stateErr := os.Stat(config.StatePath(backend.ID)); !os.IsNotExist(stateErr) {
		t.Fatalf("partial-start rollback retained confirmed-stopped state: %v", stateErr)
	}
}

func TestStateIdentityIncludesConfiguredCommandAndDirectory(t *testing.T) {
	root := t.TempDir()
	service := helperService("sleep")
	config := testConfig(t, root, service)
	token, err := processStartToken(os.Getpid())
	if err != nil {
		t.Fatalf("current process identity: %v", err)
	}
	base := ServiceState{
		ServiceID:         service.ID,
		Generation:        "test-generation",
		PID:               os.Getpid(),
		StartedAt:         time.Now().UTC(),
		ProcessStartToken: token,
		Command:           append([]string(nil), service.Command...),
		Directory:         config.ResolveDirectory(service),
		LogPath:           config.LogPath(service.ID),
	}
	for name, mutate := range map[string]func(*ServiceState){
		"command":   func(state *ServiceState) { state.Command = []string{"different-command"} },
		"directory": func(state *ServiceState) { state.Directory = filepath.Join(root, "different") },
	} {
		t.Run(name, func(t *testing.T) {
			state := base
			mutate(&state)
			inspection, _ := inspectServiceState(config, service, state)
			if inspection != stateProcessIdentityMismatch {
				t.Fatalf("state %s drift inspection = %v, want identity mismatch", name, inspection)
			}
		})
	}
}

func helperService(mode string, arguments ...string) ServiceSpec {
	command := []string{os.Args[0], "-test.run=^TestDevHelperProcess$", "--", mode}
	command = append(command, arguments...)
	return ServiceSpec{
		ID:        "backend",
		Directory: ".",
		Command:   command,
		Environment: map[string]string{
			"GO_WANT_DEV_HELPER": "1",
		},
		Required: true,
	}
}

func testConfig(t *testing.T, root string, service ServiceSpec) *Config {
	t.Helper()
	return multiServiceTestConfig(t, root, service)
}

func multiServiceTestConfig(t *testing.T, root string, services ...ServiceSpec) *Config {
	t.Helper()
	runtimeDirectory := filepath.Join(root, ".mss", "run")
	logDirectory := filepath.Join(root, ".mss", "logs")
	if err := os.MkdirAll(runtimeDirectory, 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	serviceMap := make(map[string]ServiceSpec, len(services))
	serviceDirectories := make(map[string]string, len(services))
	for index := range services {
		service := services[index]
		if service.Health != nil && service.Health.LaunchHeader == "" {
			service.Health.LaunchHeader = defaultLaunchHeader
			services[index] = service
		}
		serviceMap[service.ID] = service
		serviceDirectories[service.ID] = filepath.Join(root, filepath.FromSlash(service.Directory))
	}
	return &Config{
		Root:               root,
		Document:           Document{Metadata: Metadata{Project: "dev-test"}, Spec: Spec{Services: append([]ServiceSpec(nil), services...)}},
		StartupTimeout:     3 * time.Second,
		StopTimeout:        time.Second,
		RuntimeDirectory:   runtimeDirectory,
		LogDirectory:       logDirectory,
		services:           serviceMap,
		serviceDirectories: serviceDirectories,
	}
}

func launchStateFixture(t *testing.T, config *Config, service ServiceSpec) (ServiceState, *exec.Cmd) {
	t.Helper()
	if err := ensureDirectories(config); err != nil {
		t.Fatalf("ensure development directories: %v", err)
	}
	command := exec.Command(service.Command[0], service.Command[1:]...)
	command.Dir = config.ResolveDirectory(service)
	serviceEnvironment := copyEnvironment(service.Environment)
	healthNonce := ""
	if service.Health != nil {
		var err error
		healthNonce, err = newHealthNonce()
		if err != nil {
			t.Fatalf("create fixture health nonce: %v", err)
		}
		serviceEnvironment[healthNonceEnvironment] = healthNonce
	}
	command.Env = mergeEnvironment(os.Environ(), serviceEnvironment)
	prepareProcess(command)
	if err := command.Start(); err != nil {
		t.Fatalf("start state fixture: %v", err)
	}
	token, err := processStartToken(command.Process.Pid)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("read state fixture identity: %v", err)
	}
	state := ServiceState{
		ServiceID:         service.ID,
		Generation:        "test-generation",
		PID:               command.Process.Pid,
		StartedAt:         time.Now().UTC(),
		ProcessStartToken: token,
		HealthHeader: func() string {
			if service.Health == nil {
				return ""
			}
			return service.Health.LaunchHeader
		}(),
		HealthExpectation: func() string {
			if healthNonce == "" {
				return ""
			}
			return digestHealthNonce(healthNonce)
		}(),
		Command:     append([]string(nil), service.Command...),
		Directory:   command.Dir,
		LogPath:     config.LogPath(service.ID),
		Detached:    true,
		Environment: sanitizedEnvironment(service.Environment),
	}
	if err := writeState(config.StatePath(service.ID), state); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("write state fixture: %v", err)
	}
	return state, command
}

func cleanupFixtureProcess(t *testing.T, command *exec.Cmd, statePath string, state ServiceState) {
	t.Helper()
	if command != nil && command.Process != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	_ = os.Remove(statePath)
}

func unusedLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release loopback address: %v", err)
	}
	return address
}

func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("read helper output: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for helper output %s", path)
	return ""
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := processStartToken(pid)
		if errors.Is(err, errProcessNotRunning) {
			return nil
		}
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("PID %d remained alive for %s", pid, timeout)
}

func waitForPathAbsent(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("path %s remained present for %s", path, timeout)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	for left := range result {
		for right := left + 1; right < len(result); right++ {
			if result[right] < result[left] {
				result[left], result[right] = result[right], result[left]
			}
		}
	}
	return result
}

func TestDevHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_DEV_HELPER") != "1" {
		return
	}
	arguments := helperArguments(os.Args)
	if len(arguments) == 0 {
		os.Exit(90)
	}
	switch arguments[0] {
	case "capture-env":
		if len(arguments) != 2 {
			os.Exit(91)
		}
		if err := os.WriteFile(arguments[1], []byte(os.Getenv(initialAdminPasswordEnvironment)), 0o600); err != nil {
			os.Exit(92)
		}
		time.Sleep(10 * time.Minute)
	case "serve", "serve-ignore-term", "serve-until-file":
		if len(arguments) != 3 {
			if arguments[0] != "serve-until-file" || len(arguments) != 4 {
				os.Exit(93)
			}
		}
		if arguments[0] == "serve-ignore-term" {
			signal.Ignore(syscall.SIGTERM)
		}
		listener, err := net.Listen("tcp", arguments[1])
		if err != nil {
			os.Exit(94)
		}
		file, err := os.OpenFile(arguments[2], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			_ = listener.Close()
			os.Exit(94)
		}
		_, _ = fmt.Fprintln(file, os.Getpid())
		_ = file.Close()
		handler := http.NewServeMux()
		handler.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			if nonce := os.Getenv(healthNonceEnvironment); nonce != "" {
				writer.Header().Set(defaultLaunchHeader, nonce)
			}
			writer.WriteHeader(http.StatusOK)
		})
		server := &http.Server{Handler: handler}
		if arguments[0] == "serve-until-file" {
			served := make(chan error, 1)
			go func() { served <- server.Serve(listener) }()
			for {
				if _, err := os.Stat(arguments[3]); err == nil {
					_ = server.Close()
					os.Exit(0)
				} else if !os.IsNotExist(err) {
					os.Exit(95)
				}
				select {
				case err := <-served:
					if !errors.Is(err, http.ErrServerClosed) {
						os.Exit(95)
					}
					os.Exit(0)
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			os.Exit(95)
		}
	case "run-foreground-manager":
		if len(arguments) != 5 {
			os.Exit(97)
		}
		service := helperService("serve", arguments[2], arguments[3])
		service.Health = &HealthSpec{URL: "http://" + arguments[2] + "/healthz", Interval: "10ms", Timeout: "2s", SuccessStatus: []int{http.StatusOK}}
		config := testConfig(t, arguments[1], service)
		config.foregroundReady = func() {
			if err := os.WriteFile(arguments[4], []byte("ready\n"), 0o600); err != nil {
				os.Exit(98)
			}
		}
		if report, err := Start(context.Background(), config, StartOptions{}); err != nil || !report.Success {
			os.Exit(99)
		}
	case "sleep":
		time.Sleep(10 * time.Minute)
	case "exit":
		os.Exit(23)
	default:
		os.Exit(96)
	}
	os.Exit(0)
}

func helperArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" && index+1 < len(arguments) {
			return arguments[index+1:]
		}
	}
	return nil
}
