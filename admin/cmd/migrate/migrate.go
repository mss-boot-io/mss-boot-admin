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

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	_ "github.com/mss-boot-io/mss-boot-admin/admin/cmd/migrate/migration/custom"
	systemMigrate "github.com/mss-boot-io/mss-boot-admin/admin/cmd/migrate/migration/system"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	moduleruntime "github.com/mss-boot-io/mss-boot-admin/admin/modules/runtime"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunContext(cmd.Context())
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
	return RunContext(context.Background())
}

func RunContext(ctx context.Context) (err error) {
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
	return migrateContext(ctx, handle.DB)
}

func migrate(db *gorm.DB) error {
	return migrateContext(context.Background(), db)
}

func migrateContext(ctx context.Context, db *gorm.DB) error {
	return migrateContextWithRunner(ctx, db, migration.Migrate)
}

func migrateContextWithRunner(ctx context.Context, db *gorm.DB, runner *migration.Migration) error {
	if ctx == nil {
		return fmt.Errorf("migration context is nil")
	}
	if db == nil {
		return fmt.Errorf("migration database is nil")
	}
	if runner == nil {
		return fmt.Errorf("%w: migration runner is nil", migration.ErrMigrationNotReady)
	}
	// Registration validation is deliberately independent of the database and
	// runs before WithContext or AutoMigrate. Invalid or duplicate IDs therefore
	// cannot create or alter even the migration bookkeeping table.
	if err := runner.ValidateRegistrations(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("migration canceled before database preflight: %w", err)
	}
	db = db.WithContext(ctx)
	systemMigrate.Username = username
	systemMigrate.Password = password
	if err := db.AutoMigrate(&common.Migration{}); err != nil {
		slog.Error("auto migrate error", "err", err)
		return err
	}
	runner.SetDb(db)
	runner.SetModel(&common.Migration{})
	if err := runner.MigrateContext(ctx); err != nil {
		return err
	}
	if err := moduleruntime.Migrate(db); err != nil {
		return err
	}
	if err := schemahealth.VerifyCanonicalEmailIdentity(
		ctx,
		db,
		schemahealth.CanonicalEmailRuntimeReadiness,
	); err != nil {
		return fmt.Errorf("migration schema readiness failed: %w", err)
	}
	return nil
}
