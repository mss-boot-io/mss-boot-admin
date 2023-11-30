package migrate

import (
	"bytes"
	"log/slog"
	"strconv"
	"text/template"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/migration/models"
	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	_ "github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration/custom"
	systemMigrate "github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration/system"
	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:12:29
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:12:29
 */

var (
	generate bool
	username string
	password string
	system   bool
	StartCmd = &cobra.Command{
		Use:     "migrate",
		Short:   "Initialize the database",
		Example: "mss-boot-admin migrate",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run()
		},
	}
)

func init() {
	StartCmd.PersistentFlags().BoolVarP(&system, "system", "s",
		false, "generate system migration file")
	StartCmd.PersistentFlags().BoolVarP(&generate, "generate", "g",
		false, "generate migration file")
	StartCmd.PersistentFlags().StringVarP(&username, "username", "u",
		"admin", "system super administrator login username")
	StartCmd.PersistentFlags().StringVarP(&password, "password", "p",
		"ant.design", "system super administrator login password")
}

func Run() error {
	if !generate {
		slog.Info("start init")
		config.Cfg.Init()
		return migrate()
	}
	slog.Info(`generate migration file`)
	return genFile()
}

func migrate() error {
	systemMigrate.Username = username
	systemMigrate.Password = password
	db := gormdb.DB
	err := db.AutoMigrate(&models.Migration{})
	if err != nil {
		slog.Error("auto migrate error", "err", err)
		return err
	}
	migration.Migrate.SetDb(db)
	migration.Migrate.Migrate()
	return err
}

func genFile() error {
	t1, err := template.ParseFiles("template/migrate.tpl")
	if err != nil {
		slog.Error("parse template error", err)
		return err
	}
	m := map[string]string{}
	m["GenerateTime"] = strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	m["Package"] = "custom"
	if system {
		m["Package"] = "system"
	}
	var b1 bytes.Buffer
	err = t1.Execute(&b1, m)
	if system {
		err = pkg.FileCreate(b1, "./cmd/migrate/migration/system/"+m["GenerateTime"]+"_migrate.go", 0666)
		if err != nil {
			return err
		}
	} else {
		err = pkg.FileCreate(b1, "./cmd/migrate/migration/custom/"+m["GenerateTime"]+"_migrate.go", 0666)
		if err != nil {
			return err
		}
	}
	return nil
}
