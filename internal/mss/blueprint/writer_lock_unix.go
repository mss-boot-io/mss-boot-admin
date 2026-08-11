//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package blueprint

import (
	"context"
	"fmt"
	"syscall"
	"time"
)

func acquireSnapshotWriter(ctx context.Context, root *managedRoot) (func(), error) {
	file, err := root.openLockFile()
	if err != nil {
		return nil, fmt.Errorf("open snapshot writer lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if err := root.verifyLockFile(file); err != nil {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
				return nil, fmt.Errorf("verify snapshot writer lock: %w", err)
			}
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = file.Close()
			return nil, fmt.Errorf("acquire snapshot writer lock: %w", err)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			_ = file.Close()
			return nil, fmt.Errorf("acquire snapshot writer lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func acquireSnapshotReader(root *managedRoot) (func(), error) {
	// Readers create and open the same inode as writers. Returning without a
	// lock when the file is initially absent permits a first-reader/first-writer
	// race at the lock-first/manifest-last commit boundary.
	file, err := root.openLockFile()
	if err != nil {
		return nil, fmt.Errorf("open snapshot reader lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH)
		if err == nil {
			if err := root.verifyLockFile(file); err != nil {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
				return nil, fmt.Errorf("verify snapshot reader lock: %w", err)
			}
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if err != syscall.EINTR {
			_ = file.Close()
			return nil, fmt.Errorf("acquire snapshot reader lock: %w", err)
		}
	}
}
