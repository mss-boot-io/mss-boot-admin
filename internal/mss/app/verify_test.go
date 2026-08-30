package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerifyInvalidReleaseEvidenceDoesNotReferenceAStaleReport(t *testing.T) {
	rootOverride := repositoryRoot(t)
	command := newVerifyCommand(&rootOverride)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{
		"--all",
		"--release-evidence",
		"--expect-commit",
		"not-a-full-commit",
	})

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "full 40-character lowercase") {
		t.Fatalf("verify error = %v, want invalid commit rejection", err)
	}
	if strings.Contains(output.String(), "report:") {
		t.Fatalf("invalid evidence referenced a report that was not written: %q", output.String())
	}
}
