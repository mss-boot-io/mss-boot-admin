//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package blueprint

import (
	"os"
)

func syncCommittedEntry(parent *os.Root, _ string) error {
	directory, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func syncRemovedEntry(parent *os.Root) error {
	return syncCommittedEntry(parent, "")
}

func syncCreatedDirectory(parent *os.Root) error {
	return syncCommittedEntry(parent, "")
}
