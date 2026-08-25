package app

import (
	"fmt"
	"io"
	"os"
	"strings"

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
	var foundation string
	var frontendRegistry string
	var write bool
	var initializeGit bool
	var format string
	command := &cobra.Command{
		Use:   "app <name>",
		Short: "Plan or create a complete agent-native management-system repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if module == "" {
				return fmt.Errorf("--module is required")
			}
			options := blueprint.Options{
				Blueprint:   blueprintName,
				Destination: destination,
				Application: blueprint.Application{
					Name:        args[0],
					DisplayName: displayName,
					Module:      module,
					Repository:  repository,
				},
				Write:               write,
				InitializeGit:       initializeGit,
				FrontendRegistryURL: frontendRegistry,
			}
			plan, generateErr := generateApplication(cmd, rootOverride, foundation, options)
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
	command.Flags().StringVar(&foundation, "foundation", "", "clean Foundation checkout override for contributor development")
	command.Flags().StringVar(&frontendRegistry, "contributor-npm-registry", "", "loopback npm registry override for contributor qualification")
	_ = command.Flags().MarkHidden("contributor-npm-registry")
	command.Flags().BoolVar(&write, "write", false, "write files after a conflict-free plan; default is dry-run")
	command.Flags().BoolVar(&initializeGit, "git-init", false, "initialize a new Git repository after files are written")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func generateApplication(command *cobra.Command, rootOverride *string, foundation string, options blueprint.Options) (blueprint.Plan, error) {
	foundation = strings.TrimSpace(foundation)
	if foundation != "" {
		context, err := loadProject(foundation)
		if err != nil {
			return blueprint.Plan{}, fmt.Errorf("load contributor Foundation checkout: %w", err)
		}
		options.FoundationRoot = context.Root
		return blueprint.Generate(command.Context(), options)
	}
	if rootOverride != nil && strings.TrimSpace(*rootOverride) != "" {
		context, err := loadProject(*rootOverride)
		if err != nil {
			return blueprint.Plan{}, err
		}
		options.FoundationRoot = context.Root
		return blueprint.Generate(command.Context(), options)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return blueprint.Plan{}, fmt.Errorf("get standalone working directory: %w", err)
	}
	return blueprint.GenerateEmbedded(command.Context(), workingDirectory, options)
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
