//go:build windows

package dev

import (
	"fmt"
	"os"
	"syscall"
)

func openFileNoFollow(path string, flag int, _ os.FileMode) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access := uint32(syscall.GENERIC_READ)
	if flag&os.O_WRONLY != 0 {
		access = syscall.GENERIC_WRITE
	}
	if flag&os.O_RDWR != 0 {
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	creation := uint32(syscall.OPEN_EXISTING)
	switch {
	case flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0:
		creation = syscall.CREATE_NEW
	case flag&os.O_CREATE != 0 && flag&os.O_TRUNC != 0:
		creation = syscall.CREATE_ALWAYS
	case flag&os.O_CREATE != 0:
		creation = syscall.OPEN_ALWAYS
	case flag&os.O_TRUNC != 0:
		creation = syscall.TRUNCATE_EXISTING
	}
	handle, err := syscall.CreateFile(
		name,
		access,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		creation,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = syscall.CloseHandle(handle)
		return nil, fmt.Errorf("open %s without following reparse points", path)
	}
	var information syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &information); err != nil {
		_ = file.Close()
		return nil, err
	}
	if information.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("refusing development file reparse point %s", path)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("refusing non-regular development file %s", path)
	}
	if flag&os.O_APPEND != 0 {
		if _, err := file.Seek(0, 2); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}
