//go:build windows

package dev

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func canonicalExistingPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	name, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		name,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	size := uint32(512)
	for {
		buffer := make([]uint16, size)
		length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], size, 0)
		if err != nil {
			return "", err
		}
		if length >= size {
			size = length + 1
			continue
		}
		resolved := windows.UTF16ToString(buffer[:length])
		switch {
		case len(resolved) >= len(`\\?\UNC\`) && strings.EqualFold(resolved[:len(`\\?\UNC\`)], `\\?\UNC\`):
			resolved = `\\` + resolved[len(`\\?\UNC\`):]
		case isExtendedDOSPath(resolved):
			resolved = resolved[len(`\\?\`):]
		case strings.HasPrefix(resolved, `\\?\`):
			return "", fmt.Errorf("unsupported Windows extended path namespace %q", resolved)
		}
		return filepath.Clean(resolved), nil
	}
}

func isExtendedDOSPath(path string) bool {
	if len(path) < len(`\\?\C:\`) || !strings.HasPrefix(path, `\\?\`) {
		return false
	}
	drive := path[len(`\\?\`)]
	return ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) &&
		path[len(`\\?\`)+1] == ':' && path[len(`\\?\`)+2] == '\\'
}
