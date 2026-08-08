package gormdb

import (
	"errors"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
)

// synchronizedEnforcer keeps Casbin policy reads and reloads behind Casbin's
// SyncedEnforcer lock. It also retains the configured watcher so policy writes
// performed transactionally through GORM can publish one checked notification
// after the database commit.
type synchronizedEnforcer struct {
	*casbin.SyncedEnforcer

	watcherMu sync.RWMutex
	watcher   persist.Watcher
}

func newSynchronizedEnforcer(params ...interface{}) (*synchronizedEnforcer, error) {
	enforcer, err := casbin.NewSyncedEnforcer(params...)
	if err != nil {
		return nil, err
	}
	return &synchronizedEnforcer{SyncedEnforcer: enforcer}, nil
}

func (e *synchronizedEnforcer) SetWatcher(watcher persist.Watcher) error {
	if e == nil || e.SyncedEnforcer == nil {
		return errors.New("gormdb: Casbin enforcer is nil")
	}
	if watcher == nil {
		return errors.New("gormdb: Casbin watcher is nil")
	}
	if err := e.SyncedEnforcer.SetWatcher(watcher); err != nil {
		return err
	}
	e.watcherMu.Lock()
	e.watcher = watcher
	e.watcherMu.Unlock()
	return nil
}

func (e *synchronizedEnforcer) notifyWatcher() error {
	if e == nil {
		return errors.New("gormdb: Casbin enforcer is nil")
	}
	e.watcherMu.RLock()
	watcher := e.watcher
	e.watcherMu.RUnlock()
	if watcher == nil {
		return nil
	}
	return watcher.Update()
}

func installedEnforcer() casbin.IEnforcer {
	if handle := DefaultHandle(); handle != nil && handle.Enforcer != nil {
		return handle.Enforcer
	}
	return Enforcer
}

// ReloadPolicy reloads the installed process-wide policy using the synchronized
// enforcer. Callers receive the adapter error instead of silently continuing
// with stale authorization state.
func ReloadPolicy() error {
	enforcer := installedEnforcer()
	if enforcer == nil {
		return errors.New("gormdb: Casbin enforcer is not configured")
	}
	return enforcer.LoadPolicy()
}

// NotifyPolicyWatcher publishes a policy-change notification when a watcher is
// configured. An application without a watcher remains a valid single-process
// deployment; durable revision reconciliation still covers missed events.
func NotifyPolicyWatcher() error {
	enforcer := installedEnforcer()
	if enforcer == nil {
		return errors.New("gormdb: Casbin enforcer is not configured")
	}
	if notifier, ok := enforcer.(interface{ notifyWatcher() error }); ok {
		return notifier.notifyWatcher()
	}
	return nil
}

// ReloadPolicyAndNotify makes a committed policy update effective locally
// before asking peer processes to reconcile their own policy snapshots.
func ReloadPolicyAndNotify() error {
	if err := ReloadPolicy(); err != nil {
		return err
	}
	return NotifyPolicyWatcher()
}
