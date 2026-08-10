package storage_test

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	storagequeue "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/queue"
)

var (
	_ storage.AdapterQueue        = (*managedQueueStub)(nil)
	_ storage.ManagedAdapterQueue = (*managedQueueStub)(nil)
	_ storage.ManagedAdapterQueue = (*storagequeue.Kafka)(nil)
)

type managedQueueStub struct{}

func (*managedQueueStub) String() string                 { return "stub" }
func (*managedQueueStub) Append(...storage.Option) error { return nil }
func (*managedQueueStub) Register(...storage.Option)     {}
func (*managedQueueStub) Run(context.Context)            {}
func (*managedQueueStub) Shutdown()                      {}
func (*managedQueueStub) RegisterContext(context.Context, ...storage.Option) error {
	return nil
}
func (*managedQueueStub) Start(context.Context) error { return nil }
func (*managedQueueStub) Errors() <-chan error        { return nil }
func (*managedQueueStub) Close(context.Context) error { return nil }

func TestManagedAdapterQueueRemainsAnAdapterQueue(t *testing.T) {
	var managed storage.ManagedAdapterQueue = &managedQueueStub{}
	var legacy storage.AdapterQueue = managed
	if legacy.String() != "stub" {
		t.Fatalf("legacy String() = %q", legacy.String())
	}
}
