package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

// Options selects setup surfaces. Setup never uses production credentials.
type Options struct {
	DryRun        bool
	SkipFramework bool
	SkipFrontend  bool
	SkipDocs      bool
}

// Report records the exact setup plan and outcomes.
type Report struct {
	Project     string           `json:"project"`
	Root        string           `json:"root"`
	GeneratedAt time.Time        `json:"generatedAt"`
	DryRun      bool             `json:"dryRun"`
	Success     bool             `json:"success"`
	Steps       []command.Spec   `json:"steps"`
	Results     []command.Result `json:"results,omitempty"`
}

// Plan returns a stable, non-interactive setup sequence.
func Plan(ctx *project.Context, options Options) []command.Spec {
	steps := []command.Spec{
		{
			ID:          "go-root-dependencies",
			Description: "resolve root Go workspace dependencies",
			Directory:   ctx.Root,
			Args:        []string{"go", "list", "-deps", "./..."},
			Timeout:     10 * time.Minute,
		},
	}
	if !options.SkipFramework {
		steps = append(steps, command.Spec{
			ID:          "go-framework-dependencies",
			Description: "download reusable framework dependencies",
			Directory:   filepath.Join(ctx.Root, "mss-boot"),
			Args:        []string{"go", "mod", "download"},
			Environment: map[string]string{"GOWORK": "off"},
			Timeout:     10 * time.Minute,
		})
	}
	if !options.SkipFrontend {
		steps = append(steps, command.Spec{
			ID:          "frontend-dependencies",
			Description: "install frontend dependencies from the frozen lockfile",
			Directory:   filepath.Join(ctx.Root, "web", "antd"),
			Args:        []string{"pnpm", "install", "--frozen-lockfile"},
			Environment: map[string]string{"CI": "true"},
			Timeout:     20 * time.Minute,
		})
	}
	if !options.SkipDocs {
		steps = append(steps, command.Spec{
			ID:          "docs-dependencies",
			Description: "install documentation dependencies from the frozen lockfile",
			Directory:   filepath.Join(ctx.Root, "docs"),
			Args:        []string{"pnpm", "install", "--frozen-lockfile"},
			Environment: map[string]string{"CI": "true"},
			Timeout:     20 * time.Minute,
		})
	}
	return steps
}

// Run executes setup steps in order and stops after the first failure.
func Run(parent context.Context, ctx *project.Context, options Options) Report {
	report := Report{
		Project:     ctx.Project.Metadata.Name,
		Root:        ctx.Root,
		GeneratedAt: time.Now().UTC(),
		DryRun:      options.DryRun,
		Success:     true,
		Steps:       Plan(ctx, options),
	}
	if options.DryRun {
		return report
	}
	for _, step := range report.Steps {
		result := command.Run(parent, step)
		report.Results = append(report.Results, result)
		if result.ExitCode != 0 {
			report.Success = false
			break
		}
	}
	return report
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text renders the setup plan and results.
func (r Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss setup: %s\n", r.Project)
	fmt.Fprintf(&builder, "root: %s\n", r.Root)
	fmt.Fprintf(&builder, "dry-run: %t\n", r.DryRun)
	fmt.Fprintf(&builder, "success: %t\n\n", r.Success)
	resultByID := make(map[string]command.Result, len(r.Results))
	for _, result := range r.Results {
		resultByID[result.ID] = result
	}
	for _, step := range r.Steps {
		fmt.Fprintf(&builder, "- %s: %s\n", step.ID, command.Display(step.Args))
		if result, exists := resultByID[step.ID]; exists {
			fmt.Fprintf(&builder, "  exit=%d duration=%s\n", result.ExitCode, result.Duration.Round(time.Millisecond))
			if result.Error != "" {
				fmt.Fprintf(&builder, "  error=%s\n", result.Error)
			}
		}
	}
	return builder.String()
}

// EnsureReportDirectory creates the safe local report directory used by setup and verification.
func EnsureReportDirectory(ctx *project.Context) error {
	relative := ctx.Project.Spec.RepositoryLayout["reports"]
	if relative == "" {
		relative = ".mss/reports"
	}
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe reports path %q", relative)
	}
	return os.MkdirAll(filepath.Join(ctx.Root, filepath.FromSlash(relative)), 0o755)
}
