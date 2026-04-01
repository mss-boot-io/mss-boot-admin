package apis

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"gorm.io/gorm"
)

func resolveAuthorizeRoleID(requestRoleID, pathRoleID string) string {
	roleID := strings.TrimSpace(requestRoleID)
	if roleID != "" {
		return roleID
	}
	return strings.TrimSpace(pathRoleID)
}

func checkAuthorizeRoleExists(ctx *gin.Context, roleID string) (bool, error) {
	err := center.Default.GetDB(ctx, &models.Role{}).
		Where("id = ?", roleID).
		First(&models.Role{}).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, err
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

func loadAuthorizeMenusByPaths(ctx *gin.Context, paths []string, accessTypes ...pkg.AccessType) ([]*models.Menu, map[string]struct{}, error) {
	menus := make([]*models.Menu, 0)
	err := center.Default.GetDB(ctx, &models.Menu{}).Model(&models.Menu{}).
		Where("path in (?)", paths).
		Where("type in (?)", accessTypes).
		Find(&menus).Error
	if err != nil {
		return nil, nil, err
	}
	menuSet := make(map[string]struct{}, len(menus))
	for i := range menus {
		menuSet[menus[i].Path] = struct{}{}
	}
	return menus, menuSet, nil
}

func missingAuthorizePaths(paths []string, loaded map[string]struct{}) []string {
	missing := make([]string, 0)
	for i := range paths {
		if _, ok := loaded[paths[i]]; ok {
			continue
		}
		missing = append(missing, paths[i])
	}
	return missing
}

func authorizePathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for i := range paths {
		set[paths[i]] = struct{}{}
	}
	return set
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
