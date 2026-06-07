package system

import (
	"errors"
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260607162058SessionMenus)
}

// _20260607162058SessionMenus seeds the frontend menu entries and Casbin API
// policies for the online-sessions feature (PR #376 / issue #373):
//
//  1. A "/security" directory menu and its child "/security/online-sessions"
//     menu so non-root roles see the page after the frontend ships. The
//     MenuAccessType row triggers Menu.AfterCreate which automatically grants
//     the default role a MENU-type Casbin rule.
//  2. Five APIAccessType rows for the five new endpoints. These do NOT
//     auto-seed Casbin rules (Menu.AfterCreate only does that for MENU type),
//     so we insert API-type rules for the default role explicitly. The
//     default admin role is Root and bypasses Casbin, but seeding makes the
//     permissions discoverable in the admin UI and inheritable by future
//     non-root roles.
func _20260607162058SessionMenus(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		menus := []*models.Menu{
			{
				Name:       "menu.security",
				Path:       "/security",
				Method:     "GET",
				Icon:       "safetyCertificate",
				Type:       pkg.DirectoryAccessType,
				Permission: "/security",
				Status:     enum.Enabled,
				Sort:       40,
			},
			{
				Name:       "menu.security.onlineSessions",
				Path:       "/security/online-sessions",
				Method:     "GET",
				Component:  "./OnlineSession",
				Icon:       "userSwitch",
				ParentPath: "/security",
				Type:       pkg.MenuAccessType,
				Permission: "/security/online-sessions",
				Status:     enum.Enabled,
				Sort:       39,
			},
		}
		for i := range menus {
			if err := tx.Create(menus[i]).Error; err != nil {
				return err
			}
		}

		apiMenus := []*models.Menu{
			{Name: "api.online-sessions.list", Path: "/admin/api/online-sessions", Method: "GET",
				Type: pkg.APIAccessType, Permission: "/admin/api/online-sessions", Status: enum.Enabled},
			{Name: "api.online-sessions.get", Path: "/admin/api/online-sessions/:id", Method: "GET",
				Type: pkg.APIAccessType, Permission: "/admin/api/online-sessions/:id", Status: enum.Enabled},
			{Name: "api.online-sessions.revoke", Path: "/admin/api/online-sessions/:id", Method: "DELETE",
				Type: pkg.APIAccessType, Permission: "/admin/api/online-sessions/:id:DELETE", Status: enum.Enabled},
			{Name: "api.online-sessions.revokeByUser", Path: "/admin/api/online-sessions/user/:userID", Method: "DELETE",
				Type: pkg.APIAccessType, Permission: "/admin/api/online-sessions/user/:userID:DELETE", Status: enum.Enabled},
			{Name: "api.online-sessions.logout", Path: "/admin/api/online-sessions/logout", Method: "POST",
				Type: pkg.APIAccessType, Permission: "/admin/api/online-sessions/logout:POST", Status: enum.Enabled},
		}
		for i := range apiMenus {
			if err := tx.Create(apiMenus[i]).Error; err != nil {
				return err
			}
		}

		var defaultRole models.Role
		if err := tx.Model(&models.Role{}).Where("`default` = ?", true).First(&defaultRole).Error; err != nil {
			// No default role on this install (fresh DB before role seed) —
			// leave Casbin policies unseeded; admin role will be Root anyway.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return migration.Migrate.CreateVersion(tx, version)
			}
			return err
		}
		for _, m := range apiMenus {
			rule := &models.CasbinRule{
				PType: "p",
				V0:    defaultRole.ID,
				V1:    pkg.APIAccessType.String(),
				V2:    m.Path,
				V3:    m.Method,
			}
			if err := tx.Create(rule).Error; err != nil {
				return err
			}
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
