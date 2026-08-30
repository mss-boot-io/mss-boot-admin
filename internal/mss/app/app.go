package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/doctor"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	setupcmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/setup"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/verify"
)

// Execute runs the complete agent-facing mss CLI.
func Execute() error {
	return ExecuteAgent()
}

// NewRootCommand returns the same complete command tree used by cmd/mss.
func NewRootCommand() *cobra.Command {
	return NewAgentRootCommand()
}

func newContextCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "context",
		Short: "Print normalized project context for humans or agents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				data, err := ctx.JSON()
				if err != nil {
					return err
				}
				return writeLine(cmd.OutOrStdout(), data)
			case "text":
				_, err := io.WriteString(cmd.OutOrStdout(), contextText(ctx))
				return err
			default:
				return fmt.Errorf("unsupported output format %q", format)
			}
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func newDoctorCommand(rootOverride *string) *cobra.Command {
	var format string
	var strict bool
	var componentNames []string
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Check repository contracts and local toolchain readiness",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			components, err := doctor.ParseComponents(componentNames)
			if err != nil {
				return err
			}
			options := make([]doctor.Option, 0, 1)
			if len(components) > 0 {
				options = append(options, doctor.WithComponents(components...))
			}
			report := doctor.Run(cmd.Context(), ctx, options...)
			if err := writeDoctor(cmd.OutOrStdout(), report, format); err != nil {
				return err
			}
			if strict && !report.Ready {
				return errors.New("required development tools or repository files are missing")
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().BoolVar(&strict, "strict", false, "return a non-zero exit code when required checks fail")
	command.Flags().StringSliceVar(&componentNames, "component", nil, "limit checks to components: backend, framework, frontend, docs, or agent")
	return command
}

func newSetupCommand(rootOverride *string) *cobra.Command {
	var format string
	var options setupcmd.Options
	command := &cobra.Command{
		Use:   "setup",
		Short: "Install safe local development dependencies idempotently",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			if !options.DryRun {
				if err := setupcmd.EnsureReportDirectory(ctx); err != nil {
					return err
				}
				options.PromptInitialAdminPassword = terminalSecretPrompt(cmd.InOrStdin(), cmd.ErrOrStderr())
			}
			report := setupcmd.Run(cmd.Context(), ctx, options)
			if err := writeSetup(cmd.OutOrStdout(), report, format); err != nil {
				return err
			}
			if !report.Success {
				return errors.New("setup failed")
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().BoolVar(&options.DryRun, "dry-run", false, "print the setup plan without executing it")
	command.Flags().BoolVar(&options.SkipFramework, "skip-framework", false, "skip framework dependency download")
	command.Flags().BoolVar(&options.SkipFrontend, "skip-frontend", false, "skip frontend dependency install")
	command.Flags().BoolVar(&options.SkipDocs, "skip-docs", false, "skip docs dependency install")
	return command
}

func terminalSecretPrompt(input io.Reader, output io.Writer) setupcmd.SecretPrompt {
	file, ok := input.(*os.File)
	if !ok {
		return nil
	}
	return terminalSecretPromptWith(file, output, term.IsTerminal, term.ReadPassword)
}

func terminalSecretPromptWith(
	input *os.File,
	output io.Writer,
	isTerminal func(int) bool,
	readPassword func(int) ([]byte, error),
) setupcmd.SecretPrompt {
	if input == nil || output == nil || !isTerminal(int(input.Fd())) {
		return nil
	}
	return func() ([]byte, error) {
		if _, err := io.WriteString(output, "Initial local administrator password: "); err != nil {
			return nil, err
		}
		password, err := readPassword(int(input.Fd()))
		if _, newlineErr := io.WriteString(output, "\n"); err == nil && newlineErr != nil {
			err = newlineErr
		}
		return password, err
	}
}

func newModuleCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "module",
		Short: "Generate and inspect vertical management modules",
	}
	command.AddCommand(newModuleGenerateCommand(rootOverride))
	return command
}

func newModuleGenerateCommand(rootOverride *string) *cobra.Command {
	var format string
	var write bool
	var check bool
	var frontendTarget string
	command := &cobra.Command{
		Use:   "generate <module-spec.yaml>",
		Short: "Render a deterministic vertical module from an AdminModule specification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if write && check {
				return errors.New("--write and --check are mutually exclusive")
			}
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			path := resolveInputPath(ctx.Root, args[0])
			module, err := spec.LoadModule(path)
			if err != nil {
				return err
			}
			if relative, relErr := filepath.Rel(ctx.Root, path); relErr == nil {
				module.SourcePath = filepath.ToSlash(relative)
			}
			plan, err := generator.Generate(module, generator.Options{
				Root:           ctx.Root,
				Write:          write,
				Check:          check,
				FrontendTarget: frontendTarget,
				Project:        &ctx.Project,
			})
			if outputErr := writeGeneration(cmd.OutOrStdout(), plan, format); outputErr != nil {
				return outputErr
			}
			return err
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().BoolVar(&write, "write", false, "write generated files; default is dry-run")
	command.Flags().BoolVar(&check, "check", false, "fail when generated output is missing or stale")
	command.Flags().StringVar(&frontendTarget, "frontend-target", spec.FrontendTargetAntDV6, "generated frontend target: antd-v6")
	return command
}

func newVerifyCommand(rootOverride *string) *cobra.Command {
	var format string
	var changed bool
	var all bool
	var module string
	var baseRef string
	var planOnly bool
	var releaseEvidence bool
	var expectedCommit string
	command := &cobra.Command{
		Use:   "verify",
		Short: "Plan and execute change-aware repository validation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected := 0
			if changed {
				selected++
			}
			if all {
				selected++
			}
			if module != "" {
				selected++
			}
			if selected > 1 {
				return errors.New("--changed, --all, and --module are mutually exclusive")
			}
			mode := verify.ModeChanged
			if all {
				mode = verify.ModeAll
			} else if module != "" {
				mode = verify.ModeModule
			}
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			report, runErr := verify.Run(cmd.Context(), ctx, verify.Options{
				Mode:            mode,
				BaseRef:         baseRef,
				Module:          module,
				PlanOnly:        planOnly,
				ReleaseEvidence: releaseEvidence,
				ExpectedCommit:  expectedCommit,
			})
			if runErr != nil && report.GeneratedAt.IsZero() {
				return runErr
			}
			if outputErr := writeVerify(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return runErr
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text, json, or markdown")
	command.Flags().BoolVar(&changed, "changed", false, "validate the changed-file set; this is the default")
	command.Flags().BoolVar(&all, "all", false, "run the complete validation suite")
	command.Flags().StringVar(&module, "module", "", "validate one vertical module")
	command.Flags().StringVar(&baseRef, "base", "", "Git base ref for changed-file detection")
	command.Flags().BoolVar(&planOnly, "plan", false, "write reports without executing external checks")
	command.Flags().BoolVar(&releaseEvidence, "release-evidence", false, "bind complete local verification evidence to one exact clean commit")
	command.Flags().StringVar(&expectedCommit, "expect-commit", "", "full lowercase commit SHA required by --release-evidence")
	return command
}

func loadProject(rootOverride string) (*project.Context, error) {
	start := rootOverride
	if start == "" {
		start = "."
	}
	return project.Load(start)
}

func contextText(ctx *project.Context) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "project: %s\n", ctx.Project.Metadata.Name)
	fmt.Fprintf(&builder, "display-name: %s\n", ctx.Project.Metadata.DisplayName)
	fmt.Fprintf(&builder, "root: %s\n", ctx.Root)
	fmt.Fprintf(&builder, "foundation-version: %s\n", ctx.Project.Spec.FoundationVersion)
	if !ctx.Project.Spec.Distribution.Empty() {
		fmt.Fprintf(&builder, "admin-distribution: %s@%s\n", ctx.Project.Spec.Distribution.Name, ctx.Project.Spec.Distribution.Version)
		fmt.Fprintf(&builder, "admin-backend: %s@%s\n", ctx.Project.Spec.Distribution.Backend.Module, ctx.Project.Spec.Distribution.Backend.Version)
		fmt.Fprintf(&builder, "admin-frontend: %s@%s\n", ctx.Project.Spec.Distribution.Frontend.Package, ctx.Project.Spec.Distribution.Frontend.Version)
	}
	fmt.Fprintf(&builder, "backend-module: %s\n", ctx.Project.Spec.Backend.Module)
	fmt.Fprintf(&builder, "framework-module: %s\n", ctx.Project.Spec.Backend.FrameworkModule)
	fmt.Fprintf(&builder, "frontend: %s + %s\n", ctx.Project.Spec.Frontend.Framework, ctx.Project.Spec.Frontend.ComponentLibrary)
	builder.WriteString("\ncapabilities:\n")
	for _, capability := range ctx.Capabilities.Spec.Capabilities {
		fmt.Fprintf(&builder, "- %s [%s]: %s\n", capability.ID, capability.Status, capability.DisplayName)
	}
	builder.WriteString("\ncommands:\n")
	for _, name := range ctx.CommandNames() {
		entry := ctx.Commands.Spec.Commands[name]
		fmt.Fprintf(&builder, "- %s: %s\n", name, entry.Command)
	}
	return builder.String()
}

func writeDoctor(writer io.Writer, report doctor.Report, format string) error {
	if format == "text" {
		_, err := io.WriteString(writer, report.Text())
		return err
	}
	if format == "json" {
		data, err := report.JSON()
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	}
	return fmt.Errorf("unsupported output format %q", format)
}

func writeSetup(writer io.Writer, report setupcmd.Report, format string) error {
	if format == "text" {
		_, err := io.WriteString(writer, report.Text())
		return err
	}
	if format == "json" {
		data, err := report.JSON()
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	}
	return fmt.Errorf("unsupported output format %q", format)
}

func writeGeneration(writer io.Writer, plan generator.Plan, format string) error {
	if format == "text" {
		_, err := io.WriteString(writer, plan.Text())
		return err
	}
	if format == "json" {
		data, err := plan.JSON()
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	}
	return fmt.Errorf("unsupported output format %q", format)
}

func writeVerify(writer io.Writer, report verify.Report, format string) error {
	switch format {
	case "text":
		_, err := fmt.Fprintf(writer, "verification success: %t\nreport: .mss/reports/verify.md\n", report.Success)
		return err
	case "markdown":
		_, err := io.WriteString(writer, report.Markdown())
		return err
	case "json":
		data, err := report.JSON()
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeSpecResults(writer io.Writer, results any, format string) error {
	if format == "json" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	}
	if format != "text" {
		return fmt.Errorf("unsupported output format %q", format)
	}
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	var decoded []struct {
		Path   string       `json:"path"`
		Module string       `json:"module"`
		Valid  bool         `json:"valid"`
		Issues []spec.Issue `json:"issues"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	for _, result := range decoded {
		if result.Valid {
			if _, err := fmt.Fprintf(writer, "PASS %s (%s)\n", result.Path, result.Module); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(writer, "FAIL %s\n", result.Path); err != nil {
			return err
		}
		for _, issue := range result.Issues {
			if _, err := fmt.Fprintf(writer, "  - %s [%s]: %s\n", issue.Path, issue.Code, issue.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveInputPath(root, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, filepath.FromSlash(value))
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func writeLine(writer io.Writer, data []byte) error {
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		_, err := writer.Write([]byte{'\n'})
		return err
	}
	return nil
}

// ExecuteContext is used by tests and future protocol adapters.
func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	return root.ExecuteContext(ctx)
}

// SortedKeys provides stable map rendering for protocol adapters.
func SortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
