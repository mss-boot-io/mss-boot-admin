package gormdb

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
)

const maxIAMConnectionLifetimeSeconds = 14 * 60

// Database configures one GORM database stack.
type Database struct {
	Driver          string             `yaml:"driver" json:"driver"`
	Source          string             `yaml:"source" json:"source"`
	ConnMaxIdleTime int                `yaml:"connMaxIdleTime" json:"connMaxIdleTime"`
	ConnMaxLifeTime int                `yaml:"connMaxLifeTime" json:"connMaxLifeTime"`
	MaxIdleConns    int                `yaml:"maxIdleConns" json:"maxIdleConns"`
	MaxOpenConns    int                `yaml:"maxOpenConns" json:"maxOpenConns"`
	Registers       []DBResolverConfig `yaml:"registers" json:"registers"`
	CasbinModel     string             `yaml:"casbinModel" json:"casbinModel"`
	Config          GORMConfig         `yaml:"config" json:"config"`
	IAM             AWSRDSIAM          `yaml:"iam" json:"iam"`
}

// DBResolverConfig configures one database resolver registration.
type DBResolverConfig struct {
	Sources  []string `yaml:"sources" json:"sources"`
	Replicas []string `yaml:"replicas" json:"replicas"`
	Policy   string   `yaml:"policy" json:"policy"`
	Tables   []string `yaml:"tables" json:"tables"`
}

// GORMConfig maps the supported gorm.Config fields.
type GORMConfig struct {
	SkipDefaultTransaction                   bool `yaml:"skipDefaultTransaction" json:"skipDefaultTransaction"`
	FullSaveAssociations                     bool `yaml:"fullSaveAssociations" json:"fullSaveAssociations"`
	DryRun                                   bool `yaml:"dryRun" json:"dryRun"`
	PrepareStmt                              bool `yaml:"prepareStmt" json:"prepareStmt"`
	DisableAutomaticPing                     bool `yaml:"disableAutomaticPing" json:"disableAutomaticPing"`
	DisableForeignKeyConstraintWhenMigrating bool `yaml:"disableForeignKeyConstraintWhenMigrating" json:"disableForeignKeyConstraintWhenMigrating"`
	IgnoreRelationshipsWhenMigrating         bool `yaml:"ignoreRelationshipsWhenMigrating" json:"ignoreRelationshipsWhenMigrating"`
	DisableNestedTransaction                 bool `yaml:"disableNestedTransaction" json:"disableNestedTransaction"`
	AllowGlobalUpdate                        bool `yaml:"allowGlobalUpdate" json:"allowGlobalUpdate"`
	QueryFields                              bool `yaml:"queryFields" json:"queryFields"`
	CreateBatchSize                          int  `yaml:"createBatchSize" json:"createBatchSize"`
	TranslateError                           bool `yaml:"translateError" json:"translateError"`
}

// AWSRDSIAM configures AWS RDS IAM authentication. Tokens are generated for
// every new physical connection. TLS certificate verification is enabled by
// default; InsecureSkipVerify is an explicit compatibility escape hatch.
type AWSRDSIAM struct {
	Enable             bool   `yaml:"enable" json:"enable"`
	Region             string `yaml:"region" json:"region"`
	User               string `yaml:"user" json:"user"`
	Host               string `yaml:"host" json:"host"`
	Port               int    `yaml:"port" json:"port"`
	DBName             string `yaml:"dbName" json:"dbName"`
	Params             string `yaml:"params" json:"params"`
	RootCAFile         string `yaml:"rootCAFile" json:"rootCAFile"`
	ServerName         string `yaml:"serverName" json:"serverName"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
}

// Open initializes a database stack without mutating the receiver or any
// package global. The caller owns the returned Handle and must close it.
func (e *Database) Open(ctx context.Context) (*Handle, error) {
	if e == nil {
		return nil, fmt.Errorf("gormdb: database configuration is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resolved := e.resolved()
	resolved.Driver = strings.ToLower(strings.TrimSpace(resolved.Driver))
	if resolved.Driver == "" {
		return nil, fmt.Errorf("gormdb: database driver is required")
	}

	open, ok := Opens[resolved.Driver]
	if !ok || open == nil {
		return nil, fmt.Errorf("gormdb: unsupported database driver %q", resolved.Driver)
	}

	source := resolved.Source
	connMaxLifetime := resolved.ConnMaxLifeTime
	var preopenedPool interface{ Close() error }
	if resolved.IAM.Enable {
		if len(resolved.Registers) > 0 {
			return nil, fmt.Errorf("gormdb: RDS IAM does not support resolver registrations yet")
		}
		iam, err := resolved.IAM.normalized(resolved.Driver)
		if err != nil {
			return nil, err
		}
		awsConfig, err := loadIAMConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load AWS configuration for RDS IAM: %w", err)
		}
		dialector, sqlDB, err := makeIAMDialector(
			iam,
			resolved.Driver,
			awsConfig.Credentials,
			buildIAMToken,
		)
		if err != nil {
			return nil, err
		}
		preopenedPool = sqlDB
		open = func(string) gorm.Dialector { return dialector }
		source = ""
		if connMaxLifetime == 0 || connMaxLifetime > maxIAMConnectionLifetimeSeconds {
			connMaxLifetime = maxIAMConnectionLifetimeSeconds
		}
	}

	registers := make([]ResolverConfigure, len(resolved.Registers))
	for i := range resolved.Registers {
		registers[i] = NewResolverConfigure(
			resolved.Registers[i].Sources,
			resolved.Registers[i].Replicas,
			resolved.Registers[i].Policy,
			resolved.Registers[i].Tables,
		)
	}
	resolverConfig := NewConfigure(
		source,
		resolved.MaxIdleConns,
		resolved.MaxOpenConns,
		resolved.ConnMaxIdleTime,
		connMaxLifetime,
		registers,
	)

	db, err := resolverConfig.Init(resolved.gormConfig(), open)
	if err != nil {
		if preopenedPool != nil {
			_ = preopenedPool.Close()
		}
		return nil, fmt.Errorf("open %s database: %w", resolved.Driver, err)
	}
	handle, err := newHandle(db, nil, resolved.Driver)
	if err != nil {
		if preopenedPool != nil {
			_ = preopenedPool.Close()
		}
		return nil, fmt.Errorf("own %s database pool: %w", resolved.Driver, err)
	}

	if resolved.CasbinModel != "" {
		enforcer, err := newEnforcer(db, resolved.CasbinModel)
		if err != nil {
			_ = handle.Close()
			return nil, err
		}
		handle.Enforcer = enforcer
	}
	return handle, nil
}

// InitContext is the compatibility bridge for applications that still depend
// on package globals. It opens and installs a new default Handle, then closes
// the previously installed Handle.
func (e *Database) InitContext(ctx context.Context) (*Handle, error) {
	handle, err := e.Open(ctx)
	if err != nil {
		return nil, err
	}
	previous := InstallDefault(handle)
	if previous != nil && previous != handle {
		if err := previous.Close(); err != nil {
			return handle, fmt.Errorf("close previous database handle: %w", err)
		}
	}
	return handle, nil
}

// Init preserves the historical no-return signature. It no longer terminates
// the embedding process; applications that need startup guarantees must call
// Open or InitContext and handle the returned error.
func (e *Database) Init() {
	if _, err := e.InitContext(context.Background()); err != nil {
		slog.Error("database initialization failed", "err", err)
	}
}

func (e *Database) resolved() Database {
	resolved := *e
	resolved.Driver = pkg.ParseEnvTemplate(resolved.Driver)
	resolved.Source = pkg.ParseEnvTemplate(resolved.Source)
	resolved.CasbinModel = pkg.ParseEnvTemplate(resolved.CasbinModel)
	resolved.IAM.Region = pkg.ParseEnvTemplate(resolved.IAM.Region)
	resolved.IAM.User = pkg.ParseEnvTemplate(resolved.IAM.User)
	resolved.IAM.Host = pkg.ParseEnvTemplate(resolved.IAM.Host)
	resolved.IAM.DBName = pkg.ParseEnvTemplate(resolved.IAM.DBName)
	resolved.IAM.Params = pkg.ParseEnvTemplate(resolved.IAM.Params)
	resolved.IAM.RootCAFile = pkg.ParseEnvTemplate(resolved.IAM.RootCAFile)
	resolved.IAM.ServerName = pkg.ParseEnvTemplate(resolved.IAM.ServerName)
	resolved.Registers = make([]DBResolverConfig, len(e.Registers))
	for i := range e.Registers {
		resolved.Registers[i] = e.Registers[i]
		resolved.Registers[i].Sources = append([]string(nil), e.Registers[i].Sources...)
		resolved.Registers[i].Replicas = append([]string(nil), e.Registers[i].Replicas...)
		resolved.Registers[i].Tables = append([]string(nil), e.Registers[i].Tables...)
		for j := range resolved.Registers[i].Sources {
			resolved.Registers[i].Sources[j] = pkg.ParseEnvTemplate(resolved.Registers[i].Sources[j])
		}
		for j := range resolved.Registers[i].Replicas {
			resolved.Registers[i].Replicas[j] = pkg.ParseEnvTemplate(resolved.Registers[i].Replicas[j])
		}
	}
	return resolved
}

func (e Database) gormConfig() *gorm.Config {
	return &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		Logger:         logger.Default,

		SkipDefaultTransaction:                   e.Config.SkipDefaultTransaction,
		FullSaveAssociations:                     e.Config.FullSaveAssociations,
		DryRun:                                   e.Config.DryRun,
		PrepareStmt:                              e.Config.PrepareStmt,
		DisableAutomaticPing:                     e.Config.DisableAutomaticPing,
		DisableForeignKeyConstraintWhenMigrating: e.Config.DisableForeignKeyConstraintWhenMigrating,
		IgnoreRelationshipsWhenMigrating:         e.Config.IgnoreRelationshipsWhenMigrating,
		DisableNestedTransaction:                 e.Config.DisableNestedTransaction,
		AllowGlobalUpdate:                        e.Config.AllowGlobalUpdate,
		QueryFields:                              e.Config.QueryFields,
		CreateBatchSize:                          e.Config.CreateBatchSize,
		TranslateError:                           e.Config.TranslateError,
	}
}

func (e AWSRDSIAM) normalized(driverName string) (AWSRDSIAM, error) {
	e.Region = strings.TrimSpace(e.Region)
	e.User = strings.TrimSpace(e.User)
	e.Host = strings.TrimSpace(e.Host)
	e.DBName = strings.TrimSpace(e.DBName)
	e.RootCAFile = strings.TrimSpace(e.RootCAFile)
	e.ServerName = strings.TrimSpace(e.ServerName)

	if e.Region == "" || e.User == "" || e.Host == "" || e.DBName == "" {
		return AWSRDSIAM{}, fmt.Errorf(
			"gormdb: RDS IAM requires region, user, host, and dbName",
		)
	}
	if host, port, err := net.SplitHostPort(e.Host); err == nil {
		e.Host = host
		if e.Port == 0 {
			parsedPort, parseErr := strconv.Atoi(port)
			if parseErr != nil {
				return AWSRDSIAM{}, fmt.Errorf("gormdb: parse RDS IAM port %q: %w", port, parseErr)
			}
			e.Port = parsedPort
		}
	}
	if e.Port == 0 {
		switch driverName {
		case gorms.Postgres:
			e.Port = 5432
		case gorms.Mysql:
			e.Port = 3306
		default:
			return AWSRDSIAM{}, fmt.Errorf("gormdb: RDS IAM is not supported for driver %q", driverName)
		}
	}
	if e.Port < 1 || e.Port > 65535 {
		return AWSRDSIAM{}, fmt.Errorf("gormdb: RDS IAM port %d is outside 1..65535", e.Port)
	}
	return e, nil
}

func newEnforcer(db *gorm.DB, casbinModel string) (casbin.IEnforcer, error) {
	var adapter persist.Adapter
	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "mss_boot", "casbin_rule")
	if err != nil {
		return nil, fmt.Errorf("create Casbin GORM adapter: %w", err)
	}
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("parse Casbin model: %w", err)
	}
	enforcer, err := newSynchronizedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("create Casbin enforcer: %w", err)
	}
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load Casbin policy: %w", err)
	}
	enforcer.EnableAutoSave(true)
	enforcer.EnableAutoBuildRoleLinks(true)
	enforcer.EnableLog(true)
	return enforcer, nil
}
