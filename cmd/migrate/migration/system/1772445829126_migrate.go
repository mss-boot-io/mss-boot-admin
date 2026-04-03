package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1772445829126Migrate)
}

func _1772445829126Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		// TODO: here to write the content to be changed

		// TODO: e.g. modify table field, please delete this code during use
		//err := tx.Migrator().RenameColumn(&models.SysConfig{}, "config_id", "id")
		//if err != nil {
		// 	return err
		//}

		// TODO: e.g. add table structure, please delete this code during use
		//err = tx.Migrator().AutoMigrate(
		//		new(models.CasbinRule),
		// 		)
		//if err != nil {
		// 	return err
		//}

		menus := []*models.Menu{
			{
				Name:       "menu.welcome",
				Path:       "/welcome",
				Method:     "GET",
				Component:  "./Welcome",
				Icon:       "smile",
				Type:       pkg.MenuAccessType,
				Permission: "/welcome",
				Status:     enum.Enabled,
				Sort:       100,
			},
			{
				Name:       "menu.origination",
				Path:       "/origination",
				Method:     "GET",
				Icon:       "apartment",
				Type:       pkg.DirectoryAccessType,
				Permission: "/origination",
				Status:     enum.Enabled,
				Sort:       90,
			},
			{
				Name:       "menu.origination.user",
				Path:       "/users",
				Method:     "GET",
				Component:  "./User",
				Icon:       "user",
				ParentPath: "/origination",
				Type:       pkg.MenuAccessType,
				Permission: "/users",
				Status:     enum.Enabled,
				Sort:       89,
			},
			{
				Name:       "menu.origination.department",
				Path:       "/departments",
				Method:     "GET",
				Component:  "./Department",
				Icon:       "cluster",
				ParentPath: "/origination",
				Type:       pkg.MenuAccessType,
				Permission: "/departments",
				Status:     enum.Enabled,
				Sort:       88,
			},
			{
				Name:       "menu.origination.post",
				Path:       "/posts",
				Method:     "GET",
				Component:  "./Post",
				Icon:       "idcard",
				ParentPath: "/origination",
				Type:       pkg.MenuAccessType,
				Permission: "/posts",
				Status:     enum.Enabled,
				Sort:       87,
			},
			{
				Name:       "menu.authority",
				Path:       "/authority",
				Method:     "GET",
				Icon:       "safetyCertificate",
				Type:       pkg.DirectoryAccessType,
				Permission: "/authority",
				Status:     enum.Enabled,
				Sort:       80,
			},
			{
				Name:       "menu.authority.role",
				Path:       "/role",
				Method:     "GET",
				Component:  "./Role",
				Icon:       "team",
				ParentPath: "/authority",
				Type:       pkg.MenuAccessType,
				Permission: "/role",
				Status:     enum.Enabled,
				Sort:       79,
			},
			{
				Name:       "menu.authority.menu",
				Path:       "/menu",
				Method:     "GET",
				Component:  "./Menu/index.tsx",
				Icon:       "menu",
				ParentPath: "/authority",
				Type:       pkg.MenuAccessType,
				Permission: "/menu",
				Status:     enum.Enabled,
				Sort:       78,
			},
			{
				Name:       "menu.system",
				Path:       "/system",
				Method:     "GET",
				Icon:       "setting",
				Type:       pkg.DirectoryAccessType,
				Permission: "/system",
				Status:     enum.Enabled,
				Sort:       70,
			},
			{
				Name:       "menu.system.task",
				Path:       "/task",
				Method:     "GET",
				Component:  "./Task",
				Icon:       "wallet",
				ParentPath: "/system",
				Type:       pkg.MenuAccessType,
				Permission: "/task",
				Status:     enum.Enabled,
				Sort:       69,
			},
			{
				Name:       "menu.system.language",
				Path:       "/language",
				Method:     "GET",
				Component:  "./Language",
				Icon:       "translation",
				ParentPath: "/system",
				Type:       pkg.MenuAccessType,
				Permission: "/language",
				Status:     enum.Enabled,
				Sort:       68,
			},
			{
				Name:       "menu.system.notice",
				Path:       "/notice",
				Method:     "GET",
				Component:  "./Notice",
				Icon:       "message",
				ParentPath: "/system",
				Type:       pkg.MenuAccessType,
				Permission: "/notice",
				Status:     enum.Enabled,
				Sort:       67,
			},
			{
				Name:       "menu.system.option",
				Path:       "/option",
				Method:     "GET",
				Component:  "./Option",
				Icon:       "unorderedList",
				ParentPath: "/system",
				Type:       pkg.MenuAccessType,
				Permission: "/option",
				Status:     enum.Enabled,
				Sort:       66,
			},
			{
				Name:       "menu.super-permission",
				Path:       "/super-permission",
				Method:     "GET",
				Icon:       "audit",
				Type:       pkg.DirectoryAccessType,
				Permission: "/super-permission",
				Status:     enum.Enabled,
				Sort:       60,
			},
			{
				Name:       "menu.super-permission.system-config",
				Path:       "/system-config",
				Method:     "GET",
				Component:  "./SystemConfig",
				Icon:       "inbox",
				ParentPath: "/super-permission",
				Type:       pkg.MenuAccessType,
				Permission: "/system-config",
				Status:     enum.Enabled,
				Sort:       59,
			},
			{
				Name:       "menu.system.appConfig",
				Path:       "/app-config",
				Method:     "GET",
				Component:  "./AppConfig",
				Icon:       "setting",
				ParentPath: "/super-permission",
				Type:       pkg.MenuAccessType,
				Permission: "/app-config",
				Status:     enum.Enabled,
				Sort:       58,
			},
			{
				Name:       "menu.develop",
				Path:       "/develop",
				Method:     "GET",
				Icon:       "tool",
				Type:       pkg.DirectoryAccessType,
				Permission: "/develop",
				Status:     enum.Enabled,
				Sort:       50,
			},
			{
				Name:       "menu.develop.model",
				Path:       "/model",
				Method:     "GET",
				Component:  "./Model",
				Icon:       "desktop",
				ParentPath: "/develop",
				Type:       pkg.MenuAccessType,
				Permission: "/model",
				Status:     enum.Enabled,
				Sort:       49,
			},
			{
				Name:       "menu.develop.generator",
				Path:       "/generator",
				Method:     "GET",
				Component:  "./Generator",
				Icon:       "form",
				ParentPath: "/develop",
				Type:       pkg.MenuAccessType,
				Permission: "/generator",
				Status:     enum.Enabled,
				Sort:       48,
			},
		}

		for i := range menus {
			if err := tx.Create(menus[i]).Error; err != nil {
				return err
			}
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
