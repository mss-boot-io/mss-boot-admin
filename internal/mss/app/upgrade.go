package app

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func newUpgradeCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrade",
		Short: "Plan and apply three-way foundation upgrades",
	}
	command.AddCommand(newUpgradePlanCommand(rootOverride))
	command.AddCommand(newUpgradeApplyCommand(rootOverride))
	command.AddCommand(newUpgradeStatusCommand(rootOverride))
	return command
}

func newUpgradePlanCommand(rootOverride *string) *cobra.Command {
	var foundation string
	var blueprintName string
	var manifestPath string
	var format string
	command := &cobra.Command{
		Use:   "plan",
		Short: "Compare downstream customizations with a newer foundation checkout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFoundationUpgrade(cmd, rootOverride, foundation, blueprintName, manifestPath, false, format)
		},
	}
	addUpgradeFlags(command, &foundation, &blueprintName, &manifestPath, &format)
	return command
}

func newUpgradeApplyCommand(rootOverride *string) *cobra.Command {
	var foundation string
	var blueprintName string
	var manifestPath string
	var format string
	var yes bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a conflict-free foundation upgrade and record the new baseline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("--yes is required to apply a foundation upgrade")
			}
			return runFoundationUpgrade(cmd, rootOverride, foundation, blueprintName, manifestPath, true, format)
		},
	}
	addUpgradeFlags(command, &foundation, &blueprintName, &manifestPath, &format)
	command.Flags().BoolVar(&yes, "yes", false, "confirm writing the reviewed conflict-free plan")
	return command
}

func newUpgradeStatusCommand(rootOverride *string) *cobra.Command {
	var manifestPath string
	var format string
	command := &cobra.Command{
		Use:   "status",
		Short: "Show the foundation baseline currently recorded by the downstream project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			status, err := blueprint.ReadSnapshotStatus(projectContext.Root, manifestPath)
			if err != nil {
				return err
			}
			if err := status.ValidateProjectIdentity(
				projectContext.Project.Metadata.Name,
				projectContext.Project.Metadata.Repository,
			); err != nil {
				return err
			}
			return writeUpgradeStatus(cmd.OutOrStdout(), status, format)
		},
	}
	command.Flags().StringVar(&manifestPath, "manifest", ".mss/blueprint-manifest.json", "repository-relative blueprint manifest path")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func writeUpgradeStatus(writer io.Writer, status blueprint.SnapshotStatus, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	case "text":
		_, err := fmt.Fprintf(
			writer,
			"project: %s\nmodule: %s\nrepository: %s\nblueprint: %s@%s sha256 %s\nfoundation: %s@%s commit %s\ngenerator: %s@%s commit %s\nsnapshot: %s\nlock: %s sha256 %s\nmanifest: %s\n",
			status.Project,
			status.Module,
			status.Repository,
			status.Identities.Blueprint.Name,
			status.Identities.Blueprint.Version,
			status.Identities.Blueprint.SHA256,
			status.Identities.Foundation.Repository,
			status.Identities.Foundation.Version,
			status.Identities.Foundation.Commit,
			status.Identities.Generator.Tool,
			status.Identities.Generator.Version,
			status.Identities.Generator.Commit,
			status.Identities.Snapshot.SHA256,
			status.Records.LockPath,
			status.Records.LockSHA256,
			status.Records.ManifestPath,
		)
		return err
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func addUpgradeFlags(command *cobra.Command, foundation, blueprintName, manifestPath, format *string) {
	command.Flags().StringVar(foundation, "foundation", "", "path to the newer mss foundation checkout")
	command.Flags().StringVar(blueprintName, "blueprint", "", "blueprint name; defaults to the recorded baseline")
	command.Flags().StringVar(manifestPath, "manifest", ".mss/blueprint-manifest.json", "repository-relative blueprint manifest path")
	command.Flags().StringVar(format, "format", "text", "output format: text or json")
	_ = command.MarkFlagRequired("foundation")
}

func runFoundationUpgrade(
	command *cobra.Command,
	rootOverride *string,
	foundation string,
	blueprintName string,
	manifestPath string,
	write bool,
	format string,
) error {
	projectContext, err := loadProject(*rootOverride)
	if err != nil {
		return err
	}
	plan, upgradeErr := blueprint.Upgrade(command.Context(), blueprint.UpgradeOptions{
		ApplicationRoot: projectContext.Root,
		FoundationRoot:  foundation,
		ManifestPath:    manifestPath,
		Blueprint:       blueprintName,
		Application:     upgradeApplication(projectContext),
		Write:           write,
	})
	if outputErr := writeUpgradePlan(command.OutOrStdout(), plan, format); outputErr != nil {
		return outputErr
	}
	return upgradeErr
}

// upgradeApplication intentionally leaves Module empty. The Blueprint manifest
// owns the root application identity used for a three-way upgrade, while
// project.yaml's backend.module may identify a nested deployable module such as
// <application>/admin. Upgrade fills the root module from the signed-in baseline
// before validating identity and rendering the next desired state.
func upgradeApplication(projectContext *project.Context) blueprint.Application {
	if projectContext == nil {
		return blueprint.Application{}
	}
	return blueprint.Application{
		Name:        projectContext.Project.Metadata.Name,
		DisplayName: projectContext.Project.Metadata.DisplayName,
		Repository:  projectContext.Project.Metadata.Repository,
	}
}

func writeUpgradePlan(writer io.Writer, plan blueprint.UpgradePlan, format string) error {
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
