package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	devcmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/dev"
)

func newDevCommand(rootOverride *string) *cobra.Command {
	var detach bool
	var format string
	command := &cobra.Command{
		Use:   "dev [service...]",
		Short: "Start and manage the canonical local development environment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevStart(cmd, rootOverride, args, detach, format)
		},
	}
	command.Flags().BoolVarP(&detach, "detach", "d", false, "start services in the background")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.AddCommand(newDevStartCommand(rootOverride))
	command.AddCommand(newDevStatusCommand(rootOverride))
	command.AddCommand(newDevStopCommand(rootOverride))
	command.AddCommand(newDevLogsCommand(rootOverride))
	return command
}

func newDevStartCommand(rootOverride *string) *cobra.Command {
	var detach bool
	var format string
	command := &cobra.Command{
		Use:   "start [service...]",
		Short: "Start selected services and their dependencies",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevStart(cmd, rootOverride, args, detach, format)
		},
	}
	command.Flags().BoolVarP(&detach, "detach", "d", false, "start services in the background")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func runDevStart(cmd *cobra.Command, rootOverride *string, services []string, detach bool, format string) error {
	projectContext, err := loadProject(*rootOverride)
	if err != nil {
		return err
	}
	config, err := devcmd.Load(projectContext.Root)
	if err != nil {
		return err
	}
	report, runErr := devcmd.Start(cmd.Context(), config, devcmd.StartOptions{
		Services: services,
		Detach:   detach,
		Stdout:   cmd.OutOrStdout(),
		Stderr:   cmd.ErrOrStderr(),
	})
	if outputErr := writeDevReport(cmd.OutOrStdout(), report, format); outputErr != nil {
		return outputErr
	}
	return runErr
}

func newDevStatusCommand(rootOverride *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "status [service...]",
		Short: "Inspect service process state and HTTP health",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			config, err := devcmd.Load(projectContext.Root)
			if err != nil {
				return err
			}
			report, statusErr := devcmd.Status(cmd.Context(), config, args)
			if outputErr := writeDevReport(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			if statusErr != nil {
				return statusErr
			}
			if !report.Success {
				return errors.New("one or more development services are degraded or have stale state")
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func newDevStopCommand(rootOverride *string) *cobra.Command {
	var force bool
	var format string
	command := &cobra.Command{
		Use:   "stop [service...]",
		Short: "Stop selected services in reverse dependency order",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			config, err := devcmd.Load(projectContext.Root)
			if err != nil {
				return err
			}
			report, stopErr := devcmd.Stop(cmd.Context(), config, devcmd.StopOptions{
				Services: args,
				Force:    force,
			})
			if outputErr := writeDevReport(cmd.OutOrStdout(), report, format); outputErr != nil {
				return outputErr
			}
			return stopErr
		},
	}
	command.Flags().BoolVar(&force, "force", false, "terminate process trees immediately")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func newDevLogsCommand(rootOverride *string) *cobra.Command {
	var follow bool
	var lines int
	command := &cobra.Command{
		Use:   "logs <service>",
		Short: "Print or follow one development service log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			config, err := devcmd.Load(projectContext.Root)
			if err != nil {
				return err
			}
			return devcmd.Logs(cmd.Context(), config, args[0], lines, follow, cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVarP(&follow, "follow", "f", false, "follow appended log output")
	command.Flags().IntVarP(&lines, "lines", "n", 100, "number of trailing lines; use 0 for the complete log")
	return command
}

func writeDevReport(writer io.Writer, report devcmd.Report, format string) error {
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
