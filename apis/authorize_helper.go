package apis

import (
	"fmt"
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

func buildMenuAuthorizeRules(roleID string, keys []string) []*models.CasbinRule {
	rules := make([]*models.CasbinRule, len(keys))
	for i := range keys {
		rules[i] = &models.CasbinRule{
			PType: "p",
			V0:    roleID,
			V1:    pkg.MenuAccessType.String(),
			V2:    keys[i],
		}
	}
	return rules
}

func buildRoleAuthorizeRules(roleID string, menus []*models.Menu) []*models.CasbinRule {
	rules := make([]*models.CasbinRule, 0, len(menus)*2)
	seen := make(map[string]struct{}, len(menus)*2)
	for i := range menus {
		rules = appendRuleIfNotExists(rules, seen, roleID, menus[i].Type.String(), menus[i].Path, menus[i].Method)
		for j := range menus[i].Children {
			if menus[i].Children[j].Type != pkg.APIAccessType {
				continue
			}
			rules = appendRuleIfNotExists(rules, seen, roleID, pkg.APIAccessType.String(), menus[i].Children[j].Path, menus[i].Children[j].Method)
		}
	}
	return rules
}

func appendRuleIfNotExists(rules []*models.CasbinRule, seen map[string]struct{}, roleID, accessType, path, method string) []*models.CasbinRule {
	key := fmt.Sprintf("%s|%s|%s|%s", roleID, accessType, path, method)
	if _, ok := seen[key]; ok {
		return rules
	}
	seen[key] = struct{}{}
	rules = append(rules, &models.CasbinRule{
		PType: "p",
		V0:    roleID,
		V1:    accessType,
		V2:    path,
		V3:    method,
	})
	return rules
}
