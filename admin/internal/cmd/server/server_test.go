package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	frameworkserver "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
)

type serverManagedQueue struct {
	startErr        error
	startCalls      int
	blockUntilClose bool
	runCalled       chan struct{}
	errors          chan error
}

func (q *serverManagedQueue) String() string { return "server-managed-queue" }

func (q *serverManagedQueue) Append(...storage.Option) error { return nil }

func (q *serverManagedQueue) Register(...storage.Option) {}

func (q *serverManagedQueue) RegisterContext(context.Context, ...storage.Option) error { return nil }

func (q *serverManagedQueue) Run(context.Context) {
	if q.runCalled != nil {
		q.runCalled <- struct{}{}
	}
}

func (q *serverManagedQueue) Shutdown() {}

func (q *serverManagedQueue) Start(ctx context.Context) error {
	q.startCalls++
	if q.blockUntilClose {
		<-ctx.Done()
		return ctx.Err()
	}
	return q.startErr
}

func (q *serverManagedQueue) Errors() <-chan error { return q.errors }

func (q *serverManagedQueue) Close(context.Context) error { return nil }

type serverLegacyQueue struct {
	runCalled chan struct{}
}

func (q *serverLegacyQueue) String() string { return "server-legacy-queue" }

func (q *serverLegacyQueue) Append(...storage.Option) error { return nil }

func (q *serverLegacyQueue) Register(...storage.Option) {}

func (q *serverLegacyQueue) Run(context.Context) { q.runCalled <- struct{}{} }

func (q *serverLegacyQueue) Shutdown() {}

func TestSystemTaskSchedulesKeepInternalJobsWhenUserTasksDisabled(t *testing.T) {
	schedules := systemTaskSchedules(false, "ignored", service.DefaultMonitor, nil)
	if len(schedules) != 2 {
		t.Fatalf("disabled user-task schedules = %d, want monitor and session cleanup", len(schedules))
	}
	if schedules[0].key != "monitor-sampler" ||
		schedules[0].spec != service.DefaultMonitor.ScheduleSpec() ||
		schedules[0].job != service.DefaultMonitor {
		t.Fatalf("monitor system schedule = %#v", schedules[0])
	}
	if schedules[1].key != "session-cleanup" {
		t.Fatalf("second system schedule key = %q, want session-cleanup", schedules[1].key)
	}
}

func TestSystemTaskSchedulesAddDatabaseJobsOnlyWhenEnabled(t *testing.T) {
	const userTaskSpec = "0 */1 * * * *"
	schedules := systemTaskSchedules(true, userTaskSpec, service.DefaultMonitor, func(operation func(*gorm.DB) error) error {
		return operation(&gorm.DB{})
	})
	keys := make([]string, len(schedules))
	for i := range schedules {
		keys[i] = schedules[i].key
	}
	if want := []string{"monitor-sampler", "session-cleanup", "task"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("enabled system-task keys = %v, want %v", keys, want)
	}
	if schedules[2].spec != userTaskSpec {
		t.Fatalf("task reconciliation spec = %q, want %q", schedules[2].spec, userTaskSpec)
	}
}

func TestManagedQueueRunnableParticipatesInManagerErrorPropagation(t *testing.T) {
	wantErr := errors.New("Kafka consumer failed")
	queue := &serverManagedQueue{startErr: wantErr}
	runnable := managedQueueRunnable(queue)
	if runnable == nil || runnable.String() != queue.String() {
		t.Fatalf("managedQueueRunnable() = %#v, want managed queue runtime", runnable)
	}

	manager := frameworkserver.New(frameworkserver.WithoutSignalHandling())
	manager.Add(runnable)
	err := manager.Start(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("manager Start error = %v, want managed queue runtime error", err)
	}
	if queue.startCalls != 1 {
		t.Fatalf("managed queue Start calls = %d, want 1", queue.startCalls)
	}
}

func TestManagedQueueErrorsChannelIsNotLost(t *testing.T) {
	wantErr := errors.New("Kafka consumer-group error")
	queue := &serverManagedQueue{
		blockUntilClose: true,
		errors:          make(chan error, 1),
	}
	queue.errors <- wantErr
	manager := frameworkserver.New(frameworkserver.WithoutSignalHandling())
	manager.Add(managedQueueRunnable(queue))

	err := manager.Start(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("manager Start error = %v, want observed Errors channel value", err)
	}
	if queue.startCalls != 1 {
		t.Fatalf("managed queue Start calls = %d, want 1", queue.startCalls)
	}
}

func TestManagedQueueCompletedStartStillDrainsBufferedError(t *testing.T) {
	wantErr := errors.New("Kafka error buffered before Start returned")
	for iteration := 0; iteration < 100; iteration++ {
		queue := &serverManagedQueue{errors: make(chan error, 1)}
		queue.errors <- wantErr
		close(queue.errors)

		err := managedQueueRunnable(queue).Start(context.Background())
		if !errors.Is(err, wantErr) {
			t.Fatalf("iteration %d: managed Start error = %v, want buffered error", iteration, err)
		}
	}
}

func TestStartLegacyQueueDoesNotDetachManagedQueue(t *testing.T) {
	queue := &serverManagedQueue{runCalled: make(chan struct{}, 1)}
	startLegacyQueue(context.Background(), queue)
	select {
	case <-queue.runCalled:
		t.Fatal("managed queue was started through detached legacy Run")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestStartLegacyQueueRetainsNonManagedCompatibility(t *testing.T) {
	queue := &serverLegacyQueue{runCalled: make(chan struct{}, 1)}
	startLegacyQueue(context.Background(), queue)
	select {
	case <-queue.runCalled:
	case <-time.After(time.Second):
		t.Fatal("legacy queue compatibility Run was not started")
	}
}

func TestBusinessRoutesMountOnlyAfterCanonicalEmailSchemaReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name          string
		recordVersion bool
		wantErr       bool
		wantStatus    int
	}{
		{
			name:       "missing migration record fails closed",
			wantErr:    true,
			wantStatus: http.StatusNotFound,
		},
		{
			name:          "ready schema mounts route",
			recordVersion: true,
			wantStatus:    http.StatusNoContent,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openServerSchemaReadinessSQLite(t)
			createServerCanonicalEmailSchema(t, db, test.recordVersion)
			engine := gin.New()
			groups, err := mountCoreRouteGroupsAfterSchemaReadiness(
				t.Context(),
				db,
				engine.Group("/admin"),
			)
			if test.wantErr {
				if !errors.Is(err, schemahealth.ErrCanonicalEmailIdentityNotReady) {
					t.Fatalf("route composition error = %v, want schema readiness failure", err)
				}
			} else if err != nil {
				t.Fatalf("route composition failed: %v", err)
			} else {
				groups.ProtectedAPI.GET("/readiness-marker", func(ctx *gin.Context) {
					ctx.Status(http.StatusNoContent)
				})
			}

			request := httptest.NewRequest(http.MethodGet, "/admin/api/readiness-marker", nil)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("route status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func openServerSchemaReadinessSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "server-schema-readiness.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createServerCanonicalEmailSchema(t *testing.T, db *gorm.DB, recordVersion bool) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id TEXT PRIMARY KEY,
		email TEXT NULL,
		deleted_at DATETIME NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
			" ON mss_boot_users (LOWER(TRIM(email)))" +
			" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal(err)
	}
	if !recordVersion {
		return
	}
	version := &migrationmodels.Migration{}
	version.SetVersion(schemahealth.CanonicalEmailIdentityMigrationVersion)
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
}

var _ storage.ManagedAdapterQueue = (*serverManagedQueue)(nil)
var _ storage.AdapterQueue = (*serverLegacyQueue)(nil)
