package app

import (
	"os"

	"github.com/spf13/cobra"
)

// ExecuteAgent runs the complete agent-native command tree. It is kept separate
// from the initial root constructor while the CLI surface is being expanded in
// small checkpoint commits.
func ExecuteAgent() error {
	return NewAgentRootCommand().Execute()
}

// NewAgentRootCommand creates the complete command tree used by cmd/mss and
// future protocol adapters.
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
	root.AddCommand(newSpecCommand(&rootOverride))
	root.AddCommand(newModuleCommand(&rootOverride))
	root.AddCommand(newSkillsCommand(&rootOverride))
	root.AddCommand(newEvalCommand(&rootOverride))
	root.AddCommand(newVerifyCommand(&rootOverride))
	return root
}
