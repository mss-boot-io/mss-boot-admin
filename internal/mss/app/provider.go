package app

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/provider"
)

type providerEvidenceRunner func(context.Context, string, provider.Options) (provider.Report, error)

func newProviderCommand(rootOverride *string) *cobra.Command {
	return newProviderCommandWithRunner(rootOverride, func(_ context.Context, root string, options provider.Options) (provider.Report, error) {
		return provider.Run(root, options)
	})
}

func newProviderCommandWithRunner(rootOverride *string, runner providerEvidenceRunner) *cobra.Command {
	command := &cobra.Command{
		Use:   "provider",
		Short: "Inspect provider-specific runtime evidence",
	}
	command.AddCommand(newProviderEvidenceCommand(rootOverride, runner))
	return command
}

func newProviderEvidenceCommand(rootOverride *string, runner providerEvidenceRunner) *cobra.Command {
	options := provider.Options{Input: provider.DefaultInput}
	var format string
	command := &cobra.Command{
		Use:   "evidence",
		Short: "Validate a deterministic provider maturity report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			report, runErr := runner(cmd.Context(), projectContext.Root, options)
			if outputErr := writeProviderEvidence(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return runErr
		},
	}
	command.Flags().StringVar(&options.Input, "input", provider.DefaultInput, "repository-relative provider evidence JSON path")
	command.Flags().BoolVar(&options.Required, "required", false, "reject empty, zero-hit, skipped, failed, partial, or cached-only required evidence")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func writeProviderEvidence(writer io.Writer, report provider.Report, format string) error {
	switch format {
	case "text":
		_, err := io.WriteString(writer, report.Text())
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
