package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"golang.org/x/text/unicode/norm"
)

const themeAuditMetadataContextKey = "mss.theme.audit.metadata"

const (
	applicationThemeMutationPath = "/admin/api/app-configs/theme"
	userThemeMutationPath        = "/admin/api/user-configs/theme"
)

// ThemeAuditMetadata is deliberately value-free. It records enough structure
// to investigate a theme mutation without duplicating configuration values in
// the audit log.
type ThemeAuditMetadata struct {
	Scope       string   `json:"scope"`
	ChangedKeys []string `json:"changedKeys"`
	Outcome     string   `json:"outcome"`
	Revision    string   `json:"revision,omitempty"`
}

func SetThemeAuditMetadata(ctx *gin.Context, metadata ThemeAuditMetadata) {
	metadata.ChangedKeys = append([]string(nil), metadata.ChangedKeys...)
	ctx.Set(themeAuditMetadataContextKey, metadata)
}

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
			if matchesAuditSkipPath(path, skipPath) {
				return
			}
		}

		if strings.Contains(path, "login") || strings.Contains(path, "logout") {
			return
		}

		duration := time.Since(start).Milliseconds()
		status := c.Writer.Status()

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

		metadata, hasThemeMetadata := themeAuditMetadataFromContext(c)
		if !hasThemeMetadata {
			metadata, hasThemeMetadata = fallbackThemeAuditMetadata(
				c.Request.Method,
				path,
				status,
				requestBody,
			)
		}

		message := ""
		if hasThemeMetadata {
			if encoded, err := json.Marshal(metadata); err == nil {
				message = string(encoded)
				// Theme values must never be persisted, including when an auth
				// middleware rejects the request before the controller runs.
				requestBody = ""
			}
		}

		verify := GetVerify(c)
		if verify == nil {
			return
		}
		userID := verify.GetUserID()
		username := verify.GetUsername()

		db := center.Default.GetDB(c, nil)
		err := service.Audit.Log(db, &models.AuditLog{
			Type:      models.AuditLogType(logType),
			UserID:    userID,
			Username:  username,
			Action:    action,
			Resource:  resource,
			Method:    c.Request.Method,
			Path:      path,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Status:    auditStatus,
			Message:   message,
			Request:   requestBody,
			Duration:  duration,
			CreatedAt: time.Now(),
		})
		if err != nil {
			response.Make(c).AddError(err).Log.Error("write audit log failed")
		}
	}
}

func themeAuditMetadataFromContext(ctx *gin.Context) (ThemeAuditMetadata, bool) {
	value, ok := ctx.Get(themeAuditMetadataContextKey)
	if !ok {
		return ThemeAuditMetadata{}, false
	}
	metadata, ok := value.(ThemeAuditMetadata)
	return metadata, ok
}

func fallbackThemeAuditMetadata(
	method string,
	path string,
	status int,
	requestBody string,
) (ThemeAuditMetadata, bool) {
	if method != http.MethodPut && method != http.MethodDelete {
		return ThemeAuditMetadata{}, false
	}

	scope, isThemeMutation := themeAuditScopeFromPath(path)
	if !isThemeMutation {
		return ThemeAuditMetadata{}, false
	}

	changedKeys := make([]string, 0)
	if method == http.MethodDelete {
		changedKeys = service.ThemeFieldNames()
	} else if requestBody != "" {
		var payload struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if json.Unmarshal([]byte(requestBody), &payload) == nil {
			for key := range payload.Data {
				changedKeys = append(changedKeys, key)
			}
		}
	}
	sort.Strings(changedKeys)

	return ThemeAuditMetadata{
		Scope:       scope,
		ChangedKeys: changedKeys,
		Outcome:     themeAuditOutcome(status),
	}, true
}

func themeAuditScopeFromPath(path string) (string, bool) {
	// URL.Path is normally already decoded, but this helper also accepts an
	// escaped path so auth failures are value-free regardless of where the
	// fallback is invoked from.
	if strings.Contains(path, "%") {
		decoded, err := url.PathUnescape(path)
		if err != nil {
			return "", false
		}
		path = decoded
	}

	for _, candidate := range []struct {
		prefix string
		scope  string
	}{
		{prefix: strings.TrimSuffix(applicationThemeMutationPath, service.ThemeConfigGroup), scope: service.ThemeScopeApplication},
		{prefix: strings.TrimSuffix(userThemeMutationPath, service.ThemeConfigGroup), scope: service.ThemeScopeUser},
	} {
		if !strings.HasPrefix(path, candidate.prefix) {
			continue
		}
		group := strings.TrimPrefix(path, candidate.prefix)
		if group == "" || strings.Contains(group, "/") {
			return "", false
		}
		if canonicalThemeAuditIdentifierFold(group) == service.ThemeConfigGroup {
			return candidate.scope, true
		}
		return "", false
	}
	return "", false
}

func canonicalThemeAuditIdentifierFold(value string) string {
	value = strings.TrimRightFunc(value, unicode.IsSpace)
	decomposed := norm.NFKD.String(value)
	builder := strings.Builder{}
	builder.Grow(len(decomposed))
	for _, current := range decomposed {
		if unicode.Is(unicode.Mn, current) {
			continue
		}
		builder.WriteRune(unicode.ToLower(current))
	}
	return builder.String()
}

func themeAuditOutcome(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "authentication_failed"
	case http.StatusForbidden:
		return "authorization_failed"
	case http.StatusPreconditionFailed:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusBadRequest:
		return "bad_request"
	}
	if status >= http.StatusOK && status < http.StatusBadRequest {
		return "success"
	}
	if status >= http.StatusInternalServerError {
		return "server_error"
	}
	return "rejected"
}

func matchesAuditSkipPath(path, skipPath string) bool {
	if path == skipPath {
		return true
	}
	return len(path) > len(skipPath) &&
		strings.HasPrefix(path, skipPath) &&
		path[len(skipPath)] == '/'
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
