package gormdb

import (
	"context"
	"crypto/tls"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jackc/pgx/v5"
)

func TestDatabaseOpenReturnsOwnedSQLiteHandle(t *testing.T) {
	configuration := &Database{
		Driver: "sqlite",
		Source: "file:owned-handle?mode=memory&cache=shared",
	}

	handle, err := configuration.Open(context.Background())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if handle.DB == nil || handle.SQLDB() == nil {
		t.Fatal("expected GORM and database/sql handles")
	}
	if handle.Driver != "sqlite" {
		t.Fatalf("driver = %q, want sqlite", handle.Driver)
	}
	if err := handle.DB.Exec("create table ownership_records (id integer primary key)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close handle: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("second close must be idempotent: %v", err)
	}
	if err := handle.SQLDB().Ping(); err == nil {
		t.Fatal("expected closed pool to reject ping")
	}
}

func TestDatabaseOpenRejectsUnsupportedDriver(t *testing.T) {
	configuration := &Database{Driver: "oracle", Source: "ignored"}
	if _, err := configuration.Open(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported database driver") {
		t.Fatalf("expected unsupported driver error, got %v", err)
	}
}

func TestDatabaseOpenDoesNotMutateConfiguration(t *testing.T) {
	t.Setenv("OWNED_DATABASE_DSN", "file:resolved-without-mutation?mode=memory&cache=shared")
	configuration := &Database{
		Driver: "sqlite",
		Source: "{{.Env.OWNED_DATABASE_DSN}}",
		Registers: []DBResolverConfig{
			{
				Sources:  []string{"{{.Env.OWNED_DATABASE_DSN}}"},
				Replicas: []string{"{{.Env.OWNED_DATABASE_DSN}}"},
				Tables:   []string{"records"},
			},
		},
	}
	before := configuration.resolved()
	before.Source = configuration.Source
	before.Registers[0].Sources[0] = configuration.Registers[0].Sources[0]
	before.Registers[0].Replicas[0] = configuration.Registers[0].Replicas[0]

	handle, err := configuration.Open(context.Background())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer handle.Close()

	if !reflect.DeepEqual(*configuration, before) {
		t.Fatalf("configuration mutated:\n got: %#v\nwant: %#v", *configuration, before)
	}
}

func TestInstallDefaultSwapsCompatibilityHandleWithoutTakingOwnership(t *testing.T) {
	first := openSQLiteHandle(t, "default-first")
	second := openSQLiteHandle(t, "default-second")
	defer first.Close()
	defer second.Close()
	defer ClearDefault(nil)

	if previous := InstallDefault(first); previous != nil {
		t.Fatalf("initial previous handle = %p, want nil", previous)
	}
	if DefaultHandle() != first || DB != first.DB {
		t.Fatal("first handle was not published through compatibility accessors")
	}
	if previous := InstallDefault(second); previous != first {
		t.Fatalf("previous handle = %p, want %p", previous, first)
	}
	if DefaultHandle() != second || DB != second.DB {
		t.Fatal("second handle was not published through compatibility accessors")
	}
	if ClearDefault(first) {
		t.Fatal("stale owner must not clear a newer default handle")
	}
	if !ClearDefault(second) {
		t.Fatal("current owner should clear the default handle")
	}
	if DefaultHandle() != nil || DB != nil || Enforcer != nil {
		t.Fatal("compatibility globals were not cleared")
	}
}

func TestRDSIAMRequiresCompleteConfigurationBeforeLoadingAWS(t *testing.T) {
	configuration := &Database{
		Driver: "mysql",
		IAM: AWSRDSIAM{
			Enable: true,
			Region: "us-east-1",
		},
	}
	if _, err := configuration.Open(context.Background()); err == nil || !strings.Contains(err.Error(), "requires region, user, host, and dbName") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestIAMTLSVerifiesServerIdentityByDefault(t *testing.T) {
	tlsConfig, err := makeIAMTLSConfig(AWSRDSIAM{Host: "db.example.com"})
	if err != nil {
		t.Fatalf("make TLS config: %v", err)
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version = %d, want TLS 1.2", tlsConfig.MinVersion)
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("TLS verification must be enabled by default")
	}
	if tlsConfig.ServerName != "db.example.com" {
		t.Fatalf("server name = %q", tlsConfig.ServerName)
	}
}

func TestIAMMySQLConnectorBuildsTokenForEveryPhysicalConnection(t *testing.T) {
	base, err := makeIAMMySQLConfig(AWSRDSIAM{
		User:   "camera",
		Host:   "db.example.com",
		Port:   3306,
		DBName: "admin",
		Params: "parseTime=true&timeout=3s&tls=skip-verify",
	})
	if err != nil {
		t.Fatalf("make MySQL config: %v", err)
	}
	tlsTemplate, err := makeIAMTLSConfig(AWSRDSIAM{Host: "db.example.com"})
	if err != nil {
		t.Fatalf("make TLS config: %v", err)
	}
	calls := 0
	connector := &iamMySQLConnector{
		region:      "us-east-1",
		base:        base,
		tlsTemplate: tlsTemplate,
		credentials: staticCredentialsProvider{},
		buildToken: func(_ context.Context, endpoint, region, user string, _ aws.CredentialsProvider) (string, error) {
			calls++
			return fmt.Sprintf("token-%d-%s-%s-%s", calls, endpoint, region, user), nil
		},
	}

	first, err := connector.configForConnect(context.Background())
	if err != nil {
		t.Fatalf("first connection config: %v", err)
	}
	second, err := connector.configForConnect(context.Background())
	if err != nil {
		t.Fatalf("second connection config: %v", err)
	}
	if calls != 2 {
		t.Fatalf("token calls = %d, want 2", calls)
	}
	if first.Passwd == second.Passwd {
		t.Fatalf("tokens were reused: %q", first.Passwd)
	}
	if first.TLS == nil || first.TLS.InsecureSkipVerify || first.TLS.ServerName != "db.example.com" {
		t.Fatalf("unexpected TLS config: %#v", first.TLS)
	}
	if first.TLSConfig == "skip-verify" || second.TLSConfig == "skip-verify" {
		t.Fatal("request params must not override verified IAM TLS")
	}
	if !first.ParseTime || first.Timeout.String() != "3s" {
		t.Fatalf("MySQL params were not preserved: %#v", first)
	}
}

func TestIAMPostgresBeforeConnectRefreshesToken(t *testing.T) {
	base, err := pgx.ParseConfig("postgres://camera@db.example.com:5432/admin?sslmode=verify-full")
	if err != nil {
		t.Fatalf("parse pgx config: %v", err)
	}
	tlsTemplate, err := makeIAMTLSConfig(AWSRDSIAM{Host: "db.example.com"})
	if err != nil {
		t.Fatalf("make TLS config: %v", err)
	}
	calls := 0
	beforeConnect := makeIAMPostgresBeforeConnect(
		"us-east-1",
		"",
		tlsTemplate,
		staticCredentialsProvider{},
		func(_ context.Context, endpoint, region, user string, _ aws.CredentialsProvider) (string, error) {
			calls++
			return fmt.Sprintf("token-%d-%s-%s-%s", calls, endpoint, region, user), nil
		},
	)

	first := base.Copy()
	second := base.Copy()
	if err := beforeConnect(context.Background(), first); err != nil {
		t.Fatalf("first before-connect: %v", err)
	}
	if err := beforeConnect(context.Background(), second); err != nil {
		t.Fatalf("second before-connect: %v", err)
	}
	if calls != 2 {
		t.Fatalf("token calls = %d, want 2", calls)
	}
	if first.Password == second.Password {
		t.Fatalf("tokens were reused: %q", first.Password)
	}
	if first.TLSConfig == nil || first.TLSConfig.InsecureSkipVerify || first.TLSConfig.ServerName != "db.example.com" {
		t.Fatalf("unexpected TLS config: %#v", first.TLSConfig)
	}
}

func openSQLiteHandle(t *testing.T, name string) *Handle {
	t.Helper()
	handle, err := (&Database{
		Driver: "sqlite",
		Source: "file:" + name + "?mode=memory&cache=shared",
	}).Open(context.Background())
	if err != nil {
		t.Fatalf("open SQLite handle %s: %v", name, err)
	}
	return handle
}

type staticCredentialsProvider struct{}

func (staticCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
	}, nil
}
