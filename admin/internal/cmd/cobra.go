package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/internal/cmd/migrate"
	"github.com/mss-boot-io/mss-boot-admin/admin/internal/cmd/server"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:14:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:14:22
 */

// New returns a fresh command tree for one guarded Application execution. The
// package lives below admin/internal so repository-external consumers cannot
// obtain or execute the Cobra tree directly.
func New(registry *business.Registry) *cobra.Command {
	root := &cobra.Command{
		Use:          "mss-boot-admin",
		Short:        "mss-boot-admin",
		Version:      pkg.BuildVersion(),
		SilenceUsage: true,
		Long:         `mss-boot-admin is a background management system developed by the mss-boot framework`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				tip(cmd)
				return errors.New(pkg.Red("requires at least one arg"))
			}
			return nil
		},
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		Run: func(cmd *cobra.Command, _ []string) {
			tip(cmd)
		},
	}
	root.AddCommand(server.NewCommand(registry))
	root.AddCommand(migrate.NewCommand(registry))
	return root
}

func tip(cmd *cobra.Command) {
	usageStr := `欢迎使用 ` + pkg.Green(`mss-boot-admin `+pkg.BuildVersion()) + ` 可以使用 ` + pkg.Red(`-h`) + ` 查看命令`
	usageStr1 := `也可以参考 https://docs.mss-boot-io.top 的相关内容`
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", usageStr)
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", usageStr1)
}

// ExecuteContext remains internal for package-level compatibility tests. All
// repository-external execution enters through app.Application.
func ExecuteContext(ctx context.Context, registry *business.Registry) error {
	if ctx == nil {
		return errors.New("Admin command context is required")
	}
	return New(registry).ExecuteContext(ctx)
}
