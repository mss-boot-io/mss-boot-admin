package apis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeLogReaderUsesLiteralSearchAndRedactsResults(t *testing.T) {
	logDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(logDir, "app.log"),
		[]byte("2026/08/16 10:00:00 [INFO] password=hunter2 literal=a.*b\n"+
			"2026/08/16 10:00:01 [ERROR] literal=axb token=secret\n"),
		0o600,
	))

	req := &LogListRequest{Keyword: "a.*b", Page: 1, PageSize: 20}
	require.NoError(t, validateRuntimeLogRequest(req))
	entries, truncated, err := (&Log{}).readLogs(logDir, req)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, entries, 1, "keyword must be treated as literal text, not a regular expression")
	require.NotContains(t, entries[0].Raw, "hunter2")
	require.Contains(t, entries[0].Raw, "password=[REDACTED]")
}

func TestRuntimeLogFilesExcludePathsAndSymlinks(t *testing.T) {
	logDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "accepted.log"), []byte("safe\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "ignored.txt"), []byte("safe\n"), 0o600))
	symlinkPath := filepath.Join(logDir, "linked.log")
	if err := os.Symlink(filepath.Join(logDir, "accepted.log"), symlinkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	files, truncated, err := collectRuntimeLogFiles(logDir)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, files, 1)
	require.Equal(t, "accepted.log", files[0].name)
	require.False(t, strings.Contains(files[0].name, string(filepath.Separator)))
}

func TestRuntimeLogRequestRejectsUnboundedFilters(t *testing.T) {
	tests := []*LogListRequest{
		{Level: "verbose"},
		{Keyword: strings.Repeat("x", runtimeLogMaxKeywordRunes+1)},
		{Page: runtimeLogMaxPage + 1},
		{PageSize: runtimeLogMaxPageSize + 1},
		{StartTime: "2026-01-01T00:00:00Z"},
		{StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-03-01T00:00:00Z"},
	}
	for _, request := range tests {
		require.Error(t, validateRuntimeLogRequest(request))
	}
}
