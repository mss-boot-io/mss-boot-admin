package app

import (
	"bytes"
	"os"
	"testing"
)

func TestTerminalSecretPromptRequiresTerminalFile(t *testing.T) {
	if prompt := terminalSecretPrompt(bytes.NewReader(nil), &bytes.Buffer{}); prompt != nil {
		t.Fatal("non-file input unexpectedly enabled a password prompt")
	}

	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if prompt := terminalSecretPromptWith(file, &bytes.Buffer{}, func(int) bool { return false }, func(int) ([]byte, error) {
		return []byte("should-not-run"), nil
	}); prompt != nil {
		t.Fatal("non-terminal file unexpectedly enabled a password prompt")
	}
}

func TestTerminalSecretPromptUsesHiddenReaderAndSeparateOutput(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var output bytes.Buffer
	readCount := 0
	prompt := terminalSecretPromptWith(file, &output, func(fd int) bool {
		return fd == int(file.Fd())
	}, func(fd int) ([]byte, error) {
		readCount++
		if fd != int(file.Fd()) {
			t.Fatalf("password fd = %d, want %d", fd, file.Fd())
		}
		return []byte("Hidden!2026"), nil
	})
	if prompt == nil {
		t.Fatal("terminal input did not enable a password prompt")
	}
	secret, err := prompt()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(secret), "Hidden!2026"; got != want {
		t.Fatalf("password = %q, want %q", got, want)
	}
	if readCount != 1 {
		t.Fatalf("hidden reader count = %d, want 1", readCount)
	}
	if got, want := output.String(), "Initial local administrator password: \n"; got != want {
		t.Fatalf("prompt output = %q, want %q", got, want)
	}
	if bytes.Contains(output.Bytes(), secret) {
		t.Fatal("prompt output exposed the password")
	}
}
