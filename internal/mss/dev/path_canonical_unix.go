//go:build !windows

package dev

import "path/filepath"

func canonicalExistingPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
