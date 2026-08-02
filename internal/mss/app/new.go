package app

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
)

func newNewCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "new",
		Short: "Create downstream projects from versioned foundation blueprints",
	}
	command.AddCommand(newApplicationCommand(rootOverride))
	return command
}

func newApplicationCommand(rootOverride *string) *cobra.Command {
	var displayName string
	var module string
	var repository string
	var blueprintName string
	var destination string
	var write bool
	var initializeGit bool
	var format string
	command := &cobra.Command{
		Use:   "app <name>",
		Short: "Plan or create a complete agent-native management-system repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			if module == "" {
				return fmt.Errorf("--module is required")
			}
			plan, generateErr := blueprint.Generate(cmd.Context(), blueprint.Options{
				FoundationRoot: projectContext.Root,
				Blueprint:      blueprintName,
				Destination:    destination,
				Application: blueprint.Application{
					Name:        args[0],
					DisplayName: displayName,
					Module:      module,
					Repository:  repository,
				},
				Write:         write,
				InitializeGit: initializeGit,
			})
			if outputErr := writeBlueprintPlan(cmd.OutOrStdout(), plan, format); outputErr != nil {
				return outputErr
			}
			return generateErr
		},
	}
	command.Flags().StringVar(&displayName, "display-name", "", "human-readable project name; derived from name when omitted")
	command.Flags().StringVar(&module, "module", "", "target Go module, for example github.com/acme/customer-admin")
	command.Flags().StringVar(&repository, "repository", "", "target GitHub repository in owner/name form; inferred from github.com module when omitted")
	command.Flags().StringVar(&blueprintName, "blueprint", "management-system", "foundation blueprint name")
	command.Flags().StringVar(&destination, "destination", "", "target directory; defaults to .mss/output/<name>")
	command.Flags().BoolVar(&write, "write", false, "write files after a conflict-free plan; default is dry-run")
	command.Flags().BoolVar(&initializeGit, "git-init", false, "initialize a new Git repository after files are written")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func writeBlueprintPlan(writer io.Writer, plan blueprint.Plan, format string) error {
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
