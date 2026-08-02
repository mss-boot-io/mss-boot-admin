package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

// Status is the outcome of one environment check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusInfo Status = "info"
)

// Check records one deterministic doctor result.
type Check struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Required    bool   `json:"required"`
	Detail      string `json:"detail,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// Report is emitted in text or JSON form for humans and agents.
type Report struct {
	Project     string    `json:"project"`
	Root        string    `json:"root"`
	GeneratedAt time.Time `json:"generatedAt"`
	Platform    string    `json:"platform"`
	Ready       bool      `json:"ready"`
	Checks      []Check   `json:"checks"`
}

// Run executes environment and repository checks without mutating the workspace.
func Run(ctx context.Context, projectContext *project.Context) Report {
	report := Report{
		Project:     projectContext.Project.Metadata.Name,
		Root:        projectContext.Root,
		GeneratedAt: time.Now().UTC(),
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Ready:       true,
	}

	report.Checks = append(report.Checks,
		fileCheck(projectContext.Root, ".mss/project.yaml", true),
		fileCheck(projectContext.Root, ".mss/capabilities.yaml", true),
		fileCheck(projectContext.Root, ".mss/commands.yaml", true),
		fileCheck(projectContext.Root, "go.mod", true),
		fileCheck(projectContext.Root, "go.work", true),
		fileCheck(projectContext.Root, "web/antd/pnpm-lock.yaml", true),
		fileCheck(projectContext.Root, "docs/pnpm-lock.yaml", true),
	)

	report.Checks = append(report.Checks,
		toolCheck(ctx, "git", true, "git", "--version"),
		toolCheck(ctx, "go", true, "go", "version"),
		toolCheck(ctx, "node", true, "node", "--version"),
		toolCheck(ctx, "pnpm", true, "pnpm", "--version"),
		toolCheck(ctx, "docker", false, "docker", "--version"),
	)

	report.Checks = append(report.Checks,
		portCheck("backend-port", 8080),
		portCheck("frontend-port", 8000),
		portCheck("redis-port", 6379),
	)

	for _, check := range report.Checks {
		if check.Required && check.Status == StatusFail {
			report.Ready = false
			break
		}
	}

	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text returns a compact human-readable report.
func (r Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss doctor: %s\n", r.Project)
	fmt.Fprintf(&builder, "root: %s\n", r.Root)
	fmt.Fprintf(&builder, "platform: %s\n", r.Platform)
	fmt.Fprintf(&builder, "ready: %t\n\n", r.Ready)
	for _, check := range r.Checks {
		required := "optional"
		if check.Required {
			required = "required"
		}
		fmt.Fprintf(&builder, "[%s] %-24s (%s)", strings.ToUpper(string(check.Status)), check.Name, required)
		if check.Detail != "" {
			fmt.Fprintf(&builder, ": %s", check.Detail)
		}
		builder.WriteByte('\n')
		if check.Remediation != "" && check.Status != StatusPass {
			fmt.Fprintf(&builder, "       remediation: %s\n", check.Remediation)
		}
	}
	return builder.String()
}

func fileCheck(root, relative string, required bool) Check {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Stat(path)
	check := Check{
		ID:       "file:" + relative,
		Name:     relative,
		Required: required,
	}
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
		check.Remediation = "restore the tracked repository file"
		return check
	}
	if !info.Mode().IsRegular() {
		check.Status = StatusFail
		check.Detail = "path exists but is not a regular file"
		check.Remediation = "replace the path with the expected tracked file"
		return check
	}
	check.Status = StatusPass
	check.Detail = fmt.Sprintf("%d bytes", info.Size())
	return check
}

func toolCheck(parent context.Context, id string, required bool, executable string, args ...string) Check {
	check := Check{
		ID:       "tool:" + id,
		Name:     executable,
		Required: required,
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		if required {
			check.Status = StatusFail
		} else {
			check.Status = StatusWarn
		}
		check.Detail = "not found in PATH"
		check.Remediation = "install " + executable + " or add it to PATH"
		return check
	}

	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		check.Status = StatusFail
		check.Detail = "version check timed out"
		check.Remediation = "verify the executable can start without interaction"
		return check
	}
	if err != nil {
		if required {
			check.Status = StatusFail
		} else {
			check.Status = StatusWarn
		}
		check.Detail = strings.TrimSpace(string(output))
		if check.Detail == "" {
			check.Detail = err.Error()
		}
		check.Remediation = "repair or reinstall " + executable
		return check
	}

	check.Status = StatusPass
	check.Detail = firstLine(strings.TrimSpace(string(output)))
	if check.Detail == "" {
		check.Detail = path
	}
	return check
}

func portCheck(id string, port int) Check {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return Check{
			ID:       "port:" + id,
			Name:     address,
			Status:   StatusInfo,
			Required: false,
			Detail:   "already in use or unavailable: " + err.Error(),
		}
	}
	_ = listener.Close()
	return Check{
		ID:       "port:" + id,
		Name:     address,
		Status:   StatusInfo,
		Required: false,
		Detail:   "available",
	}
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
