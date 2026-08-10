//go:build windows

package blueprint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

var (
	kernel32SnapshotLock = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx       = kernel32SnapshotLock.NewProc("LockFileEx")
	procUnlockFileEx     = kernel32SnapshotLock.NewProc("UnlockFileEx")
	errLockViolation     = syscall.Errno(33)
)

func acquireSnapshotWriter(ctx context.Context, root *managedRoot) (func(), error) {
	file, err := root.openLockFile()
	if err != nil {
		return nil, fmt.Errorf("open snapshot writer lock: %w", err)
	}
	for {
		err = lockSnapshotFile(file, lockfileExclusiveLock|lockfileFailImmediately)
		if err == nil {
			if err := root.verifyLockFile(file); err != nil {
				snapshotFileUnlocker(file)()
				return nil, fmt.Errorf("verify snapshot writer lock: %w", err)
			}
			return snapshotFileUnlocker(file), nil
		}
		if !errors.Is(err, errLockViolation) {
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
	file, err := root.openLockFile()
	if err != nil {
		return nil, fmt.Errorf("open snapshot reader lock: %w", err)
	}
	if err := lockSnapshotFile(file, 0); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("acquire snapshot reader lock: %w", err)
	}
	if err := root.verifyLockFile(file); err != nil {
		snapshotFileUnlocker(file)()
		return nil, fmt.Errorf("verify snapshot reader lock: %w", err)
	}
	return snapshotFileUnlocker(file), nil
}

func lockSnapshotFile(file *os.File, flags uint32) error {
	overlapped := syscall.Overlapped{}
	result, _, callErr := procLockFileEx.Call(
		file.Fd(),
		uintptr(flags),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if result != 0 {
		return nil
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}

func snapshotFileUnlocker(file *os.File) func() {
	return func() {
		overlapped := syscall.Overlapped{}
		_, _, _ = procUnlockFileEx.Call(
			file.Fd(),
			0,
			1,
			0,
			uintptr(unsafe.Pointer(&overlapped)),
		)
		_ = file.Close()
	}
}
