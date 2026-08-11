//go:build windows

package blueprint

import "os"

// Windows rejects FlushFileBuffers on directory handles. The staged file was
// flushed before the atomic rename, so flush the committed file handle after
// the rename instead of treating an unsupported directory flush as success.
func syncCommittedEntry(parent *os.Root, name string) error {
	// FlushFileBuffers, which os.File.Sync uses on Windows, rejects the
	// read-only handle returned by Root.Open. Re-open the committed regular
	// file for read/write so the post-rename flush is supported by NTFS/ReFS.
	file, err := parent.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

// NTFS/ReFS do not expose a portable directory FlushFileBuffers operation via
// os.Root. Removal remains atomic; the snapshot journal makes a crash between
// managed mutations and the final manifest detectable on the next read.
func syncRemovedEntry(_ *os.Root) error {
	return nil
}

func syncCreatedDirectory(_ *os.Root) error {
	return nil
}
