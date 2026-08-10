package app

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/testevidence"
)

type testEvidenceRunner func(context.Context, testevidence.Options) (testevidence.Report, error)

func newTestCommand(rootOverride *string) *cobra.Command {
	return newTestCommandWithRunner(rootOverride, testevidence.Run)
}

func newTestCommandWithRunner(rootOverride *string, runner testEvidenceRunner) *cobra.Command {
	command := &cobra.Command{
		Use:   "test",
		Short: "Run test commands that emit machine-verifiable evidence",
	}
	command.AddCommand(newTestEvidenceCommand(rootOverride, runner))
	return command
}

func newTestEvidenceCommand(rootOverride *string, runner testEvidenceRunner) *cobra.Command {
	var options testevidence.Options
	options.GoWork = testevidence.GoWorkAuto
	command := &cobra.Command{
		Use:   "evidence",
		Short: "Require exact, uncached go test -json evidence",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			options.Root = projectContext.Root
			report, runErr := runner(cmd.Context(), options)
			if outputErr := writeTestEvidence(cmd.OutOrStdout(), report); outputErr != nil {
				return outputErr
			}
			return runErr
		},
	}
	command.Flags().StringVar(&options.Directory, "directory", "", "repository-relative Go module working directory")
	command.Flags().StringVar(&options.Package, "package", "", "single repository-relative Go package ('.' or './path'; no patterns)")
	command.Flags().StringVar(&options.Run, "run", "", "fully anchored Go test regular expression")
	command.Flags().IntVar(&options.Count, "count", 1, "required run/pass count for every named test")
	command.Flags().BoolVar(&options.Race, "race", false, "enable the Go race detector")
	command.Flags().StringVar(&options.GoWork, "go-work", testevidence.GoWorkAuto, "Go workspace mode: auto or off")
	command.Flags().StringArrayVar(&options.Required, "require", nil, "required top-level Test name; repeat for multiple tests")
	return command
}

func writeTestEvidence(writer io.Writer, report testevidence.Report) error {
	data, err := report.JSON()
	if err != nil {
		return err
	}
	return writeLine(writer, data)
}
