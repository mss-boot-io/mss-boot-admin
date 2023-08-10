package server

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/router"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:33:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:33:48
 */

var (
	apiCheck bool
	group    string
	StartCmd = &cobra.Command{
		Use:     "server",
		Short:   "start server",
		Long:    "start mss-boot-admin server",
		Example: "mss-boot-admin server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return setup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
)

func init() {
	StartCmd.PersistentFlags().BoolVarP(&apiCheck,
		"api", "a",
		false,
		"Start server with check api data")
	StartCmd.PersistentFlags().StringVarP(&group,
		"group", "g",
		"/admin",
		"Start server with group path")
}

func setup() error {
	r := gin.Default()
	router.Init(r.Group(group))

	config.Cfg.Init(r)
	return nil
}

func run() error {
	ctx := context.Background()

	return server.Manage.Start(ctx)
}
