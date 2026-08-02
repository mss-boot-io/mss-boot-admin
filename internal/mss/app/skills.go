package app

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	skillcontract "github.com/mss-boot-io/mss-boot-admin/internal/mss/skills"
)

func newSkillsCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "skills",
		Short: "Discover and validate repository-local Agent Skills",
	}
	command.AddCommand(newSkillsListCommand(rootOverride))
	command.AddCommand(newSkillsValidateCommand(rootOverride))
	return command
}

func newSkillsListCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "list",
		Short: "List repository-local Agent Skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			report, discoverErr := skillcontract.Discover(ctx.Root)
			if outputErr := writeSkills(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return discoverErr
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func newSkillsValidateCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate skill front matter, naming, portability, and instruction bodies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			report, discoverErr := skillcontract.Discover(ctx.Root)
			if outputErr := writeSkills(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return discoverErr
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func writeSkills(writer io.Writer, report skillcontract.Report, format string) error {
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
