package server

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	frameworkserver "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
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

var _ storage.ManagedAdapterQueue = (*serverManagedQueue)(nil)
var _ storage.AdapterQueue = (*serverLegacyQueue)(nil)
