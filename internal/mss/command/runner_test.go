package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type processReferenceForTest struct {
	PID      int
	Identity string
}

func captureProcessReferenceForTest(pid int) processReferenceForTest {
	reference := processReferenceForTest{PID: pid}
	if runtime.GOOS != "linux" {
		return reference
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return reference
	}
	state, startTime, ok := linuxProcessStateAndStartTimeForTest(data)
	if !ok || processStateExitedForTest(state) {
		return reference
	}
	reference.Identity = startTime
	return reference
}

func formatProcessReferenceForTest(reference processReferenceForTest) string {
	return fmt.Sprintf("%d:%s", reference.PID, reference.Identity)
}

func parseProcessReferenceForTest(value string) (processReferenceForTest, error) {
	pidValue, identity, _ := strings.Cut(value, ":")
	pid, err := strconv.Atoi(pidValue)
	if err != nil || pid <= 0 {
		return processReferenceForTest{}, fmt.Errorf("invalid process reference %q", value)
	}
	return processReferenceForTest{PID: pid, Identity: identity}, nil
}

func processReferenceExistsForTest(reference processReferenceForTest) bool {
	if runtime.GOOS != "linux" || reference.Identity == "" {
		return processExistsForTest(reference.PID)
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(reference.PID) + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		return processExistsForTest(reference.PID)
	}
	state, startTime, ok := linuxProcessStateAndStartTimeForTest(data)
	if !ok {
		return processExistsForTest(reference.PID)
	}
	return !processStateExitedForTest(state) && startTime == reference.Identity
}

func linuxProcessStateAndStartTimeForTest(data []byte) (byte, string, bool) {
	value := string(data)
	closing := strings.LastIndexByte(value, ')')
	if closing < 0 {
		return 0, "", false
	}
	fields := strings.Fields(value[closing+1:])
	if len(fields) <= 19 || len(fields[0]) != 1 {
		return 0, "", false
	}
	return fields[0][0], fields[19], true
}

func processStateExitedForTest(state byte) bool {
	return state == 'Z' || state == 'X' || state == 'x'
}

func waitForProcessExitForTest(exists func() bool, timeout, pollInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !exists() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

func TestMergeEnvironmentUnsetsInheritedValuesBeforeOverrides(t *testing.T) {
	base := []string{
		"PATH=/base/bin",
		"MSS_ADMIN_INITIAL_PASSWORD=must-not-leak",
		"UNCHANGED=value",
	}
	merged := mergeEnvironment(
		base,
		map[string]string{
			"PATH":                       "/override/bin",
			"MSS_ADMIN_INITIAL_PASSWORD": "migration-only",
		},
		[]string{"MSS_ADMIN_INITIAL_PASSWORD"},
	)
	joined := strings.Join(merged, "\n")
	if strings.Contains(joined, "must-not-leak") {
		t.Fatalf("inherited secret survived explicit unset: %q", joined)
	}
	for _, expected := range []string{
		"MSS_ADMIN_INITIAL_PASSWORD=migration-only",
		"PATH=/override/bin",
		"UNCHANGED=value",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("merged environment %q does not contain %q", joined, expected)
		}
	}
}

func TestMergeEnvironmentCanRemoveInheritedValue(t *testing.T) {
	key := "MSS_ADMIN_INITIAL_PASSWORD"
	unsetKey := key
	if runtime.GOOS == "windows" {
		unsetKey = strings.ToLower(key)
	}
	merged := mergeEnvironment(
		[]string{key + "=must-not-leak", "SAFE=value"},
		nil,
		[]string{unsetKey},
	)
	joined := strings.Join(merged, "\n")
	if strings.Contains(joined, key+"=") || strings.Contains(joined, "must-not-leak") {
		t.Fatalf("explicit unset left inherited secret in environment: %q", joined)
	}
	if !strings.Contains(joined, "SAFE=value") {
		t.Fatalf("explicit unset removed unrelated environment: %q", joined)
	}
}

func TestRunRemovesInheritedBootstrapSecret(t *testing.T) {
	const secret = "InheritedBootstrap2026"
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", secret)
	result := Run(context.Background(), Spec{
		ID:        "sensitive-unset",
		Directory: t.TempDir(),
		Args:      []string{os.Args[0], "-test.run=TestCommandHelperProcess", "--", "expect-unset"},
		Environment: map[string]string{
			"GO_WANT_COMMAND_HELPER": "1",
		},
	})
	if result.ExitCode != 0 {
		t.Fatalf("command inherited bootstrap secret: %#v", result)
	}
	if strings.Contains(result.Stdout+result.Stderr+result.Error, secret) {
		t.Fatalf("command result exposed inherited bootstrap secret: %#v", result)
	}
}

func TestRunAllowsExplicitScopedSecretAndRedactsResult(t *testing.T) {
	const secret = "migration-only"
	t.Setenv("MSS_ADMIN_INITIAL_PASSWORD", "inherited-must-not-win")
	result := Run(context.Background(), Spec{
		ID:        "sensitive-override",
		Directory: t.TempDir(),
		Args:      []string{os.Args[0], "-test.run=TestCommandHelperProcess", "--", "expect-scoped"},
		Environment: map[string]string{
			"GO_WANT_COMMAND_HELPER":     "1",
			"MSS_ADMIN_INITIAL_PASSWORD": secret,
		},
	})
	if result.ExitCode != 0 {
		t.Fatalf("command did not receive explicitly scoped secret: %#v", result)
	}
	serialized := result.Stdout + result.Stderr + result.Error
	if strings.Contains(serialized, secret) || strings.Contains(serialized, "inherited-must-not-win") {
		t.Fatalf("command result exposed bootstrap secret: %#v", result)
	}
	if !strings.Contains(serialized, "[REDACTED]") {
		t.Fatalf("command result did not redact scoped secret: %#v", result)
	}
}

func TestRunTimeoutTerminatesSecretBearingProcessTree(t *testing.T) {
	const secret = "TreeScopedBootstrap2026"
	pidFile := filepath.Join(t.TempDir(), "process-tree.pids")
	result := Run(context.Background(), Spec{
		ID:        "timeout-process-tree",
		Directory: t.TempDir(),
		Args:      []string{os.Args[0], "-test.run=TestCommandHelperProcess", "--", "spawn-grandchild"},
		Environment: map[string]string{
			"GO_WANT_COMMAND_HELPER":     "1",
			"COMMAND_HELPER_PID_FILE":    pidFile,
			"MSS_ADMIN_INITIAL_PASSWORD": secret,
		},
		Timeout: 2 * time.Second,
	})
	if !strings.Contains(result.Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("timed command error = %q, want deadline exceeded", result.Error)
	}
	serialized := result.Stdout + result.Stderr + result.Error
	if strings.Contains(serialized, secret) {
		t.Fatalf("timed command result exposed bootstrap secret: %#v", result)
	}
	if !strings.Contains(serialized, "[REDACTED]") {
		t.Fatalf("timed command result did not redact process-tree output: %#v", result)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read helper process identities: %v; result=%#v", err, result)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		t.Fatalf("helper process identities = %q, want parent and grandchild", data)
	}
	for _, field := range fields {
		reference, err := parseProcessReferenceForTest(field)
		if err != nil {
			t.Fatalf("invalid helper process identity %q", field)
		}
		if !waitForProcessExitForTest(
			func() bool { return processReferenceExistsForTest(reference) },
			5*time.Second,
			25*time.Millisecond,
		) {
			t.Fatalf("timed command left process %d running", reference.PID)
		}
	}
}

func TestWaitForProcessExitStopsAfterFirstMissingSample(t *testing.T) {
	samples := []bool{false, true}
	calls := 0
	exited := waitForProcessExitForTest(func() bool {
		value := samples[calls]
		calls++
		return value
	}, time.Second, time.Millisecond)
	if !exited {
		t.Fatal("wait reported a live process after observing it missing")
	}
	if calls != 1 {
		t.Fatalf("existence probe called %d times, want 1", calls)
	}
}

func TestWaitForCommandCleanupIsBoundedWhenWaitNeverReturns(t *testing.T) {
	started := time.Now()
	waitErr, cleanupErr := waitForCommandCleanup(make(chan error), 20*time.Millisecond)
	if waitErr != nil {
		t.Fatalf("wait error = %v, want nil", waitErr)
	}
	if !errors.Is(cleanupErr, ErrCommandCleanupTimeout) {
		t.Fatalf("cleanup error = %v, want ErrCommandCleanupTimeout", cleanupErr)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded cleanup waited %s", elapsed)
	}
}

func TestCommandHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMMAND_HELPER") != "1" {
		return
	}
	mode := ""
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) {
			mode = os.Args[index+1]
			break
		}
	}
	value := os.Getenv("MSS_ADMIN_INITIAL_PASSWORD")
	switch mode {
	case "expect-unset":
		if value != "" {
			_, _ = os.Stderr.WriteString("unexpected secret " + value)
			os.Exit(11)
		}
	case "expect-scoped":
		if value != "migration-only" {
			_, _ = os.Stderr.WriteString("wrong scoped secret " + value)
			os.Exit(12)
		}
		_, _ = os.Stdout.WriteString("migration received " + value)
	case "spawn-grandchild":
		if value == "" {
			os.Exit(14)
		}
		child := exec.Command(os.Args[0], "-test.run=TestCommandHelperProcess", "--", "grandchild")
		child.Env = os.Environ()
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			_, _ = os.Stderr.WriteString("start grandchild: " + err.Error())
			os.Exit(15)
		}
		pidFile := os.Getenv("COMMAND_HELPER_PID_FILE")
		if pidFile == "" {
			os.Exit(16)
		}
		parentReference := captureProcessReferenceForTest(os.Getpid())
		childReference := captureProcessReferenceForTest(child.Process.Pid)
		if err := os.WriteFile(
			pidFile,
			[]byte(formatProcessReferenceForTest(parentReference)+"\n"+formatProcessReferenceForTest(childReference)+"\n"),
			0o600,
		); err != nil {
			_, _ = os.Stderr.WriteString("write process identities: " + err.Error())
			os.Exit(17)
		}
		_, _ = os.Stdout.WriteString("parent received " + value + "\n")
		for {
			time.Sleep(time.Hour)
		}
	case "grandchild":
		if value == "" {
			os.Exit(18)
		}
		_, _ = os.Stderr.WriteString("grandchild received " + value + "\n")
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(13)
	}
	os.Exit(0)
}
