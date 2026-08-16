package apis

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
)

const (
	runtimeLogMaxPage          = 10_000
	runtimeLogMaxPageSize      = 100
	runtimeLogMaxFiles         = 32
	runtimeLogMaxFileBytes     = int64(16 << 20)
	runtimeLogMaxScanBytes     = int64(64 << 20)
	runtimeLogMaxLines         = 100_000
	runtimeLogMaxMatches       = 10_000
	runtimeLogMaxScannerBytes  = 1 << 20
	runtimeLogMaxEntryBytes    = 16 << 10
	runtimeLogMaxExportBytes   = 5 << 20
	runtimeLogMaxKeywordRunes  = 128
	runtimeLogMaxRangeDuration = 31 * 24 * time.Hour
)

var standardRuntimeLogPattern = regexp.MustCompile(
	`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(\w+)]\s+(.+)$`,
)

func init() {
	response.AppendController(&Log{Simple: controller.NewSimple()})
}

type Log struct {
	*controller.Simple
}

// LogEntry is deliberately bounded and redacted before it reaches a browser.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
}

type LogListRequest struct {
	Level     string `form:"level"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`

	start *time.Time
	end   *time.Time
}

type LogListResponse struct {
	Total     int        `json:"total"`
	List      []LogEntry `json:"list"`
	Truncated bool       `json:"truncated,omitempty"`
}

type runtimeLogFile struct {
	path    string
	name    string
	size    int64
	modTime time.Time
}

func (e *Log) Other(r *gin.RouterGroup) {
	r.GET("/logs", response.AuthHandler, protectOperationalResponse, e.List)
	r.GET("/logs/files", response.AuthHandler, protectOperationalResponse, e.Files)
	r.GET("/logs/export", response.AuthHandler, protectOperationalResponse, e.Export)
}

// List returns a bounded, redacted projection of runtime log files.
func (e *Log) List(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &LogListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		api.Err(http.StatusUnprocessableEntity, "invalid log query")
		return
	}
	if err := validateRuntimeLogRequest(req); err != nil {
		api.Err(http.StatusUnprocessableEntity, "invalid log query")
		return
	}

	entries, truncated, err := e.readLogs("logs", req)
	if err != nil {
		api.AddError(err).Log.Error("read runtime logs")
		api.Err(http.StatusInternalServerError, "runtime logs are unavailable")
		return
	}

	total := len(entries)
	start := (req.Page - 1) * req.PageSize
	if start > total {
		start = total
	}
	end := start + req.PageSize
	if end > total {
		end = total
	}
	api.OK(&LogListResponse{
		Total:     total,
		List:      entries[start:end],
		Truncated: truncated,
	})
}

// Files exposes display names only. It never returns filesystem paths.
func (e *Log) Files(ctx *gin.Context) {
	api := response.Make(ctx)
	files, truncated, err := collectRuntimeLogFiles("logs")
	if err != nil {
		api.AddError(err).Log.Error("list runtime log files")
		api.Err(http.StatusInternalServerError, "runtime logs are unavailable")
		return
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.name)
	}
	api.OK(gin.H{"files": names, "truncated": truncated})
}

// Export produces a redacted text file and refuses partial or oversized data.
func (e *Log) Export(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &LogListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		api.Err(http.StatusUnprocessableEntity, "invalid log query")
		return
	}
	if err := validateRuntimeLogRequest(req); err != nil {
		api.Err(http.StatusUnprocessableEntity, "invalid log query")
		return
	}

	entries, truncated, err := e.readLogs("logs", req)
	if err != nil {
		api.AddError(err).Log.Error("export runtime logs")
		api.Err(http.StatusInternalServerError, "runtime logs are unavailable")
		return
	}
	if truncated {
		api.Err(http.StatusRequestEntityTooLarge, "log export exceeds the safe scan limit")
		return
	}

	var content strings.Builder
	for _, entry := range entries {
		if content.Len()+len(entry.Raw)+1 > runtimeLogMaxExportBytes {
			api.Err(http.StatusRequestEntityTooLarge, "log export exceeds the safe size limit")
			return
		}
		content.WriteString(entry.Raw)
		content.WriteByte('\n')
	}
	filename := fmt.Sprintf("logs_%s.txt", time.Now().UTC().Format("20060102_150405"))
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	ctx.Header("Content-Type", "text/plain; charset=utf-8")
	ctx.String(http.StatusOK, content.String())
}

func validateRuntimeLogRequest(req *LogListRequest) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}
	req.Level = strings.ToLower(strings.TrimSpace(req.Level))
	switch req.Level {
	case "", "trace", "debug", "info", "warn", "error", "fatal":
	default:
		return fmt.Errorf("invalid level")
	}
	if utf8.RuneCountInString(req.Keyword) > runtimeLogMaxKeywordRunes ||
		strings.ContainsRune(req.Keyword, '\x00') {
		return fmt.Errorf("invalid keyword")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = runtimeLogMaxPageSize
	}
	if req.Page > runtimeLogMaxPage || req.PageSize > runtimeLogMaxPageSize {
		return fmt.Errorf("invalid pagination")
	}
	if (req.StartTime == "") != (req.EndTime == "") {
		return fmt.Errorf("incomplete time range")
	}
	if req.StartTime == "" {
		return nil
	}
	start, err := parseRuntimeLogTime(req.StartTime)
	if err != nil {
		return err
	}
	end, err := parseRuntimeLogTime(req.EndTime)
	if err != nil || end.Before(start) || end.Sub(start) > runtimeLogMaxRangeDuration {
		return fmt.Errorf("invalid time range")
	}
	req.start = &start
	req.end = &end
	return nil
}

func collectRuntimeLogFiles(logDir string) ([]runtimeLogFile, bool, error) {
	directory, err := os.Open(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []runtimeLogFile{}, false, nil
		}
		return nil, false, err
	}
	defer directory.Close()

	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, false, err
	}
	files := make([]runtimeLogFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 ||
			!strings.EqualFold(filepath.Ext(entry.Name()), ".log") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, runtimeLogFile{
			path:    filepath.Join(logDir, entry.Name()),
			name:    entry.Name(),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].name > files[j].name
		}
		return files[i].modTime.After(files[j].modTime)
	})
	truncated := len(files) > runtimeLogMaxFiles
	if truncated {
		files = files[:runtimeLogMaxFiles]
	}
	return files, truncated, nil
}

func (e *Log) readLogs(logDir string, req *LogListRequest) ([]LogEntry, bool, error) {
	files, truncated, err := collectRuntimeLogFiles(logDir)
	if err != nil {
		return nil, false, err
	}
	entries := make([]LogEntry, 0, runtimeLogMaxPageSize)
	remainingBytes := runtimeLogMaxScanBytes
	lineCount := 0

	for _, candidate := range files {
		if remainingBytes <= 0 || lineCount >= runtimeLogMaxLines || len(entries) >= runtimeLogMaxMatches {
			truncated = true
			break
		}
		start := int64(0)
		if candidate.size > runtimeLogMaxFileBytes {
			start = candidate.size - runtimeLogMaxFileBytes
			truncated = true
		}
		bytesToRead := candidate.size - start
		if bytesToRead > remainingBytes {
			start = candidate.size - remainingBytes
			bytesToRead = remainingBytes
			truncated = true
		}
		remainingBytes -= bytesToRead

		file, openErr := os.Open(candidate.path)
		if openErr != nil {
			truncated = true
			continue
		}
		if start > 0 {
			if _, seekErr := file.Seek(start, io.SeekStart); seekErr != nil {
				file.Close()
				truncated = true
				continue
			}
		}
		// Read only the size observed during inventory. An actively written log
		// can grow after stat; scanning the raw file to EOF would otherwise
		// bypass the per-file and aggregate byte budgets.
		scanner := bufio.NewScanner(io.LimitReader(file, bytesToRead))
		scanner.Buffer(make([]byte, 64*1024), runtimeLogMaxScannerBytes)
		if start > 0 {
			// The tail may begin in the middle of a line; never return a fragment.
			scanner.Scan()
		}
		for scanner.Scan() {
			lineCount++
			if lineCount > runtimeLogMaxLines {
				truncated = true
				break
			}
			line := service.RedactOperationalText(scanner.Text(), runtimeLogMaxEntryBytes)
			entry := e.parseLogLine(line)
			if !runtimeLogMatches(entry, line, req) {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= runtimeLogMaxMatches {
				truncated = true
				break
			}
		}
		if scanner.Err() != nil {
			truncated = true
		}
		file.Close()
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left, leftErr := parseRuntimeLogTime(entries[i].Timestamp)
		right, rightErr := parseRuntimeLogTime(entries[j].Timestamp)
		if leftErr == nil && rightErr == nil {
			return left.After(right)
		}
		return entries[i].Timestamp > entries[j].Timestamp
	})
	return entries, truncated, nil
}

func runtimeLogMatches(entry LogEntry, line string, req *LogListRequest) bool {
	if req.Level != "" && !strings.EqualFold(entry.Level, req.Level) {
		return false
	}
	if req.Keyword != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(req.Keyword)) {
		return false
	}
	if req.start == nil {
		return true
	}
	timestamp, err := parseRuntimeLogTime(entry.Timestamp)
	if err != nil {
		return false
	}
	return !timestamp.Before(*req.start) && !timestamp.After(*req.end)
}

func parseRuntimeLogTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp")
}

func (e *Log) parseLogLine(line string) LogEntry {
	entry := LogEntry{Raw: line}
	if strings.Contains(line, "time=") && strings.Contains(line, "level=") {
		parts := strings.Fields(line)
		for _, part := range parts {
			keyValue := strings.SplitN(part, "=", 2)
			if len(keyValue) != 2 {
				continue
			}
			switch keyValue[0] {
			case "time":
				entry.Timestamp = strings.Trim(keyValue[1], `"`)
			case "level":
				entry.Level = strings.ToLower(strings.Trim(keyValue[1], `"`))
			case "msg":
				entry.Message = strings.Trim(keyValue[1], `"`)
			}
		}
	}
	if entry.Message == "" {
		matches := standardRuntimeLogPattern.FindStringSubmatch(line)
		if len(matches) == 4 {
			entry.Timestamp = matches[1]
			entry.Level = strings.ToLower(matches[2])
			entry.Message = matches[3]
		} else {
			entry.Message = line
		}
	}
	entry.Message = service.RedactOperationalText(entry.Message, runtimeLogMaxEntryBytes)
	return entry
}
