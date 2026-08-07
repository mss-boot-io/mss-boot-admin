package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
)

func AuditLogMiddleware(skipPaths ...string) gin.HandlerFunc {
	skipMap := make(map[string]bool)
	for _, path := range skipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()

		var requestBody string
		if c.Request.Body != nil && c.Request.Method != http.MethodGet {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			if len(bodyBytes) > 0 && len(bodyBytes) < 2000 {
				requestBody = sanitizeAuditRequest(bodyBytes)
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

		auditStatus := enum.Status("enabled")
		if status >= 400 {
			auditStatus = enum.Status("disabled")
		}

		action := c.Request.Method + " " + path
		resource := path
		if parts := strings.Split(path, "/"); len(parts) > 3 {
			resource = strings.Join(parts[:4], "/")
		}

		db := center.Default.GetDB(c, nil)
		err := service.Audit.Log(db, &models.AuditLog{
			Type:      models.AuditLogType(logType),
			UserID:    verify.GetUserID(),
			Username:  verify.GetUsername(),
			Action:    action,
			Resource:  resource,
			Method:    c.Request.Method,
			Path:      path,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Status:    auditStatus,
			Message:   "",
			Request:   requestBody,
			Duration:  duration,
			CreatedAt: time.Now(),
		})
		if err != nil {
			response.Make(c).AddError(err).Log.Error("write audit log failed")
		}
	}
}

const auditRedactedValue = "[REDACTED]"

// sanitizeAuditRequest keeps useful structured audit metadata without copying
// credentials into the audit table. Unknown or malformed body formats fail
// closed: retaining no body is safer than persisting an opaque secret.
func sanitizeAuditRequest(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return ""
	}
	redactAuditValue(value)
	result, err := json.Marshal(value)
	if err != nil || len(result) >= 2000 {
		return ""
	}
	return string(result)
}

func redactAuditValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSensitiveAuditKey(key) {
				typed[key] = auditRedactedValue
				continue
			}
			redactAuditValue(child)
		}
	case []any:
		for _, child := range typed {
			redactAuditValue(child)
		}
	}
}

func isSensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(key)
	normalized = strings.NewReplacer("_", "", "-", "", ".", "").Replace(normalized)
	if normalized == "code" || normalized == "state" || normalized == "captcha" {
		return true
	}
	for _, marker := range []string{
		"password",
		"token",
		"secret",
		"credential",
		"authorization",
		"apikey",
		"privatekey",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
