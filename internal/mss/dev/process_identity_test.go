package dev

import "testing"

func TestProcessStartTokenUsesOnlyStableCreationTime(t *testing.T) {
	creation := processCreationSnapshot{
		Platform:    "darwin-sysctl",
		Seconds:     1_787_600_000,
		Nanoseconds: 123_456_000,
		Command:     "go run ./cmd/server server",
	}
	before, err := processStartTokenFromSnapshot(creation)
	if err != nil {
		t.Fatalf("construct process start token: %v", err)
	}
	creation.Command = "server: changed process title"
	after, err := processStartTokenFromSnapshot(creation)
	if err != nil {
		t.Fatalf("construct process start token after command change: %v", err)
	}
	if before != after {
		t.Fatalf("mutable command changed process identity: before=%q after=%q", before, after)
	}
	if want := "darwin-sysctl-created:1787600000.123456000"; before != want {
		t.Fatalf("process start token = %q, want %q", before, want)
	}
}

func TestProcessStartTokenRejectsInvalidCreationTime(t *testing.T) {
	for name, snapshot := range map[string]processCreationSnapshot{
		"missing platform": {Seconds: 1},
		"missing time":     {Platform: "darwin-sysctl"},
		"negative nanos":   {Platform: "darwin-sysctl", Seconds: 1, Nanoseconds: -1},
		"overflow nanos":   {Platform: "darwin-sysctl", Seconds: 1, Nanoseconds: 1_000_000_000},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := processStartTokenFromSnapshot(snapshot); err == nil {
				t.Fatal("invalid process creation snapshot unexpectedly succeeded")
			}
		})
	}
}
