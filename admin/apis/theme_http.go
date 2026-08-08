package apis

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
)

const (
	themeRevisionConflictCode = "THEME_REVISION_CONFLICT"
	themeIfMatchInvalidCode   = "THEME_IF_MATCH_INVALID"
	themeContractMediaType    = "application/vnd.mss.theme.v1+json"
)

func wantsCanonicalThemeContract(ctx *gin.Context) bool {
	addVaryHeader(ctx.Writer.Header(), "Accept")
	for _, raw := range ctx.Request.Header.Values("Accept") {
		for _, candidate := range splitAcceptHeader(raw) {
			mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(candidate))
			if err == nil && strings.EqualFold(mediaType, themeContractMediaType) && hasPositiveAcceptQuality(parameters) {
				return true
			}
		}
	}
	return false
}

func splitAcceptHeader(value string) []string {
	entries := make([]string, 0, 1)
	start := 0
	inQuotes := false
	escaped := false
	for index := 0; index < len(value); index++ {
		current := value[index]
		if escaped {
			escaped = false
			continue
		}
		if inQuotes && current == '\\' {
			escaped = true
			continue
		}
		if current == '"' {
			inQuotes = !inQuotes
			continue
		}
		if current == ',' && !inQuotes {
			entries = append(entries, value[start:index])
			start = index + 1
		}
	}
	return append(entries, value[start:])
}

func hasPositiveAcceptQuality(parameters map[string]string) bool {
	rawQuality := ""
	hasQuality := false
	for name, value := range parameters {
		if strings.EqualFold(name, "q") {
			rawQuality = value
			hasQuality = true
			break
		}
	}
	if !hasQuality {
		return true
	}
	quality, valid := parseAcceptQuality(rawQuality)
	return valid && quality > 0
}

func parseAcceptQuality(value string) (float64, bool) {
	whole := value
	fraction := ""
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		whole = value[:dot]
		fraction = value[dot+1:]
	}
	if (whole != "0" && whole != "1") || len(fraction) > 3 {
		return 0, false
	}
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || (whole == "1" && digit != '0') {
			return 0, false
		}
	}
	quality, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return quality, true
}

func addVaryHeader(header http.Header, name string) {
	for _, value := range header.Values("Vary") {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), name) {
				return
			}
		}
	}
	header.Add("Vary", name)
}

func themeETag(scope, revision string) string {
	return strconv.Quote("theme-" + scope + "-" + revision)
}

func setThemeETag(ctx *gin.Context, resource *dto.ThemeResource) {
	if resource == nil {
		return
	}
	ctx.Header("ETag", themeETag(resource.Meta.Scope, resource.Meta.Revision))
	ctx.Header("Cache-Control", "no-store")
	addVaryHeader(ctx.Writer.Header(), "Accept")
}

func parseThemeIfMatch(ctx *gin.Context, scope string) (*int64, error) {
	values := ctx.Request.Header.Values("If-Match")
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" {
		return nil, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	if strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") || raw == "*" {
		return nil, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: expected a quoted strong theme ETag", themeIfMatchInvalidCode)
	}
	prefix := "theme-" + scope + "-"
	if !strings.HasPrefix(value, prefix) {
		return nil, fmt.Errorf("%s: ETag scope does not match this resource", themeIfMatchInvalidCode)
	}
	revisionText := strings.TrimPrefix(value, prefix)
	if revisionText == "" || strings.Trim(revisionText, "0123456789") != "" {
		return nil, fmt.Errorf("%s: ETag revision must be a non-negative integer", themeIfMatchInvalidCode)
	}
	revision, err := strconv.ParseInt(revisionText, 10, 64)
	if err != nil || revision < 0 {
		return nil, fmt.Errorf("%s: ETag revision is out of range", themeIfMatchInvalidCode)
	}
	if raw != themeETag(scope, strconv.FormatInt(revision, 10)) {
		return nil, fmt.Errorf("%s: expected the canonical theme ETag", themeIfMatchInvalidCode)
	}
	return &revision, nil
}

func writeThemeResource(ctx *gin.Context, resource *dto.ThemeResource) {
	setThemeETag(ctx, resource)
	ctx.Header("Content-Type", themeContractMediaType)
	ctx.AbortWithStatusJSON(http.StatusOK, resource)
}

func writeThemeRevisionConflict(ctx *gin.Context, err error, canonicalContract bool) bool {
	var conflict *service.ThemeRevisionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	setThemeETag(ctx, conflict.Current)
	if canonicalContract {
		ctx.Header("Content-Type", themeContractMediaType)
	}
	ctx.AbortWithStatusJSON(http.StatusPreconditionFailed, dto.ThemeRevisionConflictResponse{
		Success:      false,
		Status:       "error",
		Code:         http.StatusPreconditionFailed,
		ErrorCode:    themeRevisionConflictCode,
		ErrorMessage: "theme settings changed since they were loaded",
		TraceID:      bootpkg.GenerateMsgIDFromContext(ctx),
		Data: dto.ThemeRevisionConflictResponseData{
			Current: conflict.Current,
		},
	})
	return true
}

func submittedThemeKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
