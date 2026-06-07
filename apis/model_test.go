package apis

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/models"
	adminPKG "github.com/mss-boot-io/mss-boot-admin/pkg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteGeneratedModelMenusRemovesGeneratedMenuTreeAndPolicies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:model-menu-cleanup?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Model{}, &models.Menu{}, &models.CasbinRule{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	model := &models.Model{
		Name:  "Orders",
		Table: "orders",
		Path:  "orders",
	}
	model.ID = "model-orders"
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	menus := []*models.Menu{
		{ParentID: "", Name: "Orders", Path: "/virtual/orders", Type: adminPKG.MenuAccessType, Method: "GET"},
		{ParentID: "menu-root", Name: "Orders API", Path: "/admin/api/orders", Type: adminPKG.APIAccessType, Method: "GET"},
		{ParentID: "menu-root", Name: "Orders Delete", Path: "/virtual/orders/delete", Type: adminPKG.ComponentAccessType, Method: "GET"},
		{ParentID: "", Name: "Keep", Path: "/virtual/keep", Type: adminPKG.MenuAccessType, Method: "GET"},
	}
	menus[0].ID = "menu-root"
	menus[1].ID = "menu-api"
	menus[2].ID = "menu-delete"
	menus[3].ID = "menu-keep"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&menus).Error; err != nil {
		t.Fatalf("create menus: %v", err)
	}

	rules := []*models.CasbinRule{
		{ID: 1, PType: "p", V0: "role-1", V1: adminPKG.MenuAccessType.String(), V2: "/virtual/orders", V3: "GET"},
		{ID: 2, PType: "p", V0: "role-1", V1: adminPKG.APIAccessType.String(), V2: "/admin/api/orders", V3: "GET"},
		{ID: 3, PType: "p", V0: "role-1", V1: adminPKG.MenuAccessType.String(), V2: "/virtual/keep", V3: "GET"},
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatalf("create rules: %v", err)
	}

	if err := db.Delete(model).Error; err != nil {
		t.Fatalf("delete model: %v", err)
	}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("ids", []string{model.ID})

	if err := deleteGeneratedModelMenus(ctx, db, &models.Model{}); err != nil {
		t.Fatalf("delete generated menus: %v", err)
	}

	var deletedGeneratedMenus int64
	if err := db.Unscoped().Model(&models.Menu{}).
		Where("id IN ?", []string{"menu-root", "menu-api", "menu-delete"}).
		Where("deleted_at IS NOT NULL").
		Count(&deletedGeneratedMenus).Error; err != nil {
		t.Fatalf("count deleted menus: %v", err)
	}
	if deletedGeneratedMenus != 3 {
		t.Fatalf("expected 3 generated menus deleted, got %d", deletedGeneratedMenus)
	}

	var activeKeepMenus int64
	if err := db.Model(&models.Menu{}).Where("id = ?", "menu-keep").Count(&activeKeepMenus).Error; err != nil {
		t.Fatalf("count keep menu: %v", err)
	}
	if activeKeepMenus != 1 {
		t.Fatalf("expected unrelated menu to stay active, got %d", activeKeepMenus)
	}

	var generatedRules int64
	if err := db.Model(&models.CasbinRule{}).
		Where("v2 IN ?", []string{"/virtual/orders", "/admin/api/orders", "/virtual/orders/delete"}).
		Count(&generatedRules).Error; err != nil {
		t.Fatalf("count generated rules: %v", err)
	}
	if generatedRules != 0 {
		t.Fatalf("expected generated policies removed, got %d", generatedRules)
	}

	var keepRules int64
	if err := db.Model(&models.CasbinRule{}).Where("v2 = ?", "/virtual/keep").Count(&keepRules).Error; err != nil {
		t.Fatalf("count keep rules: %v", err)
	}
	if keepRules != 1 {
		t.Fatalf("expected unrelated policy to stay, got %d", keepRules)
	}
}

func TestDeleteGeneratedModelMenusKeepsMenuWhenAnotherActiveModelUsesPath(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:model-menu-active-path?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Model{}, &models.Menu{}, &models.CasbinRule{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	deletedModel := &models.Model{Name: "Orders Old", Table: "orders_old", Path: "orders"}
	deletedModel.ID = "model-orders-old"
	activeModel := &models.Model{Name: "Orders Active", Table: "orders_active", Path: "orders"}
	activeModel.ID = "model-orders-active"
	if err := db.Create(deletedModel).Error; err != nil {
		t.Fatalf("create deleted model: %v", err)
	}
	if err := db.Create(activeModel).Error; err != nil {
		t.Fatalf("create active model: %v", err)
	}

	menu := &models.Menu{ParentID: "", Name: "Orders", Path: "/virtual/orders", Type: adminPKG.MenuAccessType, Method: "GET"}
	menu.ID = "menu-root"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
	rule := &models.CasbinRule{ID: 1, PType: "p", V0: "role-1", V1: adminPKG.MenuAccessType.String(), V2: "/virtual/orders", V3: "GET"}
	if err := db.Create(rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if err := db.Delete(deletedModel).Error; err != nil {
		t.Fatalf("delete model: %v", err)
	}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("ids", []string{deletedModel.ID})

	if err := deleteGeneratedModelMenus(ctx, db, &models.Model{}); err != nil {
		t.Fatalf("delete generated menus: %v", err)
	}

	var activeMenus int64
	if err := db.Model(&models.Menu{}).Where("id = ?", menu.ID).Count(&activeMenus).Error; err != nil {
		t.Fatalf("count menu: %v", err)
	}
	if activeMenus != 1 {
		t.Fatalf("expected shared-path menu to remain active, got %d", activeMenus)
	}

	var activeRules int64
	if err := db.Model(&models.CasbinRule{}).Where("v2 = ?", "/virtual/orders").Count(&activeRules).Error; err != nil {
		t.Fatalf("count rule: %v", err)
	}
	if activeRules != 1 {
		t.Fatalf("expected shared-path policy to remain, got %d", activeRules)
	}
}
