//go:build linux

package dev

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestLinuxProcessNotRunningErrorAcceptsProcfsExitRaces(t *testing.T) {
	for _, err := range []error{
		&os.PathError{Op: "read", Path: "/proc/42/stat", Err: syscall.ENOENT},
		&os.PathError{Op: "read", Path: "/proc/42/stat", Err: syscall.ESRCH},
	} {
		if !linuxProcessNotRunningError(err) {
			t.Fatalf("procfs exit race %v was not treated as a stopped process", err)
		}
	}
	if linuxProcessNotRunningError(errors.New("permission denied")) {
		t.Fatal("unverifiable process identity was treated as a stopped process")
	}
}

func TestLinuxProcessStartTokenIncludesBootAndIgnoresCommand(t *testing.T) {
	fields := make([]string, 20)
	for index := range fields {
		fields[index] = "0"
	}
	fields[0] = "S"
	fields[19] = "987654"
	const bootID = "11111111-2222-4333-8444-555555555555"
	before, err := linuxProcessStartToken(bootID, []byte("42 (old command) "+strings.Join(fields, " ")))
	if err != nil {
		t.Fatalf("construct Linux process identity: %v", err)
	}
	after, err := linuxProcessStartToken(bootID, []byte("42 (new ) command) "+strings.Join(fields, " ")))
	if err != nil {
		t.Fatalf("construct Linux process identity after command change: %v", err)
	}
	if before != after {
		t.Fatalf("mutable Linux command changed process identity: before=%q after=%q", before, after)
	}
	want := "linux-proc:" + bootID + ":987654"
	if before != want {
		t.Fatalf("Linux process identity = %q, want %q", before, want)
	}
}

func TestLinuxProcessStartTokenRejectsMissingBootIdentity(t *testing.T) {
	if _, err := linuxProcessStartToken("", []byte("42 (command) S")); err == nil {
		t.Fatal("missing kernel boot identity unexpectedly succeeded")
	}
}
