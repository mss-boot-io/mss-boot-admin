//go:build linux

package dev

import (
	"strings"
	"testing"
)

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
