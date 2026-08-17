package apis

import (
	"errors"
	"fmt"
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
	themeIfMatchRequiredCode  = "THEME_IF_MATCH_REQUIRED"
	themeContractMediaType    = "application/vnd.mss.theme.v1+json"
)

var errThemeIfMatchRequired = errors.New("theme mutation requires If-Match")

func themeETag(scope, revision string) string {
	return strconv.Quote("theme-" + scope + "-" + revision)
}

func setThemeETag(ctx *gin.Context, resource *dto.ThemeResource) {
	if resource == nil {
		return
	}
	ctx.Header("ETag", themeETag(resource.Meta.Scope, resource.Meta.Revision))
	ctx.Header("Cache-Control", "no-store")
}

func parseThemeIfMatch(ctx *gin.Context, scope string) (int64, error) {
	values := ctx.Request.Header.Values("If-Match")
	if len(values) == 0 {
		return 0, errThemeIfMatchRequired
	}
	if len(values) != 1 {
		return 0, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" {
		return 0, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	if strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") || raw == "*" {
		return 0, fmt.Errorf("%s: expected one strong theme ETag", themeIfMatchInvalidCode)
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: expected a quoted strong theme ETag", themeIfMatchInvalidCode)
	}
	prefix := "theme-" + scope + "-"
	if !strings.HasPrefix(value, prefix) {
		return 0, fmt.Errorf("%s: ETag scope does not match this resource", themeIfMatchInvalidCode)
	}
	revisionText := strings.TrimPrefix(value, prefix)
	if revisionText == "" || strings.Trim(revisionText, "0123456789") != "" {
		return 0, fmt.Errorf("%s: ETag revision must be a non-negative integer", themeIfMatchInvalidCode)
	}
	revision, err := strconv.ParseInt(revisionText, 10, 64)
	if err != nil || revision < 0 {
		return 0, fmt.Errorf("%s: ETag revision is out of range", themeIfMatchInvalidCode)
	}
	if raw != themeETag(scope, strconv.FormatInt(revision, 10)) {
		return 0, fmt.Errorf("%s: expected the canonical theme ETag", themeIfMatchInvalidCode)
	}
	return revision, nil
}

func writeThemeResource(ctx *gin.Context, resource *dto.ThemeResource) {
	setThemeETag(ctx, resource)
	ctx.Header("Content-Type", themeContractMediaType)
	ctx.AbortWithStatusJSON(http.StatusOK, resource)
}

func writeThemeRevisionConflict(ctx *gin.Context, err error) bool {
	var conflict *service.ThemeRevisionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	setThemeETag(ctx, conflict.Current)
	ctx.Header("Content-Type", themeContractMediaType)
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
