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

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
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
	username       = "admin"
	password       = "123456"
	domain         = "localhost:8001"
	system         bool
	configProvider = os.Getenv("CONFIG_PROVIDER")
	driver         = "mysql"
	dsn            = "root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local"
	StartCmd       = NewCommand(nil)
)

type commandOptions struct {
	generate       bool
	username       string
	password       string
	domain         string
	system         bool
	configProvider string
	driver         string
	dsn            string
}

func defaultCommandOptions() commandOptions {
	return commandOptions{
		username:       "admin",
		password:       "123456",
		domain:         "localhost:8001",
		configProvider: os.Getenv("CONFIG_PROVIDER"),
		driver:         "mysql",
		dsn:            "root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
	}
}

// NewCommand returns an isolated migration command for one Application.
func NewCommand(registry *business.Registry) *cobra.Command {
	options := defaultCommandOptions()
	command := &cobra.Command{
		Use:     "migrate",
		Short:   "Initialize the database",
		Example: "mss-boot-admin migrate",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupWithOptions(cmd.Context(), &options)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runContextWithOptions(cmd.Context(), options, registry)
		},
	}
	command.PersistentFlags().BoolVarP(&options.system, "system", "s",
		false, "generate system migration file")
	command.PersistentFlags().BoolVarP(&options.generate, "generate", "g",
		false, "generate migration file")
	command.PersistentFlags().StringVarP(&options.configProvider,
		"config-provider", "c", options.configProvider, "Start server with config provider")
	command.PersistentFlags().StringVarP(&options.username, "username", "u",
		options.username, "system super administrator login username")
	command.PersistentFlags().StringVarP(&options.password, "password", "p",
		options.password, "system super administrator login password")
	command.PersistentFlags().StringVarP(&options.domain, "domain", "d",
		options.domain, "system tenant domain")
	command.PersistentFlags().StringVarP(&options.driver, "gorm-driver", "r",
		options.driver, "Start server with db driver")
	command.PersistentFlags().StringVarP(&options.dsn, "gorm-dsn", "n",
		options.dsn, "Start server with db dsn")
	return command
}

func init() {
	center.SetVerify(&models.User{})
}

func setup(ctx context.Context) (err error) {
	options := defaultCommandOptions()
	options.generate = generate
	options.username = username
	options.password = password
	options.domain = domain
	options.system = system
	options.configProvider = configProvider
	options.driver = driver
	options.dsn = dsn
	return setupWithOptions(ctx, &options)
}

func setupWithOptions(ctx context.Context, options *commandOptions) (err error) {
	if options == nil {
		return errors.New("migration command options are required")
	}
	defer func() {
		if err != nil {
			if closeErr := config.Cfg.Close(); closeErr != nil {
				slog.Error("close database after migration setup failure", "err", closeErr)
			}
		}
	}()

	if os.Getenv("DB_DRIVER") != "" {
		options.driver = os.Getenv("DB_DRIVER")
	}
	if os.Getenv("DB_DSN") != "" {
		options.dsn = os.Getenv("DB_DSN")
	}

	opts := []source.Option{
		source.WithDir("config"),
		source.WithProvider(source.Local),
		source.WithWatch(true),
	}
	switch source.Provider(options.configProvider) {
	case source.GORM:
		opts = []source.Option{
			source.WithProvider(source.GORM),
			source.WithGORMDriver(options.driver),
			source.WithGORMDsn(options.dsn),
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
		return fmt.Errorf("config provider %q is not supported", options.configProvider)
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
	return middleware.Init()
}

func Run() (err error) {
	return RunContext(context.Background())
}

func RunContext(ctx context.Context) (err error) {
	options := defaultCommandOptions()
	options.generate = generate
	options.username = username
	options.password = password
	options.domain = domain
	options.system = system
	return runContextWithOptions(ctx, options, nil)
}

func runContextWithOptions(
	ctx context.Context,
	options commandOptions,
	registry *business.Registry,
) (err error) {
	defer func() {
		err = errors.Join(err, config.Cfg.Close())
	}()

	if options.generate {
		slog.Info("generate migration file")
		return migration.GenFile(options.system, filepath.Join("cmd", "migrate", "migration"))
	}
	slog.Info("start database migration")
	handle := config.Cfg.DatabaseHandle()
	if handle == nil || handle.DB == nil {
		return fmt.Errorf("migration database handle is not initialized")
	}
	runner := migration.Migrate
	if registry != nil {
		runner, err = registry.MigrationRunner()
		if err != nil {
			return fmt.Errorf("prepare application migrations: %w", err)
		}
	}
	return migrateContextWithRunnerCredentials(
		ctx,
		handle.DB,
		runner,
		options.username,
		options.password,
	)
}

func migrate(db *gorm.DB) error {
	return migrateContext(context.Background(), db)
}

func migrateContext(ctx context.Context, db *gorm.DB) error {
	return migrateContextWithRunner(ctx, db, migration.Migrate)
}

func migrateContextWithRunner(ctx context.Context, db *gorm.DB, runner *migration.Migration) error {
	return migrateContextWithRunnerCredentials(ctx, db, runner, username, password)
}

func migrateContextWithRunnerCredentials(
	ctx context.Context,
	db *gorm.DB,
	runner *migration.Migration,
	adminUsername string,
	adminPassword string,
) error {
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
	systemMigrate.Username = adminUsername
	systemMigrate.Password = adminPassword
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
