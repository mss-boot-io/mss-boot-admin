package apis

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
)

const (
	roleAuthorizationRevisionConflictCode = "AUTHORIZATION_REVISION_CONFLICT"
	roleAuthorizationIfMatchInvalidCode   = "AUTHORIZATION_IF_MATCH_INVALID"
	roleAuthorizationInactiveCode         = "AUTHORIZATION_ROLE_INACTIVE"
	roleAuthorizationPreconditionHeader   = "X-MSS-Authorization-Precondition"
)

func requireCurrentRoot(ctx *gin.Context) bool {
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		response.Make(ctx).Err(http.StatusUnauthorized)
		return false
	}
	if !verify.Root() {
		response.Make(ctx).Err(http.StatusForbidden)
		return false
	}
	return true
}

func roleAuthorizationETag(roleID, revision string) string {
	return strconv.Quote("role-authorization-" + roleID + "-" + revision)
}

func setRoleAuthorizationETag(ctx *gin.Context, resource *dto.GetAuthorizeResponse) {
	if ctx == nil || resource == nil {
		return
	}
	ctx.Header("ETag", roleAuthorizationETag(resource.RoleID, resource.Revision))
	ctx.Header("Cache-Control", "no-store")
}

func setMissingRoleAuthorizationPreconditionWarning(ctx *gin.Context) {
	ctx.Header(roleAuthorizationPreconditionHeader, "missing")
	ctx.Header("Warning", `299 mss-boot "If-Match will be required for role authorization updates"`)
}

func parseRoleAuthorizationIfMatch(ctx *gin.Context, roleID string) (*int64, error) {
	if ctx == nil || ctx.Request == nil {
		return nil, fmt.Errorf("%s: request is missing", roleAuthorizationIfMatchInvalidCode)
	}
	values := ctx.Request.Header.Values("If-Match")
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("%s: expected one strong role authorization ETag", roleAuthorizationIfMatchInvalidCode)
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" || strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") || raw == "*" {
		return nil, fmt.Errorf("%s: expected one strong role authorization ETag", roleAuthorizationIfMatchInvalidCode)
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: expected a quoted strong role authorization ETag", roleAuthorizationIfMatchInvalidCode)
	}
	prefix := "role-authorization-" + roleID + "-"
	if !strings.HasPrefix(value, prefix) {
		return nil, fmt.Errorf("%s: ETag role does not match this resource", roleAuthorizationIfMatchInvalidCode)
	}
	revisionText := strings.TrimPrefix(value, prefix)
	if revisionText == "" || strings.Trim(revisionText, "0123456789") != "" {
		return nil, fmt.Errorf("%s: ETag revision must be a non-negative integer", roleAuthorizationIfMatchInvalidCode)
	}
	revision, err := strconv.ParseInt(revisionText, 10, 64)
	if err != nil || revision < 0 {
		return nil, fmt.Errorf("%s: ETag revision is out of range", roleAuthorizationIfMatchInvalidCode)
	}
	if raw != roleAuthorizationETag(roleID, strconv.FormatInt(revision, 10)) {
		return nil, fmt.Errorf("%s: expected the canonical role authorization ETag", roleAuthorizationIfMatchInvalidCode)
	}
	return &revision, nil
}

func writeRoleAuthorizationRevisionConflict(ctx *gin.Context, err error) bool {
	var conflict *service.AuthorizationRevisionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	setRoleAuthorizationETag(ctx, conflict.Current)
	ctx.AbortWithStatusJSON(http.StatusPreconditionFailed, dto.AuthorizeRevisionConflictResponse{
		Success:      false,
		Status:       "error",
		Code:         http.StatusPreconditionFailed,
		ErrorCode:    roleAuthorizationRevisionConflictCode,
		ErrorMessage: "role authorization changed since it was loaded",
		TraceID:      bootpkg.GenerateMsgIDFromContext(ctx),
		Data: dto.AuthorizeRevisionConflictResponseData{
			Current: conflict.Current,
		},
	})
	return true
}

func resolveAuthorizeRoleID(requestRoleID, pathRoleID string) string {
	roleID := strings.TrimSpace(requestRoleID)
	if roleID != "" {
		return roleID
	}
	return strings.TrimSpace(pathRoleID)
}

func sanitizeAuthorizePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for i := range paths {
		path := strings.TrimSpace(paths[i])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func respondInvalidAuthorizeRequest(api *response.API, message string, roleID string, invalid []string) {
	if len(invalid) > 0 {
		api.Log.Error(message, "roleID", roleID, "invalid", invalid)
	} else {
		api.Log.Error(message, "roleID", roleID)
	}
	api.Err(http.StatusUnprocessableEntity)
}

func hasEmptyAuthorizeRoleID(roleID string) bool {
	return strings.TrimSpace(roleID) == ""
}
