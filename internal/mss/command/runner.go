package command

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
	Timeout     time.Duration     `json:"-"`
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

// Run executes a command directly. Spec.Args[0] is the executable; no shell is involved.
func Run(parent context.Context, spec Spec) Result {
	result := Result{
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

	cmd := exec.CommandContext(ctx, spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Directory
	cmd.Env = mergeEnvironment(os.Environ(), spec.Environment)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
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
	if errorsAs(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	}
	return result
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

func mergeEnvironment(base []string, overrides map[string]string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

// errorsAs is a small seam that keeps Result construction easy to test.
func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}
