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
	DryRun                     bool
	SkipFramework              bool
	SkipFrontend               bool
	SkipDocs                   bool
	PromptInitialAdminPassword SecretPrompt
}

// SecretPrompt obtains a one-use secret without placing it in command
// arguments, generated files, or setup reports. Callers must return a fresh
// byte slice so Run can clear it after the migration attempt.
type SecretPrompt func() ([]byte, error)

const (
	initialAdminPasswordEnvironment = "MSS_ADMIN_INITIAL_PASSWORD"
	initialAdminCredentialsSentinel = "initial administrator credentials are required"
)

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
	thinHost := ctx.LayoutKind() == "thin-host"
	steps := make([]command.Spec, 0, 4)
	if thinHost {
		backendPath := strings.TrimSpace(ctx.Project.Spec.RepositoryLayout["backend"])
		if backendPath == "" {
			backendPath = "."
		}
		steps = append(steps, command.Spec{
			ID:          "go-backend-dependencies",
			Description: "download exact Thin Host backend dependencies",
			Directory:   filepath.Join(ctx.Root, filepath.FromSlash(backendPath)),
			Args:        []string{"go", "mod", "download"},
			Environment: map[string]string{"GOWORK": "off", "GOFLAGS": "-mod=readonly"},
			Timeout:     10 * time.Minute,
		})
		steps = append(steps, command.Spec{
			ID:          "go-backend-migrate",
			Description: "initialize or upgrade the local Thin Host database idempotently",
			Directory:   filepath.Join(ctx.Root, filepath.FromSlash(backendPath)),
			Args:        []string{"go", "run", "-mod=readonly", "./cmd/server", "migrate", "--config-provider", "fs"},
			Environment: map[string]string{"GOWORK": "off", "CONFIG_PROVIDER": "fs"},
			Timeout:     10 * time.Minute,
		})
	} else {
		steps = append(steps, command.Spec{
			ID:          "go-root-dependencies",
			Description: "resolve root Go workspace dependencies",
			Directory:   ctx.Root,
			Args:        []string{"go", "list", "-deps", "./..."},
			Timeout:     10 * time.Minute,
		})
	}
	if !thinHost && !options.SkipFramework {
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
		application, ok := ctx.DefaultFrontendApplication()
		if !ok {
			frontendPath := strings.TrimSpace(ctx.Project.Spec.RepositoryLayout["frontend"])
			if frontendPath == "" {
				frontendPath = "web/antd-v6"
			}
			application = project.FrontendApplicationSpec{
				ID:                    "frontend",
				Path:                  frontendPath,
				PackageManager:        "pnpm",
				PackageManagerVersion: ctx.Project.Spec.Frontend.PackageManagerVersion,
			}
		}
		packageManager := strings.TrimSpace(application.PackageManager)
		if packageManager == "" {
			packageManager = "pnpm"
		}
		packageManagerCommand := packageManager
		if version := strings.TrimSpace(application.PackageManagerVersion); version != "" {
			packageManagerCommand += "@" + version
		}
		steps = append(steps, command.Spec{
			ID:          "frontend-dependencies",
			Description: "install " + application.ID + " dependencies from the frozen lockfile",
			Directory:   filepath.Join(ctx.Root, filepath.FromSlash(application.Path)),
			Args:        []string{"corepack", packageManagerCommand, "install", "--frozen-lockfile"},
			Environment: map[string]string{"CI": "true"},
			Timeout:     20 * time.Minute,
		})
	}
	if !thinHost && !options.SkipDocs {
		steps = append(steps, command.Spec{
			ID:          "docs-dependencies",
			Description: "install documentation dependencies from the frozen lockfile",
			Directory:   filepath.Join(ctx.Root, "docs"),
			Args:        []string{"pnpm", "install", "--frozen-lockfile"},
			Environment: map[string]string{"CI": "true"},
			Timeout:     20 * time.Minute,
		})
	}
	for index := range steps {
		steps[index].UnsetEnvironment = []string{initialAdminPasswordEnvironment}
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
	inheritedSecret := []byte(os.Getenv(initialAdminPasswordEnvironment))
	defer clearBytes(inheritedSecret)
	hasInheritedSecret := len(inheritedSecret) > 0
	for _, step := range report.Steps {
		runtimeStep := step
		secretText := ""
		if step.ID == "go-backend-migrate" && hasInheritedSecret {
			runtimeStep.Environment = cloneEnvironment(step.Environment)
			secretText = string(inheritedSecret)
			runtimeStep.Environment[initialAdminPasswordEnvironment] = secretText
		}
		result := command.Run(parent, runtimeStep)
		if secretText != "" {
			delete(runtimeStep.Environment, initialAdminPasswordEnvironment)
			redactCommandResult(&result, secretText)
			secretText = ""
		}
		if shouldPromptForInitialAdminPassword(step, result, options, hasInheritedSecret) {
			result = retryMigrationWithPromptedPassword(parent, step, result, options.PromptInitialAdminPassword)
		}
		report.Results = append(report.Results, result)
		if result.ExitCode != 0 {
			report.Success = false
			break
		}
	}
	return report
}

func shouldPromptForInitialAdminPassword(
	step command.Spec,
	result command.Result,
	options Options,
	hasInheritedSecret bool,
) bool {
	if options.PromptInitialAdminPassword == nil || step.ID != "go-backend-migrate" || result.ExitCode == 0 {
		return false
	}
	if hasInheritedSecret {
		return false
	}
	return strings.Contains(result.Stderr, initialAdminCredentialsSentinel)
}

func retryMigrationWithPromptedPassword(
	parent context.Context,
	step command.Spec,
	initialResult command.Result,
	prompt SecretPrompt,
) command.Result {
	secret, err := prompt()
	if err != nil {
		initialResult.Error = "read hidden initial administrator password: " + err.Error()
		return initialResult
	}
	defer clearBytes(secret)
	if len(secret) == 0 {
		initialResult.Error = "initial administrator password cannot be empty"
		return initialResult
	}

	retry := step
	retry.Environment = cloneEnvironment(step.Environment)
	password := string(secret)
	retry.Environment[initialAdminPasswordEnvironment] = password
	result := command.Run(parent, retry)
	delete(retry.Environment, initialAdminPasswordEnvironment)
	redactCommandResult(&result, password)
	password = ""
	return result
}

func cloneEnvironment(environment map[string]string) map[string]string {
	clone := make(map[string]string, len(environment)+1)
	for key, value := range environment {
		clone[key] = value
	}
	return clone
}

func redactCommandResult(result *command.Result, secret string) {
	if result == nil || secret == "" {
		return
	}
	const redacted = "[REDACTED]"
	result.Stdout = strings.ReplaceAll(result.Stdout, secret, redacted)
	result.Stderr = strings.ReplaceAll(result.Stderr, secret, redacted)
	result.Error = strings.ReplaceAll(result.Error, secret, redacted)
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
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
