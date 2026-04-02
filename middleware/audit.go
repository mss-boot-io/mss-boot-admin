package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func AuditLogMiddleware(skipPaths ...string) gin.HandlerFunc {
	skipMap := make(map[string]bool)
	for _, path := range skipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()

		blw := &responseWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		var requestBody string
		if c.Request.Body != nil && c.Request.Method != http.MethodGet {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			if len(bodyBytes) > 0 && len(bodyBytes) < 2000 {
				requestBody = string(bodyBytes)
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		c.Next()

		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions {
			return
		}

		path := c.Request.URL.Path
		for skipPath := range skipMap {
			if strings.HasPrefix(path, skipPath) {
				return
			}
		}

		if strings.Contains(path, "login") || strings.Contains(path, "logout") {
			return
		}

		duration := time.Since(start).Milliseconds()
		status := c.Writer.Status()

		verify := GetVerify(c)
		if verify == nil {
			return
		}

		var logType string
		switch c.Request.Method {
		case http.MethodPost:
			logType = "create"
		case http.MethodPut, http.MethodPatch:
			logType = "update"
		case http.MethodDelete:
			logType = "delete"
		default:
			return
		}

		auditStatus := "enabled"
		if status >= 400 {
			auditStatus = "disabled"
		}

		action := c.Request.Method + " " + path
		resource := path
		if parts := strings.Split(path, "/"); len(parts) > 3 {
			resource = strings.Join(parts[:4], "/")
		}

		db := center.Default.GetDB(c, nil)
		db.Table("mss_boot_audit_logs").Create(map[string]interface{}{
			"id":         pkg.SimpleID(),
			"type":       logType,
			"user_id":    verify.GetUserID(),
			"username":   verify.GetUsername(),
			"action":     action,
			"resource":   resource,
			"method":     c.Request.Method,
			"path":       path,
			"ip":         c.ClientIP(),
			"user_agent": c.GetHeader("User-Agent"),
			"status":     auditStatus,
			"message":    "",
			"request":    requestBody,
			"duration":   duration,
			"created_at": time.Now(),
		})
	}
}