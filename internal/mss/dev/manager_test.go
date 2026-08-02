package dev

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.json")
	want := ServiceState{
		ServiceID: "backend",
		PID:       12345,
		StartedAt: time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC),
		Command:   []string{"go", "run", ".", "server"},
		Directory: "/workspace",
		LogPath:   "/workspace/.mss/logs/backend.log",
		Detached:  true,
		Environment: map[string]string{
			"MSS_DEV_MANAGED": "true",
		},
	}
	if err := writeState(path, want); err != nil {
		t.Fatalf("write state: %v", err)
	}
	got, err := readState(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state mismatch\n got: %#v\nwant: %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestReadStateRejectsInvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"serviceId":"backend","pid":0}`), 0o600); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	if _, err := readState(path); err == nil {
		t.Fatal("expected invalid state to fail")
	}
}

func TestWriteTailReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var output bytes.Buffer
	if err := writeTail(path, 2, &output); err != nil {
		t.Fatalf("tail log: %v", err)
	}
	if output.String() != "three\nfour\n" {
		t.Fatalf("tail output = %q", output.String())
	}
}

func TestMergeEnvironmentIsStableAndOverridesValues(t *testing.T) {
	result := mergeEnvironment(
		[]string{"PATH=/usr/bin", "HOME=/tmp/home", "DUPLICATE=old"},
		map[string]string{"DUPLICATE": "new", "MSS_DEV_MANAGED": "true"},
	)
	joined := strings.Join(result, "\n")
	for _, expected := range []string{
		"DUPLICATE=new",
		"HOME=/tmp/home",
		"MSS_DEV_MANAGED=true",
		"PATH=/usr/bin",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in environment %#v", expected, result)
		}
	}
	for index := 1; index < len(result); index++ {
		if result[index-1] > result[index] {
			t.Fatalf("environment is not sorted: %#v", result)
		}
	}
}
