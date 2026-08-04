package gormdb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
)

type iamTokenBuilder func(
	context.Context,
	string,
	string,
	string,
	aws.CredentialsProvider,
) (string, error)

var (
	loadIAMConfig = func(ctx context.Context) (aws.Config, error) {
		return awsconfig.LoadDefaultConfig(ctx)
	}
	buildIAMToken iamTokenBuilder = func(
		ctx context.Context,
		endpoint string,
		region string,
		user string,
		credentials aws.CredentialsProvider,
	) (string, error) {
		return auth.BuildAuthToken(ctx, endpoint, region, user, credentials)
	}
)

type iamMySQLConnector struct {
	region      string
	base        *mysqldriver.Config
	tlsTemplate *tls.Config
	credentials aws.CredentialsProvider
	buildToken  iamTokenBuilder
}

func (c *iamMySQLConnector) Connect(ctx context.Context) (driver.Conn, error) {
	cfg, err := c.configForConnect(ctx)
	if err != nil {
		return nil, err
	}
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return connector.Connect(ctx)
}

func (c *iamMySQLConnector) Driver() driver.Driver {
	return &mysqldriver.MySQLDriver{}
}

func (c *iamMySQLConnector) configForConnect(ctx context.Context) (*mysqldriver.Config, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("gormdb: MySQL IAM connector is not configured")
	}
	builder := c.buildToken
	if builder == nil {
		builder = buildIAMToken
	}
	cfg := c.base.Clone()
	token, err := builder(ctx, cfg.Addr, c.region, cfg.User, c.credentials)
	if err != nil {
		return nil, fmt.Errorf("build MySQL RDS IAM token: %w", err)
	}
	cfg.Passwd = token
	cfg.AllowCleartextPasswords = true
	if c.tlsTemplate != nil {
		cfg.TLS = tlsForHost(c.tlsTemplate, hostOnly(cfg.Addr))
		cfg.TLSConfig = ""
	}
	return cfg, nil
}

func makeIAMDialector(
	iam AWSRDSIAM,
	driverName string,
	credentials aws.CredentialsProvider,
	builder iamTokenBuilder,
) (gorm.Dialector, *sql.DB, error) {
	tlsTemplate, err := makeIAMTLSConfig(iam)
	if err != nil {
		return nil, nil, err
	}

	switch driverName {
	case gorms.Mysql:
		base, err := makeIAMMySQLConfig(iam)
		if err != nil {
			return nil, nil, err
		}
		connector := &iamMySQLConnector{
			region:      iam.Region,
			base:        base,
			tlsTemplate: tlsTemplate,
			credentials: credentials,
			buildToken:  builder,
		}
		sqlDB := sql.OpenDB(connector)
		return gmysql.New(gmysql.Config{Conn: sqlDB}), sqlDB, nil
	case gorms.Postgres:
		connConfig, err := makeIAMPostgresConfig(iam, tlsTemplate)
		if err != nil {
			return nil, nil, err
		}
		beforeConnect := makeIAMPostgresBeforeConnect(
			iam.Region,
			iam.ServerName,
			tlsTemplate,
			credentials,
			builder,
		)
		sqlDB := pgxstdlib.OpenDB(
			*connConfig,
			pgxstdlib.OptionBeforeConnect(beforeConnect),
		)
		return gpostgres.New(gpostgres.Config{Conn: sqlDB}), sqlDB, nil
	default:
		return nil, nil, fmt.Errorf("gormdb: RDS IAM is not supported for driver %q", driverName)
	}
}

func makeIAMMySQLConfig(iam AWSRDSIAM) (*mysqldriver.Config, error) {
	cfg := mysqldriver.NewConfig()
	cfg.User = iam.User
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(iam.Host, strconv.Itoa(iam.Port))
	cfg.DBName = iam.DBName
	cfg.Params = make(map[string]string)

	values, err := url.ParseQuery(strings.TrimPrefix(strings.TrimSpace(iam.Params), "?"))
	if err != nil {
		return nil, fmt.Errorf("parse MySQL IAM params: %w", err)
	}
	for key, entries := range values {
		if len(entries) == 0 {
			continue
		}
		value := entries[len(entries)-1]
		switch strings.ToLower(key) {
		case "parsetime":
			cfg.ParseTime, err = strconv.ParseBool(value)
		case "timeout":
			cfg.Timeout, err = time.ParseDuration(value)
		case "readtimeout":
			cfg.ReadTimeout, err = time.ParseDuration(value)
		case "writetimeout":
			cfg.WriteTimeout, err = time.ParseDuration(value)
		case "loc":
			if strings.EqualFold(value, "local") {
				cfg.Loc = time.Local
			} else {
				cfg.Loc, err = time.LoadLocation(value)
			}
		case "collation":
			cfg.Collation = value
		case "interpolateparams":
			cfg.InterpolateParams, err = strconv.ParseBool(value)
		case "multistatements":
			cfg.MultiStatements, err = strconv.ParseBool(value)
		case "tls", "allowcleartextpasswords":
			// IAM controls TLS and the cleartext authentication plugin. Request
			// parameters cannot weaken those settings.
			continue
		default:
			cfg.Params[key] = value
		}
		if err != nil {
			return nil, fmt.Errorf("parse MySQL IAM param %q: %w", key, err)
		}
	}
	return cfg, nil
}

func makeIAMPostgresConfig(iam AWSRDSIAM, tlsTemplate *tls.Config) (*pgx.ConnConfig, error) {
	connectionURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(iam.Host, strconv.Itoa(iam.Port)),
		Path:   "/" + iam.DBName,
		User:   url.User(iam.User),
	}
	values, err := parsePostgresIAMParams(iam.Params)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"host", "port", "user", "password", "dbname", "database", "sslmode"} {
		values.Del(key)
	}
	if iam.InsecureSkipVerify {
		values.Set("sslmode", "require")
	} else {
		values.Set("sslmode", "verify-full")
	}
	connectionURL.RawQuery = values.Encode()

	cfg, err := pgx.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL IAM configuration: %w", err)
	}
	cfg.Password = ""
	cfg.TLSConfig = tlsForHost(tlsTemplate, iam.Host)
	return cfg, nil
}

func makeIAMPostgresBeforeConnect(
	region string,
	serverName string,
	tlsTemplate *tls.Config,
	credentials aws.CredentialsProvider,
	builder iamTokenBuilder,
) func(context.Context, *pgx.ConnConfig) error {
	return func(ctx context.Context, cfg *pgx.ConnConfig) error {
		if cfg == nil {
			return fmt.Errorf("gormdb: PostgreSQL IAM connection config is nil")
		}
		if builder == nil {
			builder = buildIAMToken
		}
		endpoint := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
		token, err := builder(ctx, endpoint, region, cfg.User, credentials)
		if err != nil {
			return fmt.Errorf("build PostgreSQL RDS IAM token: %w", err)
		}
		cfg.Password = token
		host := cfg.Host
		if serverName != "" {
			host = serverName
		}
		cfg.TLSConfig = tlsForHost(tlsTemplate, host)
		return nil
	}
}

func makeIAMTLSConfig(iam AWSRDSIAM) (*tls.Config, error) {
	var roots *x509.CertPool
	if iam.RootCAFile != "" {
		var err error
		roots, err = x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		pem, err := os.ReadFile(iam.RootCAFile)
		if err != nil {
			return nil, fmt.Errorf("read RDS root CA file: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("append certificates from RDS root CA file %q", iam.RootCAFile)
		}
	}

	serverName := strings.TrimSpace(iam.ServerName)
	if serverName == "" {
		serverName = iam.Host
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            roots,
		ServerName:         serverName,
		InsecureSkipVerify: iam.InsecureSkipVerify, //nolint:gosec // explicit compatibility escape hatch; secure by default
	}, nil
}

func tlsForHost(template *tls.Config, host string) *tls.Config {
	if template == nil {
		return nil
	}
	cfg := template.Clone()
	if cfg.ServerName == "" {
		cfg.ServerName = hostOnly(host)
	}
	return cfg
}

func hostOnly(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return strings.Trim(address, "[]")
}

func parsePostgresIAMParams(raw string) (url.Values, error) {
	values := make(url.Values)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return values, nil
	}
	if strings.Contains(raw, "&") || strings.HasPrefix(raw, "?") {
		parsed, err := url.ParseQuery(strings.TrimPrefix(raw, "?"))
		if err != nil {
			return nil, fmt.Errorf("parse PostgreSQL IAM params: %w", err)
		}
		return parsed, nil
	}
	for _, field := range strings.Fields(raw) {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("parse PostgreSQL IAM param %q: expected key=value", field)
		}
		values.Set(parts[0], parts[1])
	}
	return values, nil
}
