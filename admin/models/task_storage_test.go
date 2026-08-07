package models

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	servertask "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTaskStorageUsesCurrentDatabaseLease(t *testing.T) {
	oldDB := openTaskStorageTestDatabase(t, "old")
	newDB := openTaskStorageTestDatabase(t, "new")
	createTaskStorageFixture(t, oldDB, "old-task")
	createTaskStorageFixture(t, newDB, "new-task")

	current := oldDB
	storage := &TaskStorage{UseDatabase: func(operation func(*gorm.DB) error) error {
		return operation(current)
	}}
	keys, err := storage.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys(old) error = %v", err)
	}
	if want := []string{"old-task"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("ListKeys(old) = %v, want %v", keys, want)
	}

	current = newDB
	keys, err = storage.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys(new) error = %v", err)
	}
	if want := []string{"new-task"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("ListKeys(new) = %v, want %v", keys, want)
	}
}

func TestTaskDeleteHookAllowsCleanupWhenUserSchedulesDisabled(t *testing.T) {
	previous := servertask.UserSchedulesEnabled()
	servertask.New(servertask.WithUserSchedulesEnabled(false))
	t.Cleanup(func() { servertask.New(servertask.WithUserSchedulesEnabled(previous)) })

	if err := (&Task{}).BeforeDelete(nil); err != nil {
		t.Fatalf("BeforeDelete() with disabled user scheduler error = %v", err)
	}
}

func TestTaskStorageGetTreatsMissingAndK8STasksAsAbsent(t *testing.T) {
	db := openTaskStorageTestDatabase(t, "absent")
	createTaskStorageFixtureWithProvider(t, db, "k8s-task", TaskProviderK8S)
	storage := &TaskStorage{DB: db}

	for _, key := range []string{"missing-task", "k8s-task"} {
		_, _, _, exists, err := storage.Get(key)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", key, err)
		}
		if exists {
			t.Fatalf("Get(%q) exists = true, want false", key)
		}
	}
}

func TestTaskDeleteHookAllowsK8STask(t *testing.T) {
	db := openTaskStorageTestDatabase(t, "k8s-delete")
	createTaskStorageFixtureWithProvider(t, db, "k8s-task", TaskProviderK8S)
	previous := servertask.UserSchedulesEnabled()
	servertask.New(
		servertask.WithStorage(&TaskStorage{DB: db}),
		servertask.WithUserSchedulesEnabled(true),
	)
	t.Cleanup(func() { servertask.New(servertask.WithUserSchedulesEnabled(previous)) })

	k8sTask := &Task{Provider: TaskProviderK8S}
	k8sTask.ID = "k8s-task"
	if err := k8sTask.BeforeDelete(nil); err != nil {
		t.Fatalf("BeforeDelete() for k8s task error = %v", err)
	}
}

func TestTaskRunKeepsDatabaseLeaseThroughExecution(t *testing.T) {
	db := openTaskStorageTestDatabase(t, "task-run-lease")
	if err := db.AutoMigrate(&TaskRun{}, &TaskRunLog{}); err != nil {
		t.Fatalf("migrate task run schema: %v", err)
	}
	createTaskStorageFixtureWithProvider(t, db, "blocking-task", TaskProviderFunc)
	if err := db.Model(&Task{}).Where("id = ?", "blocking-task").
		Updates(map[string]any{"method": "blocking-test", "timeout": 5}).Error; err != nil {
		t.Fatalf("configure blocking task: %v", err)
	}

	previousLease := withTaskDatabase
	previousFunc, hadPreviousFunc := TaskFuncMap["blocking-test"]
	t.Cleanup(func() {
		withTaskDatabase = previousLease
		if hadPreviousFunc {
			TaskFuncMap["blocking-test"] = previousFunc
		} else {
			delete(TaskFuncMap, "blocking-test")
		}
	})

	started := make(chan struct{})
	releaseExecution := make(chan struct{})
	leaseReleased := make(chan struct{})
	TaskFuncMap["blocking-test"] = func(context.Context, ...string) error {
		close(started)
		<-releaseExecution
		return nil
	}
	withTaskDatabase = func(operation func(*gorm.DB) error) error {
		err := operation(db)
		close(leaseReleased)
		return err
	}

	taskToRun := &Task{}
	taskToRun.ID = "blocking-task"
	runDone := make(chan struct{})
	go func() {
		taskToRun.Run()
		close(runDone)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task execution did not start")
	}
	select {
	case <-leaseReleased:
		t.Fatal("database lease was released while task execution was active")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseExecution)
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("task execution did not finish")
	}
	select {
	case <-leaseReleased:
	case <-time.After(time.Second):
		t.Fatal("database lease was not released after task execution")
	}

	var run TaskRun
	if err := db.Where("task_id = ?", "blocking-task").First(&run).Error; err != nil {
		t.Fatalf("load task run: %v", err)
	}
	if run.Status != enum.Enabled {
		t.Fatalf("task run status = %q, want %q", run.Status, enum.Enabled)
	}
}

func TestTaskRunWriteUsesLeasedDatabase(t *testing.T) {
	db := openTaskStorageTestDatabase(t, "task-run-writer")
	if err := db.AutoMigrate(&TaskRun{}, &TaskRunLog{}); err != nil {
		t.Fatalf("migrate task run schema: %v", err)
	}
	run := &TaskRun{TaskID: "task", Status: enum.Locked, database: db}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create task run: %v", err)
	}
	if _, err := run.Write([]byte("output")); err != nil {
		t.Fatalf("write task output: %v", err)
	}
	var log TaskRunLog
	if err := db.Where("task_run_id = ?", run.ID).First(&log).Error; err != nil {
		t.Fatalf("load task run log: %v", err)
	}
	if log.Content != "output" {
		t.Fatalf("task run log content = %q, want output", log.Content)
	}
}

func openTaskStorageTestDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), name+".db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open task storage %s database: %v", name, err)
	}
	if err := db.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("migrate task storage %s database: %v", name, err)
	}
	return db
}

func createTaskStorageFixture(t *testing.T, db *gorm.DB, id string) {
	createTaskStorageFixtureWithProvider(t, db, id, TaskProviderDefault)
}

func createTaskStorageFixtureWithProvider(t *testing.T, db *gorm.DB, id string, provider TaskProvider) {
	t.Helper()
	value := &Task{
		Name:      id,
		Namespace: "default",
		Provider:  provider,
		Image:     "default",
		Spec:      "@every 1h",
		Command:   JsonRawMessage("[]"),
		Endpoint:  "localhost",
		Status:    enum.Enabled,
	}
	value.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(value).Error; err != nil {
		t.Fatalf("create task fixture %s: %v", id, err)
	}
}
