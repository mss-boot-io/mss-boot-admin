package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Spec is one validated external command invocation.
type Spec struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Directory   string            `json:"directory"`
	Args        []string          `json:"args"`
	Environment map[string]string `json:"environment,omitempty"`
	// UnsetEnvironment removes inherited variables before explicit Environment
	// overrides are applied. It is used for one-process secret scoping without
	// mutating the parent process environment.
	UnsetEnvironment []string      `json:"unsetEnvironment,omitempty"`
	Timeout          time.Duration `json:"-"`
}

// Result captures a command outcome without relying on shell parsing.
type Result struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Directory   string        `json:"directory"`
	Args        []string      `json:"args"`
	StartedAt   time.Time     `json:"startedAt"`
	Duration    time.Duration `json:"duration"`
	ExitCode    int           `json:"exitCode"`
	Stdout      string        `json:"stdout,omitempty"`
	Stderr      string        `json:"stderr,omitempty"`
	Error       string        `json:"error,omitempty"`
}

var defaultSensitiveEnvironmentKeys = []string{"MSS_ADMIN_INITIAL_PASSWORD"}

var ErrCommandCleanupTimeout = errors.New("command cleanup timed out")

// Run executes a command directly. Spec.Args[0] is the executable; no shell is involved.
func Run(parent context.Context, spec Spec) (result Result) {
	baseEnvironment := os.Environ()
	sensitiveValues := sensitiveEnvironmentValues(baseEnvironment, spec.Environment)
	defer func() {
		redactResult(&result, sensitiveValues)
	}()
	result = Result{
		ID:          spec.ID,
		Description: spec.Description,
		Directory:   spec.Directory,
		Args:        append([]string(nil), spec.Args...),
		StartedAt:   time.Now().UTC(),
		ExitCode:    -1,
	}
	started := time.Now()
	defer func() {
		result.Duration = time.Since(started)
	}()

	if len(spec.Args) == 0 || strings.TrimSpace(spec.Args[0]) == "" {
		result.Error = "command executable is required"
		return result
	}
	if spec.Directory == "" {
		result.Error = "command working directory is required"
		return result
	}

	ctx := parent
	cancel := func() {}
	if spec.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, spec.Timeout)
	}
	defer cancel()

	cmd := exec.Command(spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Directory
	unsetEnvironment := append([]string(nil), defaultSensitiveEnvironmentKeys...)
	unsetEnvironment = append(unsetEnvironment, spec.UnsetEnvironment...)
	cmd.Env = mergeEnvironment(baseEnvironment, spec.Environment, unsetEnvironment)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := runManagedCommand(ctx, cmd)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	if err == nil {
		result.ExitCode = 0
		return result
	}
	if ctx.Err() != nil {
		result.Error = ctx.Err().Error()
	} else {
		result.Error = err.Error()
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	}
	return result
}

const (
	commandTerminationGrace = 2 * time.Second
	commandKillWait         = 5 * time.Second
)

func runManagedCommand(ctx context.Context, command *exec.Cmd) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	processTree, err := newProcessTree(command)
	if err != nil {
		return fmt.Errorf("prepare command process tree: %w", err)
	}
	defer func() {
		result = errors.Join(result, processTree.close())
	}()

	// Bound the final descriptor-copy wait if a broken descendant escaped while
	// retaining stdout or stderr. The managed tree is still terminated first.
	command.WaitDelay = commandKillWait
	if err := command.Start(); err != nil {
		return err
	}
	if err := processTree.afterStart(); err != nil {
		terminateErr := processTree.terminate()
		killErr := processTree.kill()
		waited := make(chan error, 1)
		go func() {
			waited <- command.Wait()
		}()
		waitErr, cleanupErr := waitForCommandCleanup(waited, commandKillWait)
		return errors.Join(
			fmt.Errorf("attach command process tree: %w", err),
			terminateErr,
			killErr,
			waitErr,
			cleanupErr,
		)
	}

	waited := make(chan error, 1)
	go func() {
		waited <- command.Wait()
	}()
	select {
	case err := <-waited:
		return err
	case <-ctx.Done():
	}

	terminationErr := processTree.terminate()
	timer := time.NewTimer(commandTerminationGrace)
	defer timer.Stop()
	select {
	case <-waited:
		return errors.Join(ctx.Err(), terminationErr)
	case <-timer.C:
	}

	killErr := processTree.kill()
	// kill() is required to terminate the direct process as a final fallback;
	// WaitDelay bounds inherited output handles after that process exits.
	waitErr, cleanupErr := waitForCommandCleanup(waited, commandKillWait)
	if waitErr != nil {
		var exitError *exec.ExitError
		if !errors.As(waitErr, &exitError) {
			killErr = errors.Join(killErr, waitErr)
		}
	}
	return errors.Join(ctx.Err(), terminationErr, killErr, cleanupErr)
}

func waitForCommandCleanup(waited <-chan error, timeout time.Duration) (error, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("%w: cleanup timeout must be positive", ErrCommandCleanupTimeout)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-waited:
		return err, nil
	case <-timer.C:
		return nil, fmt.Errorf("%w after %s", ErrCommandCleanupTimeout, timeout)
	}
}

// Display joins arguments for diagnostics only. It is never passed to a shell.
func Display(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'") {
			quoted = append(quoted, fmt.Sprintf("%q", arg))
		} else {
			quoted = append(quoted, arg)
		}
	}
	return strings.Join(quoted, " ")
}

func mergeEnvironment(base []string, overrides map[string]string, unset []string) []string {
	type environmentEntry struct {
		key   string
		value string
	}
	values := make(map[string]environmentEntry, len(base)+len(overrides))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[normalizedEnvironmentKey(key)] = environmentEntry{key: key, value: value}
		}
	}
	for _, key := range unset {
		delete(values, normalizedEnvironmentKey(key))
	}
	for key, value := range overrides {
		values[normalizedEnvironmentKey(key)] = environmentEntry{key: key, value: value}
	}
	entries := make([]environmentEntry, 0, len(values))
	for _, entry := range values {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return normalizedEnvironmentKey(entries[i].key) < normalizedEnvironmentKey(entries[j].key)
	})
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.key+"="+entry.value)
	}
	return result
}

func normalizedEnvironmentKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func sensitiveEnvironmentValues(base []string, overrides map[string]string) []string {
	sensitiveKeys := make(map[string]struct{}, len(defaultSensitiveEnvironmentKeys))
	for _, key := range defaultSensitiveEnvironmentKeys {
		sensitiveKeys[normalizedEnvironmentKey(key)] = struct{}{}
	}
	unique := make(map[string]struct{})
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || value == "" {
			continue
		}
		if _, sensitive := sensitiveKeys[normalizedEnvironmentKey(key)]; sensitive {
			unique[value] = struct{}{}
		}
	}
	for key, value := range overrides {
		if value == "" {
			continue
		}
		if _, sensitive := sensitiveKeys[normalizedEnvironmentKey(key)]; sensitive {
			unique[value] = struct{}{}
		}
	}
	values := make([]string, 0, len(unique))
	for value := range unique {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	return values
}

func redactResult(result *Result, sensitiveValues []string) {
	if result == nil {
		return
	}
	const redacted = "[REDACTED]"
	for _, value := range sensitiveValues {
		if value == "" {
			continue
		}
		result.Stdout = strings.ReplaceAll(result.Stdout, value, redacted)
		result.Stderr = strings.ReplaceAll(result.Stderr, value, redacted)
		result.Error = strings.ReplaceAll(result.Error, value, redacted)
	}
}
