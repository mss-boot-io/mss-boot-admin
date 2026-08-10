//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package blueprint

import "os"

func syncCommittedEntry(_ *os.Root, _ string) error { return nil }
func syncRemovedEntry(_ *os.Root) error             { return nil }
func syncCreatedDirectory(_ *os.Root) error         { return nil }
