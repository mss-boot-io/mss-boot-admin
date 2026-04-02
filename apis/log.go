package apis

import (
	"bufio"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * Phase 4 - 运行时日志查看
 * 用于运维排障，支持日志过滤、搜索、导出
 */

func init() {
	e := &Log{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type Log struct {
	*controller.Simple
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
}

// LogListRequest 日志列表请求
type LogListRequest struct {
	Level     string `form:"level"`     // info, warn, error, debug
	Keyword   string `form:"keyword"`   // 关键词搜索
	StartTime string `form:"startTime"` // 开始时间
	EndTime   string `form:"endTime"`   // 结束时间
	Page      int    `form:"page"`      // 页码
	PageSize  int    `form:"pageSize"`  // 每页条数
}

// LogListResponse 日志列表响应
type LogListResponse struct {
	Total int         `json:"total"`
	List  []LogEntry `json:"list"`
}

func (e *Log) Other(r *gin.RouterGroup) {
	r.GET("/logs", e.List)
	r.GET("/logs/files", e.Files)
	r.GET("/logs/export", e.Export)
}

// List 日志列表
// @Summary 日志列表
// @Description 查询运行时日志
// @Tags log
// @Param level query string false "日志级别"
// @Param keyword query string false "关键词"
// @Param startTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Param page query int false "页码"
// @Param pageSize query int false "每页条数"
// @Success 200 {object} LogListResponse
// @Router /admin/api/logs [get]
// @Security Bearer
func (e *Log) List(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &LogListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		api.Err(http.StatusBadRequest)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	// 日志目录
	logDir := "logs"
	entries, err := e.readLogs(logDir, req)
	if err != nil {
		api.AddError(err).Log.Error("read logs error")
		api.Err(http.StatusInternalServerError)
		return
	}

	// 分页
	total := len(entries)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	api.OK(&LogListResponse{
		Total: total,
		List:  entries[start:end],
	})
}

// Files 日志文件列表
// @Summary 日志文件列表
// @Description 获取可用的日志文件列表
// @Tags log
// @Success 200 {array} string
// @Router /admin/api/logs/files [get]
// @Security Bearer
func (e *Log) Files(ctx *gin.Context) {
	api := response.Make(ctx)

	logDir := "logs"
	files := []string{}

	err := filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".log") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		api.AddError(err).Log.Error("walk logs dir error")
		api.Err(http.StatusInternalServerError)
		return
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	api.OK(files)
}

// Export 导出日志
// @Summary 导出日志
// @Description 导出日志文件
// @Tags log
// @Param level query string false "日志级别"
// @Param keyword query string false "关键词"
// @Param startTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Success 200 {file} file
// @Router /admin/api/logs/export [get]
// @Security Bearer
func (e *Log) Export(ctx *gin.Context) {
	req := &LogListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	logDir := "logs"
	entries, err := e.readLogs(logDir, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 生成文件内容
	var buf strings.Builder
	for _, entry := range entries {
		buf.WriteString(entry.Raw)
		buf.WriteString("\n")
	}

	// 设置响应头
	filename := fmt.Sprintf("logs_%s.txt", time.Now().Format("20060102_150405"))
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	ctx.Header("Content-Type", "text/plain; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// readLogs 读取日志文件
func (e *Log) readLogs(logDir string, req *LogListRequest) ([]LogEntry, error) {
	entries := []LogEntry{}

	// 检查日志目录是否存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return entries, nil
	}

	// 遍历日志文件
	err := filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".log") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// 增加缓冲区大小以支持长行
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			entry := e.parseLogLine(line)

			// 过滤级别
			if req.Level != "" && !strings.EqualFold(entry.Level, req.Level) {
				continue
			}

			// 过滤关键词
			if req.Keyword != "" {
				matched, err := regexp.MatchString(req.Keyword, line)
				if err != nil || !matched {
					continue
				}
			}

			// 过滤时间范围
			if req.StartTime != "" || req.EndTime != "" {
				if entry.Timestamp != "" {
					ts := entry.Timestamp
					if req.StartTime != "" && ts < req.StartTime {
						continue
					}
					if req.EndTime != "" && ts > req.EndTime {
						continue
					}
				}
			}

			entries = append(entries, entry)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 按时间倒序排序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	return entries, nil
}

// parseLogLine 解析日志行
func (e *Log) parseLogLine(line string) LogEntry {
	entry := LogEntry{
		Raw: line,
	}

	// 尝试解析 slog 格式
	// 格式: time=2024-01-01T12:00:00 level=info msg=message
	if strings.Contains(line, "time=") && strings.Contains(line, "level=") {
		parts := strings.Fields(line)
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				switch kv[0] {
				case "time":
					entry.Timestamp = strings.Trim(kv[1], `"`)
				case "level":
					entry.Level = strings.Trim(kv[1], `"`)
				case "msg":
					entry.Message = strings.Trim(kv[1], `"`)
				}
			}
		}
	} else {
		// 尝试解析标准日志格式
		// 格式: 2024/01/01 12:00:00 [INFO] message
		re := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(\w+)\]\s+(.+)$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			entry.Timestamp = matches[1]
			entry.Level = strings.ToLower(matches[2])
			entry.Message = matches[3]
		} else {
			// 无法解析，作为普通文本
			entry.Message = line
		}
	}

	return entry
}