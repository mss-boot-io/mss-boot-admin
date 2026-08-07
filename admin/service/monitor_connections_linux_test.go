//go:build linux

package service

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	gopsutilcommon "github.com/shirou/gopsutil/v3/common"
)

func TestLinuxMonitorConnectionCountReadsContextHostProcTables(t *testing.T) {
	procRoot := t.TempDir()
	netRoot := filepath.Join(procRoot, "net")
	if err := os.MkdirAll(netRoot, 0o755); err != nil {
		t.Fatalf("create proc net fixture: %v", err)
	}
	fixtures := map[string]string{
		"tcp": `  sl  local_address rem_address   st tx_queue
   0: 0100007F:1F90 00000000:0000 01 00000000:00000000
   1: 00000000:1F91 00000000:0000 0A 00000000:00000000
   2: 0100007F:1F92 0100007F:1F93 06 00000000:00000000
   3: 0100007F:1F94 0100007F:1F95 08 00000000:00000000
`,
		"tcp6": `  sl  local_address rem_address   st tx_queue
   0: 00000000000000000000000000000000:1F90 00000000000000000000000000000000:0000 01 00000000:00000000
`,
		"udp": `  sl  local_address rem_address   st tx_queue
   0: 00000000:0035 00000000:0000 07 00000000:00000000
`,
		"udp6": `  sl  local_address rem_address   st tx_queue
   0: 00000000000000000000000000000000:0035 00000000000000000000000000000000:0000 07 00000000:00000000
`,
		"unix": `Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000002 00000000 00010000 0001 01 11111 /run/one.sock
0000000000000001: 00000002 00000000 00010000 0001 01 22222
`,
	}
	for name, content := range fixtures {
		if err := os.WriteFile(filepath.Join(netRoot, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s fixture: %v", name, err)
		}
	}

	t.Setenv("HOST_PROC", filepath.Join(t.TempDir(), "wrong-environment-root"))
	ctx := context.WithValue(context.Background(), gopsutilcommon.EnvKey, gopsutilcommon.EnvMap{
		gopsutilcommon.HostProcEnvKey: procRoot,
	})
	got, err := sampleMonitorConnectionCount(ctx)
	if err != nil {
		t.Fatalf("sample Linux connection count: %v", err)
	}
	want := &dto.MonitorConnectionCount{
		Established: 2,
		Listen:      1,
		TimeWait:    1,
		CloseWait:   1,
		Total:       9,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("connection count = %#v, want %#v", got, want)
	}
}

func TestLinuxMonitorConnectionTableStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelAfterRead{
		content: []byte("  sl  local_address rem_address st tx_queue\n"),
		cancel:  cancel,
	}
	_, err := countLinuxConnectionTable(ctx, reader, linuxConnectionTables[0])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled connection table read error = %v, want %v", err, context.Canceled)
	}
}

type cancelAfterRead struct {
	content []byte
	cancel  context.CancelFunc
	read    bool
}

func (e *cancelAfterRead) Read(target []byte) (int, error) {
	if e.read {
		return 0, io.EOF
	}
	e.read = true
	n := copy(target, e.content)
	e.cancel()
	return n, nil
}
