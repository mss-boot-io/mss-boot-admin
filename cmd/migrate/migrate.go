package migrate

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/migration"
	common "github.com/mss-boot-io/mss-boot/pkg/migration/models"
	"github.com/spf13/cobra"

	_ "github.com/mss-boot-io/mss-boot-admin/cmd/migrate/migration/custom"
	systemMigrate "github.com/mss-boot-io/mss-boot-admin/cmd/migrate/migration/system"
	"github.com/mss-boot-io/mss-boot-admin/config"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:12:29
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:12:29
 */

var (
	generate       bool
	username       string
	password       string
	system         bool
	configProvider string
	driver         string
	dsn            string
	StartCmd       = &cobra.Command{
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
	StartCmd.PersistentFlags().StringVarP(&configProvider,
		"config-provider", "c",
		os.Getenv("CONFIG_PROVIDER"), "Start server with config provider")
	StartCmd.PersistentFlags().StringVarP(&username, "username", "u",
		"admin", "system super administrator login username")
	StartCmd.PersistentFlags().StringVarP(&password, "password", "p",
		"123456", "system super administrator login password")
	StartCmd.PersistentFlags().StringVarP(&driver,
		"gorm-driver", "r",
		"mysql", "Start server with db driver")
	StartCmd.PersistentFlags().StringVarP(&dsn,
		"gorm-dsn", "n",
		"root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
		"Start server with db dsn")
	center.SetTenant(&models.Tenant{}).
		SetVerify(&models.User{})
}

func setup() error {
	// setup 00 set params
	// env overwrite args
	if os.Getenv("DB_DRIVER") != "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if os.Getenv("DB_DSN") != "" {
		dsn = os.Getenv("DB_DSN")
	}
	// setup 01 config init
	opts := []source.Option{
		// use local config file
		source.WithDir("config"),
		source.WithProvider(source.Local),
		source.WithWatch(true),
	}
	switch source.Provider(configProvider) {
	case source.GORM:
		opts = []source.Option{
			source.WithProvider(source.GORM),
			source.WithGORMDriver(driver),
			source.WithGORMDsn(dsn),
			source.WithDriver(&models.SystemConfig{}),
		}
	case source.FS:
		opts = []source.Option{
			source.WithProvider(source.FS),
			source.WithFrom(config.FS),
		}
	case source.Local, "":
	default:
		slog.Error("config provider not support", "provider", configProvider)
		os.Exit(-1)
	}
	center.SetConfig(config.Cfg).Init(opts...)

	center.SetStatistics(&models.Statistics{})

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
	return migration.GenFile(system, filepath.Join("cmd", "migrate", "migration"))
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
	migration.Migrate.SetModel(&common.Migration{})
	migration.Migrate.Migrate()
	return nil
}
