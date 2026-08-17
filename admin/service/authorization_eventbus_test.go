package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	runtimeeventbus "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/eventbus"
)

func TestCasbinRevisionReconcilesCommitPublishCrash(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	var reloads atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return nil },
		func() error { t.Fatal("legacy watcher must not be used by the EventBus runtime"); return nil },
	)
	var databaseUses atomic.Int64
	runtime, err := BuildMemoryAuthorizationEventRuntime(
		svc,
		func(ctx context.Context, operation func(*gorm.DB) error) error {
			databaseUses.Add(1)
			return operation(db.WithContext(ctx))
		},
		2*time.Millisecond,
	)
	require.NoError(t, err)
	require.Zero(t, databaseUses.Load(), "Build must not read the database")
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	// Model a process crash after the authoritative transaction commits but
	// before it can publish the new revision.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		current, lockErr := lockConfigRevision(tx, globalAuthorizationRevisionKey())
		if lockErr != nil {
			return lockErr
		}
		if createErr := tx.Create(&models.CasbinRule{
			PType: "p",
			V0:    "role-a",
			V1:    adminpkg.MenuAccessType.String(),
			V2:    "/committed-without-publish",
			V3:    "GET",
		}).Error; createErr != nil {
			return createErr
		}
		_, advanceErr := advanceConfigRevision(tx, globalAuthorizationRevisionKey(), current)
		return advanceErr
	}))
	require.Zero(t, reloads.Load())

	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- runtime.Start(runCtx) }()
	require.Eventually(t, func() bool { return reloads.Load() == 1 }, time.Second, time.Millisecond)
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	require.Equal(t, runtimeeventbus.Revision(1), runtime.bus.LastRevision())

	// An equal duplicate and an older notification cannot trigger another
	// reload after the subscriber has successfully observed revision 1.
	require.NoError(t, runtime.bus.Publish(context.Background(), runtimeeventbus.Event[AuthorizationRevisionEvent]{
		Revision: 1,
		Payload:  AuthorizationRevisionEvent{},
	}))
	require.NoError(t, runtime.bus.Publish(context.Background(), runtimeeventbus.Event[AuthorizationRevisionEvent]{
		Revision: 1,
		Payload:  AuthorizationRevisionEvent{},
	}))
	require.Equal(t, int64(1), reloads.Load())

	cancel()
	require.NoError(t, <-runDone)
}

func TestAuthorizationMutationPublishesCommittedRevisionWithoutWorkQueueWatcher(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db, authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""))
	var reloads atomic.Int64
	var watcherNotifications atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return nil },
		func() error { watcherNotifications.Add(1); return nil },
	)
	runtime, err := BuildMemoryAuthorizationEventRuntime(svc, directAuthorizationDatabase(db), time.Hour)
	require.NoError(t, err)
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", resource.Revision)
	require.Equal(t, int64(1), reloads.Load())
	require.Zero(t, watcherNotifications.Load(), "bound EventBus must bypass legacy WorkQueue watcher")
	require.Equal(t, runtimeeventbus.Revision(1), runtime.bus.LastRevision())
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND v2 = ?", "role-a", "/a").
		Count(&policyCount).Error)
	require.Equal(t, int64(1), policyCount)
}

func TestAuthorizationRevisionNotifierRunsAfterReloadAndCannotPoisonMutation(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db, authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""))
	var reloads atomic.Int64
	var notified atomic.Uint64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return nil },
		func() error { t.Fatal("legacy watcher must not run"); return nil },
	)
	runtime, err := BuildMemoryAuthorizationEventRuntime(
		svc,
		directAuthorizationDatabase(db),
		time.Hour,
		WithAuthorizationRevisionNotifier(func(revision uint64) {
			if reloads.Load() == 0 {
				t.Error("revision notification ran before policy reload")
			}
			notified.Store(revision)
			panic("optional-realtime-notifier")
		}),
	)
	require.NoError(t, err)
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.NoError(t, err, "optional notifier panic must not fail a committed mutation")
	require.Equal(t, "1", resource.Revision)
	require.Equal(t, uint64(1), notified.Load())
	require.Equal(t, runtimeeventbus.Revision(1), runtime.bus.LastRevision())
}

func TestAuthorizationSubscriberPanicIsolatedAndReconciled(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db, authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""))
	var reloads atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error {
			if reloads.Add(1) == 1 {
				panic("private-enforcer-panic")
			}
			return nil
		},
		func() error { t.Fatal("legacy watcher must not run"); return nil },
	)
	runtime, err := BuildMemoryAuthorizationEventRuntime(svc, directAuthorizationDatabase(db), time.Hour)
	require.NoError(t, err)
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.Equal(t, "1", resource.Revision)
	var propagation *AuthorizationPropagationError
	require.ErrorAs(t, err, &propagation)
	require.ErrorIs(t, err, runtimeeventbus.ErrSubscriberPanicked)
	require.NotContains(t, err.Error(), "private-enforcer-panic")
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))

	require.NoError(t, runtime.Reconcile(context.Background()))
	require.Equal(t, int64(2), reloads.Load())
	require.NoError(t, runtime.Health(context.Background()))
}

func TestAuthorizationEventBusOutagePreservesCommittedPolicy(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db, authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""))
	var watcherNotifications atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { return nil },
		func() error { watcherNotifications.Add(1); return nil },
	)
	runtime, err := BuildMemoryAuthorizationEventRuntime(svc, directAuthorizationDatabase(db), time.Hour)
	require.NoError(t, err)
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	// Keep the configured publisher bound while making its provider unavailable.
	require.NoError(t, runtime.bus.Close(context.Background()))
	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.Equal(t, "1", resource.Revision)
	var propagation *AuthorizationPropagationError
	require.ErrorAs(t, err, &propagation)
	require.ErrorIs(t, err, runtimeeventbus.ErrClosing)
	require.Zero(t, watcherNotifications.Load(), "configured EventBus outage must not fall back to WorkQueue")
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND v2 = ?", "role-a", "/a").
		Count(&policyCount).Error)
	require.Equal(t, int64(1), policyCount, "bus outage must not roll back committed policy")
}

func directAuthorizationDatabase(db *gorm.DB) AuthorizationDatabaseUse {
	return func(ctx context.Context, operation func(*gorm.DB) error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return operation(db.WithContext(ctx))
	}
}

func TestAuthorizationEventRuntimeRejectsSecondOwner(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	first, err := BuildMemoryAuthorizationEventRuntime(svc, directAuthorizationDatabase(db), time.Hour)
	require.NoError(t, err)
	second, err := BuildMemoryAuthorizationEventRuntime(svc, directAuthorizationDatabase(db), time.Hour)
	require.NoError(t, err)
	require.NoError(t, first.Open(context.Background()))
	t.Cleanup(func() { _ = first.Close(context.Background()) })
	require.ErrorIs(t, second.Open(context.Background()), ErrAuthorizationEventBusBound)
	require.NoError(t, second.Close(context.Background()))
}

func TestAuthorizationEventRuntimeHealthRecordsAndRecoversReconcileFailure(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	var fail atomic.Bool
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	runtime, err := BuildMemoryAuthorizationEventRuntime(
		svc,
		func(ctx context.Context, operation func(*gorm.DB) error) error {
			if fail.Load() {
				return errors.New("temporary database failure")
			}
			return operation(db.WithContext(ctx))
		},
		time.Hour,
	)
	require.NoError(t, err)
	require.NoError(t, runtime.Open(context.Background()))
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	fail.Store(true)
	require.Error(t, runtime.Reconcile(context.Background()))
	require.Error(t, runtime.Health(context.Background()))
	fail.Store(false)
	require.NoError(t, runtime.Reconcile(context.Background()))
	require.NoError(t, runtime.Health(context.Background()))
}
