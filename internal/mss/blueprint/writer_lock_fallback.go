//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package blueprint

import (
	"context"
	"sync"
)

var snapshotWriterRegistry = struct {
	sync.Mutex
	locks map[string]chan struct{}
}{locks: map[string]chan struct{}{}}

func acquireSnapshotWriter(ctx context.Context, root *managedRoot) (func(), error) {
	snapshotWriterRegistry.Lock()
	lock := snapshotWriterRegistry.locks[root.path]
	if lock == nil {
		lock = make(chan struct{}, 1)
		lock <- struct{}{}
		snapshotWriterRegistry.locks[root.path] = lock
	}
	snapshotWriterRegistry.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lock:
		return func() { lock <- struct{}{} }, nil
	}
}

func acquireSnapshotReader(root *managedRoot) (func(), error) {
	snapshotWriterRegistry.Lock()
	lock := snapshotWriterRegistry.locks[root.path]
	if lock == nil {
		lock = make(chan struct{}, 1)
		lock <- struct{}{}
		snapshotWriterRegistry.locks[root.path] = lock
	}
	snapshotWriterRegistry.Unlock()
	<-lock
	return func() { lock <- struct{}{} }, nil
}
