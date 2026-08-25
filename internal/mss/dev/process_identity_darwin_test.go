//go:build darwin

package dev

import (
	"errors"
	"os/exec"
	"testing"
)

func TestDarwinMissingProcessHasStoppedIdentity(t *testing.T) {
	command := exec.Command("sleep", "10")
	if err := command.Start(); err != nil {
		t.Fatalf("start Darwin process identity fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	if _, err := processStartToken(command.Process.Pid); err != nil {
		t.Fatalf("read live Darwin process identity: %v", err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatalf("kill Darwin process identity fixture: %v", err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("killed Darwin process identity fixture exited successfully")
	}
	if _, err := processStartToken(command.Process.Pid); !errors.Is(err, errProcessNotRunning) {
		t.Fatalf("missing Darwin PID identity error = %v, want process not running", err)
	}
}
