package app

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func newUpgradeCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrade",
		Short: "Plan and apply three-way foundation upgrades",
	}
	command.AddCommand(newUpgradePlanCommand(rootOverride))
	command.AddCommand(newUpgradeApplyCommand(rootOverride))
	command.AddCommand(newUpgradeAdminCommand(rootOverride))
	command.AddCommand(newUpgradeStatusCommand(rootOverride))
	return command
}

func newUpgradeAdminCommand(rootOverride *string) *cobra.Command {
	var foundation string
	var blueprintName string
	var manifestPath string
	var format string
	var frontendRegistry string
	var apply bool
	var yes bool
	command := &cobra.Command{
		Use:   "admin <vX.Y.Z>",
		Short: "Plan or apply one coordinated Admin Distribution upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if apply && !yes {
				return fmt.Errorf("--yes is required with --apply for an Admin Distribution upgrade")
			}
			if yes && !apply {
				return fmt.Errorf("--yes is only valid together with --apply")
			}
			return runAdminDistributionUpgrade(cmd, rootOverride, args[0], foundation, blueprintName, manifestPath, frontendRegistry, apply, format)
		},
	}
	addUpgradeFlags(command, &foundation, &blueprintName, &manifestPath, &format, false)
	command.Flags().StringVar(&frontendRegistry, "contributor-npm-registry", "", "loopback npm registry override for contributor qualification")
	_ = command.Flags().MarkHidden("contributor-npm-registry")
	command.Flags().BoolVar(&apply, "apply", false, "apply the reviewed conflict-free Distribution plan; default is read-only")
	command.Flags().BoolVar(&yes, "yes", false, "confirm applying the complete Admin Distribution upgrade")
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
	addUpgradeFlags(command, &foundation, &blueprintName, &manifestPath, &format, true)
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
	addUpgradeFlags(command, &foundation, &blueprintName, &manifestPath, &format, true)
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
			"project: %s\nmodule: %s\nrepository: %s\nadmin distribution: %s\nblueprint: %s@%s sha256 %s\nfoundation: %s@%s commit %s\ngenerator: %s@%s commit %s\nsnapshot: %s\nlock: %s sha256 %s\nmanifest: %s\n",
			status.Project,
			status.Module,
			status.Repository,
			upgradeDistributionSummary(status.Distribution),
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

func addUpgradeFlags(command *cobra.Command, foundation, blueprintName, manifestPath, format *string, requireFoundation bool) {
	command.Flags().StringVar(foundation, "foundation", "", "clean Foundation checkout override for contributor development")
	command.Flags().StringVar(blueprintName, "blueprint", "", "blueprint name; defaults to the recorded baseline")
	command.Flags().StringVar(manifestPath, "manifest", ".mss/blueprint-manifest.json", "repository-relative blueprint manifest path")
	command.Flags().StringVar(format, "format", "text", "output format: text or json")
	if requireFoundation {
		_ = command.MarkFlagRequired("foundation")
	}
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
		ApplicationRoot:          projectContext.Root,
		FoundationRoot:           foundation,
		ManifestPath:             manifestPath,
		Blueprint:                blueprintName,
		Application:              upgradeApplication(projectContext),
		ModuleSpecificationsPath: upgradeModuleSpecificationsPath(projectContext),
		PreservedBusinessPaths:   upgradePreservedBusinessPaths(projectContext),
		ValidationCommands:       upgradeValidationCommands(projectContext),
		Write:                    write,
	})
	if outputErr := writeUpgradePlan(command.OutOrStdout(), plan, format); outputErr != nil {
		return outputErr
	}
	return upgradeErr
}

func runAdminDistributionUpgrade(
	command *cobra.Command,
	rootOverride *string,
	requestedVersion string,
	foundation string,
	blueprintName string,
	manifestPath string,
	frontendRegistry string,
	write bool,
	format string,
) error {
	projectContext, err := loadProject(*rootOverride)
	if err != nil {
		return err
	}
	options := blueprint.UpgradeOptions{
		ApplicationRoot:              projectContext.Root,
		FoundationRoot:               foundation,
		ManifestPath:                 manifestPath,
		Blueprint:                    blueprintName,
		Application:                  upgradeApplication(projectContext),
		RequestedDistributionVersion: requestedVersion,
		ModuleSpecificationsPath:     upgradeModuleSpecificationsPath(projectContext),
		PreservedBusinessPaths:       upgradePreservedBusinessPaths(projectContext),
		ValidationCommands:           upgradeValidationCommands(projectContext),
		FrontendRegistryURL:          frontendRegistry,
		Write:                        write,
	}
	var plan blueprint.UpgradePlan
	var upgradeErr error
	if strings.TrimSpace(foundation) == "" {
		if err := validateEmbeddedAdminUpgradeVersion(requestedVersion); err != nil {
			return err
		}
		plan, upgradeErr = blueprint.UpgradeEmbedded(command.Context(), options)
	} else {
		plan, upgradeErr = blueprint.Upgrade(command.Context(), options)
	}
	if outputErr := writeUpgradePlan(command.OutOrStdout(), plan, format); outputErr != nil {
		return outputErr
	}
	return upgradeErr
}

func validateEmbeddedAdminUpgradeVersion(requestedVersion string) error {
	provenance, err := buildinfo.ReleaseProvenance()
	if err != nil {
		return fmt.Errorf("embedded Admin upgrade is unavailable: %w; install an official release-built mss binary or use --foundation only from a clean Foundation contributor checkout", err)
	}
	installed := strings.TrimSpace(provenance.Version)
	if !strings.HasPrefix(installed, "v") {
		installed = "v" + installed
	}
	if strings.TrimSpace(requestedVersion) != installed {
		return fmt.Errorf(
			"installed mss %s cannot upgrade Admin Distribution %s; install the matching mss %s tool or use --foundation only from a clean Foundation contributor checkout",
			installed,
			requestedVersion,
			requestedVersion,
		)
	}
	return nil
}

func upgradeModuleSpecificationsPath(projectContext *project.Context) string {
	if projectContext == nil {
		return ".mss/modules"
	}
	specifications := strings.TrimSpace(projectContext.Project.Spec.RepositoryLayout["specifications"])
	if specifications == "" {
		specifications = ".mss"
	}
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "modules"))
}

func upgradeValidationCommands(projectContext *project.Context) []string {
	commands := []string{"mss doctor --strict --format json"}
	if projectContext == nil {
		return append(commands, "mss verify --all")
	}
	for _, name := range []string{"backend-test", "backend-build", "frontend-lint", "frontend-test", "frontend-build", "verify"} {
		entry, ok := projectContext.Commands.Spec.Commands[name]
		if !ok || strings.TrimSpace(entry.Command) == "" {
			continue
		}
		commands = append(commands, strings.TrimSpace(entry.Command))
	}
	if len(commands) == 1 {
		commands = append(commands, "mss verify --all")
	}
	return commands
}

func upgradePreservedBusinessPaths(projectContext *project.Context) []string {
	if projectContext == nil {
		return nil
	}
	layout := projectContext.Project.Spec.RepositoryLayout
	specifications := strings.TrimSpace(layout["specifications"])
	if specifications == "" {
		specifications = ".mss"
	}
	frontend := strings.TrimSpace(layout["frontend"])
	paths := []string{
		strings.TrimSpace(layout["modules"]),
		strings.TrimSpace(layout["generated"]),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "modules")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(specifications), "features")),
	}
	if frontend != "" {
		paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(frontend), "src", "business")))
	}
	documentation := strings.TrimSpace(layout["documentation"])
	if documentation != "" {
		if projectContext.LayoutKind() == "foundation" {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "docs", "modules")))
		} else {
			paths = append(paths, filepath.ToSlash(filepath.Join(filepath.FromSlash(documentation), "modules")))
		}
	}
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

func upgradeDistributionSummary(distribution project.DistributionSpec) string {
	if distribution.Empty() {
		return "unrecorded"
	}
	return fmt.Sprintf(
		"%s@%s (backend %s@%s, frontend %s@%s)",
		distribution.Name,
		distribution.Version,
		distribution.Backend.Module,
		distribution.Backend.Version,
		distribution.Frontend.Package,
		distribution.Frontend.Version,
	)
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
