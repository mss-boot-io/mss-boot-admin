package config

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gopkg.in/yaml.v3"
)

func TestSQLiteConfigurationsUseSupportedConnectionPragmas(t *testing.T) {
	for _, filename := range []string{"application.yml", "application-e2e.yml"} {
		t.Run(filename, func(t *testing.T) {
			assertSupportedSQLiteParameters(t, readSQLiteSource(t, filename))
		})
	}
}

func TestE2ESQLiteUsesConfiguredPragmasAndImmediateTransactions(t *testing.T) {
	parsed := readSQLiteSource(t, "application-e2e.yml")
	assertSupportedSQLiteParameters(t, parsed)

	parsed.Path = filepath.ToSlash(filepath.Join(t.TempDir(), "e2e.db"))
	handle, err := (&gormdb.Database{
		Driver:       "sqlite",
		Source:       parsed.String(),
		MaxOpenConns: 4,
		MaxIdleConns: 4,
	}).Open(context.Background())
	if err != nil {
		t.Fatalf("open e2e SQLite connection: %v", err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Errorf("close e2e SQLite connection: %v", err)
		}
	})
	if err := handle.DB.Exec("CREATE TABLE e2e_connection_probe (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create e2e SQLite probe table: %v", err)
	}

	connections := make([]interface{ Close() error }, 0, 2)
	first, err := handle.SQLDB().Conn(context.Background())
	if err != nil {
		t.Fatalf("acquire first e2e SQLite connection: %v", err)
	}
	connections = append(connections, first)
	second, err := handle.SQLDB().Conn(context.Background())
	if err != nil {
		t.Fatalf("acquire second e2e SQLite connection: %v", err)
	}
	connections = append(connections, second)
	t.Cleanup(func() {
		for _, connection := range connections {
			if err := connection.Close(); err != nil {
				t.Errorf("close e2e SQLite leased connection: %v", err)
			}
		}
	})
	for index, connection := range []*sql.Conn{first, second} {
		var journalMode string
		if err := connection.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("read journal mode on connection %d: %v", index+1, err)
		}
		if journalMode != "wal" {
			t.Errorf("journal mode on connection %d = %q, want wal", index+1, journalMode)
		}
		var busyTimeout int
		if err := connection.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("read busy timeout on connection %d: %v", index+1, err)
		}
		if busyTimeout != 30000 {
			t.Errorf("busy timeout on connection %d = %d, want 30000", index+1, busyTimeout)
		}
	}

	firstTransaction, err := first.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin first e2e SQLite transaction: %v", err)
	}
	t.Cleanup(func() { _ = firstTransaction.Rollback() })
	type beginResult struct {
		transaction *sql.Tx
		err         error
	}
	secondAttempting := make(chan struct{})
	secondBegan := make(chan beginResult, 1)
	go func() {
		close(secondAttempting)
		transaction, beginErr := second.BeginTx(context.Background(), nil)
		secondBegan <- beginResult{transaction: transaction, err: beginErr}
	}()
	<-secondAttempting
	select {
	case result := <-secondBegan:
		if result.transaction != nil {
			_ = result.transaction.Rollback()
		}
		t.Fatalf("second SQLite transaction began before the first committed: %v", result.err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := firstTransaction.Commit(); err != nil {
		t.Fatalf("commit first e2e SQLite transaction: %v", err)
	}
	select {
	case result := <-secondBegan:
		if result.err != nil {
			t.Fatalf("begin second e2e SQLite transaction after release: %v", result.err)
		}
		if err := result.transaction.Rollback(); err != nil {
			t.Fatalf("roll back second e2e SQLite transaction: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second e2e SQLite transaction did not resume after the first committed")
	}
}

func readSQLiteSource(t *testing.T, filename string) *url.URL {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	var configuration struct {
		Database struct {
			Driver string `yaml:"driver"`
			Source string `yaml:"source"`
		} `yaml:"database"`
	}
	if err := yaml.Unmarshal(data, &configuration); err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	if configuration.Database.Driver != "sqlite" {
		t.Fatalf("%s database driver = %q, want sqlite", filename, configuration.Database.Driver)
	}
	parsed, err := url.Parse(configuration.Database.Source)
	if err != nil {
		t.Fatalf("parse %s SQLite source: %v", filename, err)
	}
	return parsed
}

func assertSupportedSQLiteParameters(t *testing.T, parsed *url.URL) {
	t.Helper()
	want := map[string]bool{
		"busy_timeout(30000)": false,
		"journal_mode(WAL)":   false,
	}
	for _, pragma := range parsed.Query()["_pragma"] {
		if _, required := want[pragma]; required {
			want[pragma] = true
		}
	}
	for pragma, present := range want {
		if !present {
			t.Errorf("SQLite source is missing supported _pragma=%s", pragma)
		}
	}
	for _, unsupported := range []string{"_busy_timeout", "_journal_mode"} {
		if parsed.Query().Has(unsupported) {
			t.Errorf("SQLite source uses unsupported connection parameter %s", unsupported)
		}
	}
	if got := parsed.Query().Get("_txlock"); got != "immediate" {
		t.Errorf("SQLite transaction lock = %q, want immediate", got)
	}
}
