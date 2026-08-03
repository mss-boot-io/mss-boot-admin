package app

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	evalcmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/eval"
)

func newEvalCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "eval",
		Short: "List and run deterministic Agent foundation evaluations",
	}
	command.AddCommand(newEvalListCommand(rootOverride))
	command.AddCommand(newEvalRunCommand(rootOverride))
	command.AddCommand(newEvalReportCommand(rootOverride))
	return command
}

func newEvalListCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "list",
		Short: "List available Agent evaluation scenarios",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			catalog, err := evalcmd.Load(projectContext.Root)
			if err != nil {
				return err
			}
			cases, err := catalog.List(nil)
			if err != nil {
				return err
			}
			switch format {
			case "text":
				_, err = io.WriteString(cmd.OutOrStdout(), evalcmd.ListText(cases))
				return err
			case "json":
				data, err := json.MarshalIndent(cases, "", "  ")
				if err != nil {
					return err
				}
				return writeLine(cmd.OutOrStdout(), data)
			default:
				return fmt.Errorf("unsupported output format %q", format)
			}
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func newEvalRunCommand(rootOverride *string) *cobra.Command {
	var all bool
	var format string
	command := &cobra.Command{
		Use:   "run [case...]",
		Short: "Run selected evaluations; no case means the complete catalog",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("--all cannot be combined with explicit case IDs")
			}
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			if all {
				args = nil
			}
			report, runErr := evalcmd.Run(cmd.Context(), projectContext.Root, args)
			if outputErr := writeEvalReport(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return runErr
		},
	}
	command.Flags().BoolVar(&all, "all", false, "run all catalog cases")
	command.Flags().StringVar(&format, "format", "text", "output format: text, markdown, or json")
	return command
}

func newEvalReportCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "report",
		Short: "Read the latest persisted Agent evaluation report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			report, err := evalcmd.ReadLatest(projectContext.Root)
			if err != nil {
				return err
			}
			return writeEvalReport(cmd.OutOrStdout(), report, format)
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text, markdown, or json")
	return command
}

func writeEvalReport(writer io.Writer, report evalcmd.Report, format string) error {
	switch format {
	case "text", "markdown":
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
