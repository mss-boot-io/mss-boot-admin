package app

import (
	"os"

	"github.com/spf13/cobra"
)

// ExecuteAgent runs the complete agent-native command tree.
func ExecuteAgent() error {
	return NewAgentRootCommand().Execute()
}

// NewAgentRootCommand creates the complete command tree used by cmd/mss and
// protocol adapters.
func NewAgentRootCommand() *cobra.Command {
	var rootOverride string
	root := &cobra.Command{
		Use:           "mss",
		Short:         "Agent-native management-system foundation CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.PersistentFlags().StringVar(&rootOverride, "root", "", "repository root override")
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.AddCommand(newNewCommand(&rootOverride))
	root.AddCommand(newUpgradeCommand(&rootOverride))
	root.AddCommand(newContextCommand(&rootOverride))
	root.AddCommand(newDoctorCommand(&rootOverride))
	root.AddCommand(newSetupCommand(&rootOverride))
	root.AddCommand(newDevCommand(&rootOverride))
	root.AddCommand(newUnifiedSpecCommand(&rootOverride))
	root.AddCommand(newFeatureCommand(&rootOverride))
	root.AddCommand(newModuleCommand(&rootOverride))
	root.AddCommand(newSkillsCommand(&rootOverride))
	root.AddCommand(newEvalCommand(&rootOverride))
	root.AddCommand(newVerifyCommand(&rootOverride))
	return root
}
