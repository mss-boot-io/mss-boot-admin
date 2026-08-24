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

const (
	themeAuditMetadataContextKey        = "mss.theme.audit.metadata"
	presentationAuditMetadataContextKey = "mss.presentation.audit.metadata"
)

const (
	applicationThemeMutationPath = "/admin/api/app-configs/theme"
	userThemeMutationPath        = "/admin/api/user-configs/theme"
	auditRequestBodyLimit        = int64(2000)
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

// PresentationAuditMetadata intentionally contains no editable presentation
// values. SubjectFingerprint is a one-way digest and SubjectPresent permits
// application-scope evidence without persisting role or user identifiers.
type PresentationAuditMetadata struct {
	AggregateID        string `json:"aggregateID,omitempty"`
	PageKey            string `json:"pageKey,omitempty"`
	Scope              string `json:"scope,omitempty"`
	SubjectPresent     bool   `json:"subjectPresent"`
	SubjectFingerprint string `json:"subjectFingerprint,omitempty"`
	Transition         string `json:"transition"`
	Outcome            string `json:"outcome"`
	AggregateVersion   int64  `json:"aggregateVersion,omitempty"`
	PublishedRevision  int64  `json:"publishedRevision,omitempty"`
	DefinitionHash     string `json:"definitionHash,omitempty"`
	ContentDigest      string `json:"contentDigest,omitempty"`
}

func SetThemeAuditMetadata(ctx *gin.Context, metadata ThemeAuditMetadata) {
	metadata.ChangedKeys = append([]string(nil), metadata.ChangedKeys...)
	ctx.Set(themeAuditMetadataContextKey, metadata)
}

func SetPresentationAuditMetadata(ctx *gin.Context, metadata PresentationAuditMetadata) {
	if ctx == nil {
		return
	}
	ctx.Set(presentationAuditMetadataContextKey, metadata)
}

func AuditLogMiddleware(skipPaths ...string) gin.HandlerFunc {
	skipMap := make(map[string]bool)
	for _, path := range skipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()

		var requestBody string
		if shouldCaptureAuditRequestBody(c.Request) {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			if len(bodyBytes) > 0 && int64(len(bodyBytes)) < auditRequestBodyLimit {
				requestBody = sanitizeAuditRequest(bodyBytes)
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		c.Next()

		if (c.Request.Method == http.MethodGet && !isAuditedRead(c.Request.URL.Path)) ||
			c.Request.Method == http.MethodOptions {
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
		case http.MethodGet:
			logType = string(models.AuditLogTypeExport)
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

		var metadata any
		hasValueFreeMetadata := false
		if presentationMetadata, ok := presentationAuditMetadataFromContext(c); ok {
			metadata, hasValueFreeMetadata = presentationMetadata, true
		} else if presentationMetadata, ok := fallbackPresentationAuditMetadata(c.Request.Method, path, status); ok {
			metadata, hasValueFreeMetadata = presentationMetadata, true
		} else if themeMetadata, ok := themeAuditMetadataFromContext(c); ok {
			metadata, hasValueFreeMetadata = themeMetadata, true
		} else if themeMetadata, ok := fallbackThemeAuditMetadata(c.Request.Method, path, status, requestBody); ok {
			metadata, hasValueFreeMetadata = themeMetadata, true
		}
		message := ""
		if hasValueFreeMetadata {
			if encoded, err := json.Marshal(metadata); err == nil {
				message = string(encoded)
				// Theme and presentation values must never be persisted, including
				// when authorization rejects before the controller runs.
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

func shouldCaptureAuditRequestBody(request *http.Request) bool {
	if request == nil || request.Body == nil || request.Method == http.MethodGet || request.Method == http.MethodOptions {
		return false
	}
	if isPresentationAuditPath(request.URL.Path) {
		return false
	}
	// Never buffer multipart uploads in the audit middleware. Upload handlers
	// already enforce file size and MIME policy, while audit evidence records
	// actor, route, status, and duration without copying file bytes into memory.
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(request.Header.Get("Content-Type"), ";", 2)[0]))
	if mediaType != "application/json" {
		return false
	}
	// Unknown/chunked or large bodies are intentionally not captured. Reading
	// them merely to decide not to persist them would amplify request memory.
	return request.ContentLength >= 0 && request.ContentLength < auditRequestBodyLimit
}

func presentationAuditMetadataFromContext(ctx *gin.Context) (PresentationAuditMetadata, bool) {
	value, ok := ctx.Get(presentationAuditMetadataContextKey)
	if !ok {
		return PresentationAuditMetadata{}, false
	}
	metadata, ok := value.(PresentationAuditMetadata)
	return metadata, ok
}

func fallbackPresentationAuditMetadata(method, path string, status int) (PresentationAuditMetadata, bool) {
	transition, aggregateID, ok := presentationAuditRoute(method, path)
	if !ok {
		return PresentationAuditMetadata{}, false
	}
	return PresentationAuditMetadata{
		AggregateID: aggregateID,
		Transition:  transition,
		Outcome:     presentationAuditOutcome(status),
	}, true
}

func isPresentationAuditPath(path string) bool {
	return path == "/admin/api/presentation-profiles" ||
		strings.HasPrefix(path, "/admin/api/presentation-profiles/")
}

func presentationAuditRoute(method, path string) (transition, aggregateID string, ok bool) {
	const prefix = "/admin/api/presentation-profiles"
	if path == prefix {
		if method == http.MethodPost {
			return "create-draft", "", true
		}
		return "", "", false
	}
	remainder := strings.TrimPrefix(path, prefix+"/")
	if remainder == path || remainder == "" {
		return "", "", false
	}
	segments := strings.Split(remainder, "/")
	if len(segments) == 1 && segments[0] == "validate" && method == http.MethodPost {
		return "validate", "", true
	}
	if len(segments) != 2 || strings.TrimSpace(segments[0]) == "" {
		return "", "", false
	}
	switch {
	case segments[1] == "draft" && method == http.MethodPut:
		return "replace-draft", segments[0], true
	case segments[1] == "publish" && method == http.MethodPost:
		return "publish", segments[0], true
	case segments[1] == "rollback" && method == http.MethodPost:
		return "rollback", segments[0], true
	default:
		return "", "", false
	}
}

func presentationAuditOutcome(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "authentication_failed"
	case http.StatusForbidden:
		return "authorization_failed"
	case http.StatusPreconditionRequired, http.StatusBadRequest:
		return "bad_precondition"
	case http.StatusPreconditionFailed:
		return "conflict"
	case http.StatusConflict:
		return "idempotency_conflict"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusServiceUnavailable:
		return "database_failure"
	}
	if status >= http.StatusOK && status < http.StatusBadRequest {
		return "success"
	}
	if status >= http.StatusInternalServerError {
		return "server_error"
	}
	return "rejected"
}

func isAuditedRead(path string) bool {
	return path == "/admin/api/logs/export"
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
