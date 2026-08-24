//go:build windows

package dev

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
	developmentLockFailImmediately = 0x00000001
	developmentLockExclusive       = 0x00000002
)

var (
	kernel32DevelopmentLock = syscall.NewLazyDLL("kernel32.dll")
	procDevelopmentLock     = kernel32DevelopmentLock.NewProc("LockFileEx")
	procDevelopmentUnlock   = kernel32DevelopmentLock.NewProc("UnlockFileEx")
	errDevelopmentLockBusy  = syscall.Errno(33)
)

func acquireLifecycleLock(ctx context.Context, path string) (func(), error) {
	file, err := openFileNoFollow(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open development lifecycle lock: %w", err)
	}
	for {
		err = lockDevelopmentFile(file)
		if err == nil {
			return developmentFileUnlocker(file), nil
		}
		if !errors.Is(err, errDevelopmentLockBusy) {
			_ = file.Close()
			return nil, fmt.Errorf("acquire development lifecycle lock: %w", err)
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
			return nil, fmt.Errorf("acquire development lifecycle lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func lockDevelopmentFile(file *os.File) error {
	overlapped := syscall.Overlapped{}
	result, _, callErr := procDevelopmentLock.Call(
		file.Fd(),
		developmentLockExclusive|developmentLockFailImmediately,
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

func developmentFileUnlocker(file *os.File) func() {
	return func() {
		overlapped := syscall.Overlapped{}
		_, _, _ = procDevelopmentUnlock.Call(
			file.Fd(),
			0,
			1,
			0,
			uintptr(unsafe.Pointer(&overlapped)),
		)
		_ = file.Close()
	}
}
