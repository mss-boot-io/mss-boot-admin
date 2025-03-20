package models

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/10 06:50:14
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/10 06:50:14
 */

func (t *Tenant) Migrate(tenantImp center.TenantImp, tx *gorm.DB) error {
	tenant := tenantImp.(*Tenant)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Referer", fmt.Sprintf("http://%s/", tenant.Domains[0].Domain))
	tx = tx.WithContext(c)

	role := Role{
		Name:   "admin",
		Status: enum.Enabled,
		Remark: "admin",
	}
	err := tx.Model(&role).Create(&role).Error
	if err != nil {
		return err
	}
	err = tx.Table(role.TableName()).Where("id = ?", role.ID).Updates(map[string]interface{}{
		"default": true,
		"root":    true,
	}).Error
	if err != nil {
		return err
	}
	user := &User{
		UserLogin: UserLogin{
			RoleID:   role.ID,
			Username: tenant.AdminUser.Username,
			Password: tenant.AdminUser.Password,
			Email:    "",
			Status:   enum.Enabled,
		},
		Name:     tenant.AdminUser.Username,
		Avatar:   "https://avatars.githubusercontent.com/u/12806223?v=4",
		Country:  "China",
		Province: "320000",
		City:     "320800",
		Address:  "一个有梦想的地方",
		Profile:  "https://docs.mss-boot-io.top",
		Title:    "管理员",
		Tags: ArrayString{
			"有想法",
		},
	}
	err = tx.Create(user).Error
	if err != nil {
		return err
	}
	department := &Department{
		Name:     "mss-boot-io",
		LeaderID: user.ID,
		Phone:    user.Phone,
		Email:    user.Email,
		Code:     "mss-boot-io",
		Status:   enum.Enabled,
		Sort:     99,
	}
	err = tx.Create(department).Error
	if err != nil {
		return err
	}
	post := &Post{
		Name:      "mss-boot-io",
		Code:      "mss-boot-io",
		Status:    enum.Enabled,
		Sort:      99,
		DataScope: DataScopeAll,
		DeptIDS:   department.ID,
	}
	err = tx.Create(post).Error
	if err != nil {
		return err
	}
	user.DepartmentID = department.ID
	user.PostID = post.ID

	err = tx.Model(user).Select("DepartmentID", "PostID").Updates(user).Error
	if err != nil {
		return err
	}

	modelMenu := Menu{
		Name: "model",
		Path: "/model",
		Icon: "desktop",
		Sort: 19,
		Type: pkg.MenuAccessType,
		Children: []*Menu{
			{
				Name:   "/admin/api/models",
				Path:   "/admin/api/models",
				Method: http.MethodGet,
				Type:   pkg.APIAccessType,
			},
			{
				Name:   "/admin/api/models/*",
				Path:   "/admin/api/models/:id",
				Method: http.MethodGet,
				Type:   pkg.APIAccessType,
			},
			{
				Name:       "control",
				Path:       "/model/:id",
				HideInMenu: true,
				Type:       pkg.MenuAccessType,
			},
			{
				Name:       "create",
				Path:       "/model/create",
				HideInMenu: true,
				Type:       pkg.ComponentAccessType,
				Children: []*Menu{
					{
						Name:   "/admin/api/models",
						Path:   "/admin/api/models",
						Method: http.MethodPost,
						Type:   pkg.APIAccessType,
					},
				},
			},
			{
				Name:       "edit",
				Path:       "/model/edit",
				HideInMenu: true,
				Type:       pkg.ComponentAccessType,
				Children: []*Menu{
					{
						Name:   "/admin/api/models/*",
						Path:   "/admin/api/models/:id",
						Method: http.MethodPut,
						Type:   pkg.APIAccessType,
					},
				},
			},
			{
				Name:       "field",
				Path:       "/model/field",
				HideInMenu: true,
				Type:       pkg.ComponentAccessType,
			},
			{
				Name:       "generate-data",
				Path:       "/model/generate-data",
				HideInMenu: true,
				Type:       pkg.ComponentAccessType,
				Children: []*Menu{
					{
						Name:   "/admin/api/model/generate-data",
						Path:   "/admin/api/model/generate-data",
						Method: http.MethodPut,
						Type:   pkg.APIAccessType,
					},
				},
			},
			{
				Name:       "delete",
				Path:       "/model/delete",
				HideInMenu: true,
				Type:       pkg.ComponentAccessType,
				Children: []*Menu{
					{
						Name:   "/admin/api/models/*",
						Path:   "/admin/api/models/:id",
						Method: http.MethodDelete,
						Type:   pkg.APIAccessType,
					},
				},
			},
		},
	}

	tenantMenu := []Menu{
		{
			Name: "super-permission",
			Path: "/super-permission",
			Icon: "audit",
			Sort: 16,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				{
					Name: "tenant",
					Path: "/tenant",
					Icon: "desktop",
					Sort: 20,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/tenants",
							Path:   "/admin/api/tenants",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/tenants/*",
							Path:   "/admin/api/tenants/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/tenant/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/tenant/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tenants",
									Path:   "/admin/api/tenants",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/tenant/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tenants/*",
									Path:   "/admin/api/tenants/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/tenant/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tenants/*",
									Path:   "/admin/api/tenants/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "system-config",
					Path: "/system-config",
					Icon: "inbox",
					Sort: 12,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/system-configs",
							Path:   "/admin/api/system-configs",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{

							Name:   "/admin/api/system-configs/*",
							Path:   "/admin/api/system-configs/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{

							Name:       "control",
							Path:       "/system-config/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{

							Name:       "create",
							Path:       "/system-config/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{

									Name:   "/admin/api/system-configs",
									Path:   "/admin/api/system-configs",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{

							Name:       "edit",
							Path:       "/system-config/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{

									Name:   "/admin/api/system-configs/*",
									Path:   "/admin/api/system-configs/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{

							Name:       "delete",
							Path:       "/system-config/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{

									Name:   "/admin/api/system-configs/*",
									Path:   "/admin/api/system-configs/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "develop",
			Path: "/development-tools",
			Icon: "tool",
			Sort: 15,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				&modelMenu,
				{

					Name: "generator",
					Path: "/generator",
					Icon: "form",
					Sort: 18,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/template/get-branches",
							Path:   "/admin/api/template/get-branches",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/template/get-path",
							Path:   "/admin/api/template/get-path",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/template/get-params",
							Path:   "/admin/api/template/get-params",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/template/generate",
							Path:   "/admin/api/template/generate",
							Method: http.MethodPost,
							Type:   pkg.APIAccessType,
						},
					},
				},
			},
		},
	}

	// init menu
	menus := []Menu{
		{
			Name: "dashboard",
			Path: "/dashboard",
			Icon: "smile",
			Sort: 20,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				{
					Name: "workplace",
					Path: "/workplace",
					Sort: 20,
					Type: pkg.MenuAccessType,
				},
				{
					Name: "analysis",
					Path: "/analysis",
					Sort: 19,
					Type: pkg.MenuAccessType,
				},
			},
		},
		{
			Name: "system",
			Path: "/system",
			Icon: "setting",
			Sort: 19,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				{
					Name: "appConfig",
					Path: "/app-config",
					Icon: "setting",
					Sort: 21,
					Type: pkg.MenuAccessType,
				},
				{
					Name: "task",
					Path: "/task",
					Icon: "wallet",
					Type: pkg.MenuAccessType,
					Sort: 20,
					Children: []*Menu{
						{
							Name:   "/admin/api/tasks",
							Path:   "/admin/api/tasks",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/tasks/*",
							Path:   "/admin/api/tasks/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/task/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/task/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tasks",
									Path:   "/admin/api/tasks",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/task/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tasks/*",
									Path:   "/admin/api/tasks/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/task/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tasks/*",
									Path:   "/admin/api/tasks/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "operate",
							Path:       "/task/operate",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/tasks/*/*",
									Path:   "/admin/api/tasks/:operate/:id",
									Method: http.MethodGet,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "language",
					Path: "/language",
					Icon: "translation",
					Sort: 15,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/languages",
							Path:   "/admin/api/languages",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/languages/*",
							Path:   "/admin/api/languages/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/language/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/language/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/languages",
									Path:   "/admin/api/languages",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/language/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/languages/*",
									Path:   "/admin/api/languages/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/language/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/languages/*",
									Path:   "/admin/api/languages/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "option",
					Path: "/option",
					Icon: "message",
					Sort: 13,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/options",
							Path:   "/admin/api/options",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/options/*",
							Path:   "/admin/api/options/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/option/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/option/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/options",
									Path:   "/admin/api/options",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/option/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/options/*",
									Path:   "/admin/api/options/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/option/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/options/*",
									Path:   "/admin/api/options/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "notice",
					Path: "/notice",
					Icon: "message",
					Sort: 12,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/notices",
							Path:   "/admin/api/notices",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/notices/*",
							Path:   "/admin/api/notices/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/notice/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/notice/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/notices",
									Path:   "/admin/api/notices",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/notice/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/notices/*",
									Path:   "/admin/api/notices/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/notice/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/notices/*",
									Path:   "/admin/api/notices/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "read",
							Path:       "/notice/read",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/notice/read/*",
									Path:   "/notice/read/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "origination",
			Path: "/origination",
			Icon: "apartment",
			Sort: 18,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				{
					Name: "department",
					Path: "/departments",
					Sort: 20,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/departments",
							Path:   "/admin/api/departments",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/departments/*",
							Path:   "/admin/api/departments/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/departments/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/departments/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/departments",
									Path:   "/admin/api/departments",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/departments/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/departments/*",
									Path:   "/admin/api/departments/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/departments/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/departments/*",
									Path:   "/admin/api/departments/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "post",
					Path: "/post",
					Sort: 19,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/posts",
							Path:   "/admin/api/posts",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/posts/*",
							Path:   "/admin/api/posts/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/posts/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/posts/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/posts",
									Path:   "/admin/api/posts",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/posts/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/posts/*",
									Path:   "/admin/api/posts/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/posts/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/posts/*",
									Path:   "/admin/api/posts/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "user",
					Path: "/users",
					Icon: "user",
					Sort: 18,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/users",
							Path:   "/admin/api/users",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/users/*",
							Path:   "/admin/api/users/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/users/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/users/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/users",
									Path:   "/admin/api/users",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/users/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/users/*",
									Path:   "/admin/api/users/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/users/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/users/*",
									Path:   "/admin/api/users/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "password-reset",
							Path:       "/users/password-reset",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/user/*/password-reset",
									Path:   "/admin/api/user/:id/password-reset",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "authority",
			Path: "/authority",
			Icon: "safetyCertificate",
			Sort: 17,
			Type: pkg.DirectoryAccessType,
			Children: []*Menu{
				{
					Name: "role",
					Path: "/role",
					Sort: 18,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/roles",
							Path:   "/admin/api/roles",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/roles/*",
							Path:   "/admin/api/roles/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/role/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "auth",
							Path:       "/role/auth",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/menu/tree",
									Path:   "/admin/api/menu/tree",
									Method: http.MethodGet,
									Type:   pkg.APIAccessType,
								},
								{
									Name:   "/admin/api/role/authorize/*",
									Path:   "/admin/api/role/authorize/:id",
									Method: http.MethodGet,
									Type:   pkg.APIAccessType,
								},
								{
									Name:   "/admin/api/role/authorize/*",
									Path:   "/admin/api/role/authorize/:id",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "create",
							Path:       "/role/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/roles",
									Path:   "/admin/api/roles",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/role/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/roles/*",
									Path:   "/admin/api/roles/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/role/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/roles/*",
									Path:   "/admin/api/roles/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
				{
					Name: "menu",
					Path: "/menu",
					Sort: 16,
					Type: pkg.MenuAccessType,
					Children: []*Menu{
						{
							Name:   "/admin/api/menus",
							Path:   "/admin/api/menus",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:   "/admin/api/menus/*",
							Path:   "/admin/api/menus/:id",
							Method: http.MethodGet,
							Type:   pkg.APIAccessType,
						},
						{
							Name:       "control",
							Path:       "/menu/:id",
							HideInMenu: true,
							Type:       pkg.MenuAccessType,
						},
						{
							Name:       "create",
							Path:       "/menu/create",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/menus",
									Path:   "/admin/api/menus",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "edit",
							Path:       "/menu/edit",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/menus/*",
									Path:   "/admin/api/menus/:id",
									Method: http.MethodPut,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "delete",
							Path:       "/menu/delete",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/menus/*",
									Path:   "/admin/api/menus/:id",
									Method: http.MethodDelete,
									Type:   pkg.APIAccessType,
								},
							},
						},
						{
							Name:       "bind-api",
							Path:       "/menu/bind-api",
							HideInMenu: true,
							Type:       pkg.ComponentAccessType,
							Children: []*Menu{
								{
									Name:   "/admin/api/menu/bind-api",
									Path:   "/admin/api/menu/bind-api",
									Method: http.MethodPost,
									Type:   pkg.APIAccessType,
								},
							},
						},
					},
				},
			},
		},
	}
	if tenant.Default {
		menus = append(menus, tenantMenu...)
	}
	err = tx.Create(&menus).Error
	if err != nil {
		return err
	}

	languages := []Language{
		{

			Name:   "zh-CN",
			Remark: "简体中文",
			Status: enum.Enabled,
		},
		{

			Name:   "en-US",
			Remark: "English",
			Status: enum.Enabled,
		},
	}
	err = tx.Create(&languages).Error
	if err != nil {
		return err
	}

	systemItems := OptionItems{
		{
			Key:   "enabled",
			Label: "启用",
			Value: "enabled",
			Color: "green",
			Sort:  0,
		},
		{
			Key:   "disabled",
			Label: "禁用",
			Value: "disabled",
			Color: "red",
			Sort:  1,
		},
		{
			Key:   "locked",
			Label: "锁定",
			Value: "locked",
			Color: "orange",
			Sort:  2,
		},
	}
	options := []Option{
		{
			Name:   "system.status",
			Remark: "系统状态",
			Items:  &systemItems,
		},
	}
	err = tx.Create(&options).Error
	if err != nil {
		return err
	}
	return nil
}
