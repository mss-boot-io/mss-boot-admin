package app

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	featurecmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/feature"
)

func newFeatureCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "feature",
		Short: "Validate and plan cross-module Feature implementation",
	}
	command.AddCommand(newFeaturePlanCommand(rootOverride))
	return command
}

func newFeaturePlanCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "plan <feature.yaml>",
		Short: "Validate a Feature and its AdminModule contracts, then produce a read-only implementation plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			plan, planErr := featurecmd.Build(featurecmd.Options{
				Root:        projectContext.Root,
				FeaturePath: args[0],
			})
			if outputErr := writeFeaturePlan(cmd.OutOrStdout(), plan, format); outputErr != nil {
				return outputErr
			}
			return planErr
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func writeFeaturePlan(writer io.Writer, plan featurecmd.Plan, format string) error {
	switch format {
	case "text":
		_, err := io.WriteString(writer, plan.Text())
		return err
	case "json":
		data, err := plan.JSON()
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
