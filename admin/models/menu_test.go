package models

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMenuCreateDoesNotGrantDefaultRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Role{}, &Menu{}, &CasbinRule{}))

	role := &Role{Name: "admin", Status: enum.Enabled}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, db.Exec(`UPDATE mss_boot_roles SET "default" = ? WHERE id = ?`, true, role.ID).Error)

	menu := &Menu{
		Name:       "menu.test",
		Path:       "/test",
		Method:     "GET",
		Component:  "./Test",
		Icon:       "experiment",
		Type:       pkg.MenuAccessType,
		Permission: "/test",
		Status:     enum.Enabled,
	}
	require.NoError(t, db.Create(menu).Error)

	var count int64
	require.NoError(t, db.Model(&CasbinRule{}).Where(&CasbinRule{
		PType: "p",
		V0:    role.ID,
		V1:    pkg.MenuAccessType.String(),
		V2:    "/test",
		V3:    "GET",
	}).Count(&count).Error)
	require.Zero(t, count)
}

func TestCompleteNamePreservesCanonicalAndCompletesRelativeNames(t *testing.T) {
	tree := []*Menu{
		{
			Name: "menu.origination",
			Children: []*Menu{
				{Name: "menu.origination.user"},
				{Name: "origination.department"},
				{Name: "post", Children: []*Menu{{Name: "create"}}},
			},
		},
		{
			Name:     "menu.super-permission",
			Children: []*Menu{{Name: "menu.system.appConfig"}},
		},
	}

	completed := CompleteName(tree)

	require.Equal(t, "menu.origination.user", completed[0].Children[0].Name)
	require.Equal(t, "menu.origination.department", completed[0].Children[1].Name)
	require.Equal(t, "menu.origination.post", completed[0].Children[2].Name)
	require.Equal(t, "menu.origination.post.create", completed[0].Children[2].Children[0].Name)
	require.Equal(t, "menu.system.appConfig", completed[1].Children[0].Name)
}
