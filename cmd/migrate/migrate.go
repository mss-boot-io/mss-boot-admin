package migrate

import (
	"bytes"
	"log/slog"
	"os"
	"strconv"
	"text/template"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	common "github.com/mss-boot-io/mss-boot/pkg/migration/models"
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
	driver   string
	dsn      string
	StartCmd = &cobra.Command{
		Use:     "migrate",
		Short:   "Initialize the database",
		Example: "mss-boot-admin migrate",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return setup()
		},
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
	StartCmd.PersistentFlags().StringVarP(&config.Cfg.Database.Driver,
		"gorm-driver", "r",
		"mysql", "Start server with db driver")
	StartCmd.PersistentFlags().StringVarP(&config.Cfg.Database.Source,
		"gorm-dsn", "n",
		"root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
		"Start server with db dsn")
}

func setup() error {
	// setup 00 set params
	// env overwrite args
	if os.Getenv("DB_DRIVER") != "" {
		config.Cfg.Database.Driver = os.Getenv("DB_DRIVER")
	}
	if os.Getenv("DB_DSN") != "" {
		config.Cfg.Database.Source = os.Getenv("DB_DSN")
	}
	config.Cfg.Database.Config.DisableForeignKeyConstraintWhenMigrating = true
	// setup 01 set logger
	config.Cfg.Logger.Level = slog.LevelInfo
	config.Cfg.Logger.AddSource = true

	config.Cfg.Logger.Init()
	return nil
}

func Run() error {
	if !generate {
		slog.Info("start init")
		//config.Cfg.Init(driver, dsn, &models.SystemConfig{})
		config.Cfg.Database.Init()
		return migrate()
	}
	slog.Info(`generate migration file`)
	return genFile()
}

func migrate() error {
	systemMigrate.Username = username
	systemMigrate.Password = password
	db := gormdb.DB
	err := db.AutoMigrate(&common.Migration{})
	if err != nil {
		slog.Error("auto migrate error", "err", err)
		return err
	}
	migration.Migrate.SetDb(db)
	migration.Migrate.Migrate()
	return nil
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
		err = pkg.FileCreate(b1, "./cmd/migrate/migration/system/"+m["GenerateTime"]+"_migrate.go")
		if err != nil {
			return err
		}
	} else {
		err = pkg.FileCreate(b1, "./cmd/migrate/migration/custom/"+m["GenerateTime"]+"_migrate.go")
		if err != nil {
			return err
		}
	}
	return nil
}
