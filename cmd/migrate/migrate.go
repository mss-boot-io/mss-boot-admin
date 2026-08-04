package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	common "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"

	"github.com/mss-boot-io/mss-boot-admin/center"
	_ "github.com/mss-boot-io/mss-boot-admin/cmd/migrate/migration/custom"
	systemMigrate "github.com/mss-boot-io/mss-boot-admin/cmd/migrate/migration/system"
	"github.com/mss-boot-io/mss-boot-admin/config"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
	moduleruntime "github.com/mss-boot-io/mss-boot-admin/modules/runtime"
)

var (
	generate       bool
	username       string
	password       string
	domain         string
	system         bool
	configProvider string
	driver         string
	dsn            string
	StartCmd       = &cobra.Command{
		Use:     "migrate",
		Short:   "Initialize the database",
		Example: "mss-boot-admin migrate",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return setup(cmd.Context())
		},
		RunE: func(_ *cobra.Command, _ []string) error {
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
	StartCmd.PersistentFlags().StringVarP(&domain, "domain", "d",
		"localhost:8000", "system tenant domain")
	StartCmd.PersistentFlags().StringVarP(&driver,
		"gorm-driver", "r",
		"mysql", "Start server with db driver")
	StartCmd.PersistentFlags().StringVarP(&dsn,
		"gorm-dsn", "n",
		"root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
		"Start server with db dsn")
	center.SetVerify(&models.User{})
}

func setup(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			if closeErr := config.Cfg.Close(); closeErr != nil {
				slog.Error("close database after migration setup failure", "err", closeErr)
			}
		}
	}()

	if os.Getenv("DB_DRIVER") != "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if os.Getenv("DB_DSN") != "" {
		dsn = os.Getenv("DB_DSN")
	}

	opts := []source.Option{
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
	case source.ConfigMap:
		opts = []source.Option{
			source.WithProvider(source.ConfigMap),
			source.WithConfigmap("mss-boot-admin"),
			source.WithNamespace(pkg.GetStage()),
		}
	case source.Consul:
		opts = []source.Option{
			source.WithProvider(source.Consul),
			source.WithDir("mss-boot-admin/config"),
			source.WithWatch(true),
		}
	case source.APPConfig:
		opts = []source.Option{
			source.WithProvider(source.APPConfig),
			source.WithProjectName(pkg.GetProjectName()),
			source.WithNamespace(pkg.GetStage()),
		}
	case source.Local, "":
	default:
		return fmt.Errorf("config provider %q is not supported", configProvider)
	}
	center.SetConfig(config.Cfg)
	if err := config.Cfg.InitContext(ctx, opts...); err != nil {
		return err
	}

	center.SetAppConfig(&models.AppConfig{})
	center.SetUserConfig(&models.UserConfig{})
	center.SetStatistics(&models.Statistics{})
	center.SetGRPCClient(&config.Cfg.GRPC)

	middleware.Verifier = center.GetUser()
	middleware.Init()
	return nil
}

func Run() (err error) {
	defer func() {
		err = errors.Join(err, config.Cfg.Close())
	}()

	if generate {
		slog.Info("generate migration file")
		return migration.GenFile(system, filepath.Join("cmd", "migrate", "migration"))
	}
	slog.Info("start database migration")
	handle := config.Cfg.DatabaseHandle()
	if handle == nil || handle.DB == nil {
		return fmt.Errorf("migration database handle is not initialized")
	}
	return migrate(handle.DB)
}

func migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is nil")
	}
	systemMigrate.Username = username
	systemMigrate.Password = password
	if err := db.AutoMigrate(&common.Migration{}); err != nil {
		slog.Error("auto migrate error", "err", err)
		return err
	}
	migration.Migrate.SetDb(db)
	migration.Migrate.SetModel(&common.Migration{})
	migration.Migrate.Migrate()
	return moduleruntime.Migrate(db)
}
