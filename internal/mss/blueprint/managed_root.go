package blueprint

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	snapshotLockPath    = snapshotRuntimePrefix + "blueprint-snapshot.lock"
	snapshotJournalPath = snapshotRuntimePrefix + "blueprint-snapshot.txn"
)

// managedRoot pins the downstream repository directory. All mutation paths
// are resolved relative to this handle so a concurrent rename or symlink swap
// cannot redirect a write outside the selected repository.
type managedRoot struct {
	path string
	root *os.Root
}

func openManagedRoot(path string, create bool) (*managedRoot, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve managed repository root: %w", err)
	}
	absolute = filepath.Clean(absolute)
	root, err := openVerifiedRoot(absolute)
	if err == nil {
		return &managedRoot{path: absolute, root: root}, nil
	}
	if !create || !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	ancestor := absolute
	var missing []string
	for {
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil, fmt.Errorf("find existing ancestor for managed repository root %s", absolute)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect managed repository ancestor %s: %w", ancestor, statErr)
		}
	}

	current, err := openVerifiedRoot(ancestor)
	if err != nil {
		return nil, err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		name := missing[index]
		info, statErr := current.Lstat(name)
		switch {
		case errors.Is(statErr, os.ErrNotExist):
			if err := current.Mkdir(name, 0o755); err != nil {
				_ = current.Close()
				return nil, fmt.Errorf("create managed repository directory %s: %w", name, err)
			}
			if err := syncCreatedDirectory(current); err != nil {
				_ = current.Close()
				return nil, fmt.Errorf("sync managed repository directory %s: %w", name, err)
			}
			info, statErr = current.Lstat(name)
		case statErr != nil:
			_ = current.Close()
			return nil, fmt.Errorf("inspect managed repository directory %s: %w", name, statErr)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("managed repository path component is not a real directory: %s", name)
		}
		next, err := current.OpenRoot(name)
		if err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("open managed repository directory %s: %w", name, err)
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = next.Close()
			_ = current.Close()
			if err == nil {
				err = errors.New("directory identity changed while it was opened")
			}
			return nil, fmt.Errorf("pin managed repository directory %s: %w", name, err)
		}
		_ = current.Close()
		current = next
	}
	return &managedRoot{path: absolute, root: current}, nil
}

func openVerifiedRoot(path string) (*os.Root, error) {
	volume := filepath.VolumeName(path)
	anchor := volume + string(filepath.Separator)
	if volume == "" {
		anchor = string(filepath.Separator)
	}
	relative, err := filepath.Rel(anchor, path)
	if err != nil {
		return nil, fmt.Errorf("resolve managed repository root from filesystem anchor: %w", err)
	}
	current, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, fmt.Errorf("open managed repository filesystem anchor: %w", err)
	}
	if relative == "." {
		return current, nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			_ = current.Close()
			return nil, errors.New("managed repository root contains an invalid path component")
		}
		info, err := current.Lstat(part)
		if err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("inspect managed repository root component %s: %w", part, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("managed repository root component is not a real directory: %s", part)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("open managed repository root component %s: %w", part, err)
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = next.Close()
			_ = current.Close()
			if err == nil {
				err = errors.New("directory identity changed while it was opened")
			}
			return nil, fmt.Errorf("pin managed repository root component %s: %w", part, err)
		}
		_ = current.Close()
		current = next
	}
	return current, nil
}

func (root *managedRoot) Close() error {
	if root == nil || root.root == nil {
		return nil
	}
	return root.root.Close()
}

func (root *managedRoot) pathStillPinned() error {
	info, err := os.Lstat(root.path)
	if err != nil {
		return fmt.Errorf("inspect managed repository path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("managed repository path no longer names a real directory")
	}
	opened, err := root.root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return errors.New("managed repository path changed during the operation")
	}
	return nil
}

func confinedManagedPath(relative string) (string, error) {
	normalized := normalizedPath(relative)
	if !safeRelativePath(normalized) || normalized != strings.TrimSpace(filepath.ToSlash(relative)) {
		return "", fmt.Errorf("managed path is unsafe or non-canonical: %s", relative)
	}
	return filepath.FromSlash(normalized), nil
}

// openParent pins every directory component and rejects symlink parents. The
// returned Root references the real parent even if its directory entry is
// concurrently renamed after this function returns.
func (root *managedRoot) openParent(relative string, create bool) (*os.Root, string, error) {
	name, err := confinedManagedPath(relative)
	if err != nil {
		return nil, "", err
	}
	parts := strings.Split(name, string(filepath.Separator))
	current, err := root.root.OpenRoot(".")
	if err != nil {
		return nil, "", err
	}
	for _, part := range parts[:len(parts)-1] {
		info, statErr := current.Lstat(part)
		switch {
		case errors.Is(statErr, os.ErrNotExist) && create:
			if err := current.Mkdir(part, 0o755); err != nil {
				_ = current.Close()
				return nil, "", fmt.Errorf("create managed parent for %s: %w", relative, err)
			}
			if err := syncCreatedDirectory(current); err != nil {
				_ = current.Close()
				return nil, "", fmt.Errorf("sync managed parent for %s: %w", relative, err)
			}
			info, statErr = current.Lstat(part)
		case errors.Is(statErr, os.ErrNotExist):
			_ = current.Close()
			return nil, "", os.ErrNotExist
		case statErr != nil:
			_ = current.Close()
			return nil, "", statErr
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = current.Close()
			return nil, "", fmt.Errorf("managed path has a non-directory or symlink parent: %s", relative)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			_ = current.Close()
			return nil, "", err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = next.Close()
			_ = current.Close()
			if err == nil {
				err = errors.New("parent identity changed while it was opened")
			}
			return nil, "", fmt.Errorf("pin managed parent for %s: %w", relative, err)
		}
		_ = current.Close()
		current = next
	}
	return current, parts[len(parts)-1], nil
}

func (root *managedRoot) readFile(relative string) ([]byte, bool, fs.FileMode, error) {
	parent, base, err := root.openParent(relative, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, 0, nil
	}
	if err != nil {
		return nil, false, 0, err
	}
	defer parent.Close()
	info, err := parent.Lstat(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, 0, nil
	}
	if err != nil {
		return nil, false, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("managed path is not a regular file: %s", relative)
	}
	file, err := parent.Open(base)
	if err != nil {
		return nil, false, 0, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, false, 0, err
	}
	current, err := parent.Lstat(base)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		if err == nil {
			err = errors.New("file identity changed while it was opened")
		}
		return nil, false, 0, fmt.Errorf("pin managed file %s: %w", relative, err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false, 0, err
	}
	return data, true, opened.Mode().Perm(), nil
}

func (root *managedRoot) openLockFile() (*os.File, error) {
	parent, base, err := root.openParent(snapshotLockPath, true)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if info, statErr := parent.Lstat(base); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, errors.New("snapshot lock path is not a regular file")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	file, err := parent.OpenFile(base, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	current, err := parent.Lstat(base)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		_ = file.Close()
		if err == nil {
			err = errors.New("lock identity changed while it was opened")
		}
		return nil, fmt.Errorf("pin snapshot lock file: %w", err)
	}
	return file, nil
}

func (root *managedRoot) verifyLockFile(file *os.File) error {
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	parent, base, err := root.openParent(snapshotLockPath, false)
	if err != nil {
		return err
	}
	defer parent.Close()
	current, err := parent.Lstat(base)
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return errors.New("snapshot lock path was replaced while acquiring the lock")
	}
	return nil
}

func (root *managedRoot) createTemporary(parent *os.Root, prefix string, mode fs.FileMode) (*os.File, string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := prefix + hex.EncodeToString(random[:])
		file, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, mode.Perm())
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return file, name, err
	}
	return nil, "", errors.New("could not allocate a unique managed temporary file")
}

func (root *managedRoot) writeAtomic(relative string, data []byte, mode fs.FileMode) error {
	parent, base, err := root.openParent(relative, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	temporary, name, err := root.createTemporary(parent, ".mss-write-", mode)
	if err != nil {
		return err
	}
	cleanup := func(cause error) error {
		_ = temporary.Close()
		_ = parent.Remove(name)
		return cause
	}
	if err := temporary.Chmod(mode.Perm()); err != nil {
		return cleanup(err)
	}
	if _, err := temporary.Write(data); err != nil {
		return cleanup(err)
	}
	if err := temporary.Sync(); err != nil {
		return cleanup(err)
	}
	if err := temporary.Close(); err != nil {
		_ = parent.Remove(name)
		return err
	}
	if current, statErr := parent.Lstat(base); statErr == nil && current.Mode()&os.ModeSymlink != 0 {
		_ = parent.Remove(name)
		return fmt.Errorf("managed target is a symlink: %s", relative)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		_ = parent.Remove(name)
		return statErr
	}
	if err := parent.Rename(name, base); err != nil {
		_ = parent.Remove(name)
		return err
	}
	return syncCommittedEntry(parent, base)
}

func (root *managedRoot) removeFile(relative string) error {
	parent, base, err := root.openParent(relative, false)
	if errors.Is(err, os.ErrNotExist) {
		return os.ErrNotExist
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(base)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("managed path is not a regular file: %s", relative)
	}
	if err := parent.Remove(base); err != nil {
		return err
	}
	return syncRemovedEntry(parent)
}

func (root *managedRoot) ensureDirectory(relative string, mode fs.FileMode) error {
	parent, base, err := root.openParent(relative, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(base)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := parent.Mkdir(base, mode.Perm()); err != nil {
			return err
		}
		return syncCreatedDirectory(parent)
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0 || !info.IsDir():
		return fmt.Errorf("managed directory path is not a real directory: %s", relative)
	default:
		return nil
	}
}

func (root *managedRoot) removeTree(relative string) error {
	name, err := confinedManagedPath(relative)
	if err != nil {
		return err
	}
	if err := root.root.RemoveAll(name); err != nil {
		return err
	}
	return syncRemovedEntry(root.root)
}

func (root *managedRoot) beginSnapshotTransaction(operation string) (func() error, error) {
	parent, base, err := root.openParent(snapshotJournalPath, true)
	if err != nil {
		return nil, err
	}
	file, err := parent.OpenFile(base, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		_ = parent.Close()
		return nil, errors.New("an incomplete blueprint snapshot transaction requires recovery")
	}
	if err != nil {
		_ = parent.Close()
		return nil, err
	}
	if _, err := io.WriteString(file, operation+"\n"); err != nil {
		_ = file.Close()
		_ = parent.Remove(base)
		_ = parent.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = parent.Remove(base)
		_ = parent.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = parent.Remove(base)
		_ = parent.Close()
		return nil, err
	}
	if err := syncCommittedEntry(parent, base); err != nil {
		_ = parent.Remove(base)
		_ = parent.Close()
		return nil, err
	}
	return func() error {
		defer parent.Close()
		if err := parent.Remove(base); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncRemovedEntry(parent)
	}, nil
}

func (root *managedRoot) checkSnapshotTransaction() error {
	_, exists, _, err := root.readFile(snapshotJournalPath)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("an incomplete blueprint snapshot transaction requires recovery")
	}
	return nil
}

func (root *managedRoot) removeEmptyParents(relativeDirectory string) {
	relativeDirectory = normalizedPath(relativeDirectory)
	for relativeDirectory != "." && relativeDirectory != "" {
		parent, base, err := root.openParent(relativeDirectory, false)
		if err != nil {
			return
		}
		directory, err := parent.Open(base)
		if err != nil {
			_ = parent.Close()
			return
		}
		entries, err := directory.ReadDir(1)
		_ = directory.Close()
		if err == nil || (err != nil && !errors.Is(err, io.EOF)) || len(entries) != 0 {
			_ = parent.Close()
			return
		}
		if err := parent.Remove(base); err != nil {
			_ = parent.Close()
			return
		}
		_ = syncRemovedEntry(parent)
		_ = parent.Close()
		relativeDirectory = normalizedPath(filepath.Dir(filepath.FromSlash(relativeDirectory)))
	}
}

func (root *managedRoot) unknownFiles(desired map[string]desiredFile) ([]string, error) {
	var unknown []string
	err := fs.WalkDir(root.root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(filepath.ToSlash(path), "./")
		if relative == "" || relative == "." {
			return nil
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git/") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if relative == strings.TrimSuffix(snapshotRuntimePrefix, "/") || strings.HasPrefix(relative, snapshotRuntimePrefix) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if _, exists := desired[relative]; !exists {
			unknown = append(unknown, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(unknown)
	return unknown, nil
}
