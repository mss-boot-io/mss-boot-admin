package apis

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/center"

	adminPKG "github.com/mss-boot-io/mss-boot-admin/pkg"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 12:58:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 12:58:01
 */

func init() {
	e := &Model{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Model)),
			controller.WithSearch(new(dto.ModelSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithAfterDelete(deleteGeneratedModelMenus),
		),
	}
	response.AppendController(e)
}

type Model struct {
	*controller.Simple
}

func (e *Model) Other(r *gin.RouterGroup) {
	r.PUT("/model/generate-data", e.GenerateData)
}

// GenerateData 生成数据
// @Summary 生成数据
// @Description 生成数据
// @Tags model
// @Param data body dto.ModelGenerateDataRequest true "data"
// @Success 200
// @Router /admin/api/model/generate-data [put]
// @Security Bearer
func (e *Model) GenerateData(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.ModelGenerateDataRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	m := &models.Model{}
	err := center.Default.GetDB(ctx, m).Preload("Fields").First(m, "id = ?", req.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("get error")
		api.Err(http.StatusInternalServerError)
		return
	}
	tx := center.Default.GetDB(ctx, m).Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()
	err = tx.Model(m).Where("id = ?", m.ID).Update("generated_data", true).Error
	if err != nil {
		api.AddError(err).Log.Error("update error")
		api.Err(http.StatusInternalServerError)
		return
	}
	err = e.migrate(api, tx, m)
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	err = e.menu(api, tx, m, req)
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	err = e.i18n(api, tx, m, req)
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(struct{}{})
}

func (e *Model) migrate(api *response.API, tx *gorm.DB, m *models.Model) error {

	vm := m.MakeVirtualModel()
	if vm == nil {
		return fmt.Errorf("make virtual model error")
	}
	err := vm.Migrate(tx)
	if err != nil {
		api.AddError(err).Log.Error("migrate error")
		return err
	}
	return nil
}

func (e *Model) menu(api *response.API, tx *gorm.DB, m *models.Model, req *dto.ModelGenerateDataRequest) error {
	basePath := "/admin/api/" + m.Path
	menu := &models.Menu{
		Name:     m.Name,
		ParentID: req.MenuParentID,
		Path:     "/virtual/" + m.Path,
		Type:     adminPKG.MenuAccessType,
		Children: []*models.Menu{
			{
				Name:   basePath,
				Path:   basePath,
				Method: http.MethodGet,
				Type:   adminPKG.APIAccessType,
			},
			{
				Name:   basePath + "/*",
				Path:   basePath + "/:id",
				Method: http.MethodGet,
				Type:   adminPKG.APIAccessType,
			},
			{
				Name:       "control",
				Path:       "/virtual/" + m.Path + "/:id",
				HideInMenu: true,
				Type:       adminPKG.MenuAccessType,
			},
			{
				Name:       "create",
				Path:       "/virtual/" + m.Path + "/create",
				HideInMenu: true,
				Type:       adminPKG.ComponentAccessType,
				Children: []*models.Menu{
					{
						Name:   basePath,
						Path:   basePath,
						Method: http.MethodPost,
						Type:   adminPKG.APIAccessType,
					},
				},
			},
			{
				Name:       "edit",
				Path:       "/virtual/" + m.Path + "/edit",
				HideInMenu: true,
				Type:       adminPKG.ComponentAccessType,
				Children: []*models.Menu{
					{
						Name:   basePath + "/*",
						Path:   basePath + "/:id",
						Method: http.MethodPut,
						Type:   adminPKG.APIAccessType,
					},
				},
			},
			{
				Name:       "delete",
				Path:       "/virtual/" + m.Path + "/delete",
				HideInMenu: true,
				Type:       adminPKG.ComponentAccessType,
				Children: []*models.Menu{
					{
						Name:   basePath + "/*",
						Path:   basePath + "/:id",
						Method: http.MethodDelete,
						Type:   adminPKG.APIAccessType,
					},
				},
			},
		},
	}

	var count int64
	tx.Model(menu).Where("path = ?", menu.Path).Count(&count)
	if count > 0 {
		return nil
	}
	err := tx.Create(menu).Error
	if err != nil {
		api.AddError(err).Log.Error("create error")
		return err
	}
	return nil
}

func (e *Model) i18n(api *response.API, tx *gorm.DB, m *models.Model, req *dto.ModelGenerateDataRequest) error {
	languages := make([]*models.Language, 0)
	err := tx.Find(&languages).Error
	if err != nil {
		return nil
	}
	i18nKey := m.Name
	if req.MenuParentID != "" {
		list := make([]*models.Menu, 0)
		err := tx.Where("type <> ?", adminPKG.APIAccessType).
			Find(&list).Error
		if err != nil {
			api.AddError(err).Log.Error("get menu tree error")
			return err
		}
		list = models.CompleteName(
			models.TreeTransferToMenuSlice(
				adminPKG.BuildTree(
					models.MenuTransferToTreeSlice(list), "")))
		var ok bool
		for i := range list {
			if list[i].ID == req.MenuParentID {
				i18nKey = list[i].Name + "." + i18nKey
				ok = true
				break
			}
			for j := range list[i].Children {
				if list[i].Children[j].ID == req.MenuParentID {
					i18nKey = list[i].Name + "." + list[i].Children[j].Name + "." + i18nKey
					ok = true
					break
				}
			}
			if ok {
				break
			}
		}
	}

	for i := range languages {
		defines := []*models.LanguageDefine{
			{
				ID:    adminPKG.SimpleID(),
				Group: "menu",
				Key:   i18nKey,
				Value: m.Name + " List",
			},
			{
				ID:    adminPKG.SimpleID(),
				Group: "menu",
				Key:   i18nKey + ".control",
				Value: "Manage " + m.Name,
			},
			{
				ID:    adminPKG.SimpleID(),
				Group: "pages",
				Key:   m.Name + ".list.title",
				Value: m.Name + " List",
			},
		}
		if languages[i].Name == "zh-CN" {
			defines = []*models.LanguageDefine{
				{
					ID:    adminPKG.SimpleID(),
					Group: "menu",
					Key:   i18nKey,
					Value: m.Name + "列表",
				},
				{
					ID:    adminPKG.SimpleID(),
					Group: "menu",
					Key:   i18nKey + ".control",
					Value: "管理" + m.Name,
				},
				{
					ID:    adminPKG.SimpleID(),
					Group: "pages",
					Key:   m.Name + ".list.title",
					Value: m.Name + "列表",
				},
			}
		}
		if languages[i].Defines == nil {
			languageDefines := models.LanguageDefines(defines)
			languages[i].Defines = &languageDefines
			continue
		}
		var existList, existControl bool
		for j := range *languages[i].Defines {
			if (*languages[i].Defines)[j].Key == defines[0].Key {
				(*languages[i].Defines)[j].Value = defines[0].Value
				existList = true
				continue
			}
			if (*languages[i].Defines)[j].Key == defines[1].Key {
				(*languages[i].Defines)[j].Value = defines[1].Value
				existList = true
				continue
			}
		}
		if !existList {
			*languages[i].Defines = append(*languages[i].Defines, defines[0])
		}
		if !existControl {
			*languages[i].Defines = append(*languages[i].Defines, defines[1])
		}
	}
	err = tx.Save(&languages).Error
	if err != nil {
		api.AddError(err).Log.Error("save error")
		return err
	}
	return nil
}

type generatedMenuPolicyTarget struct {
	ID     string
	Path   string
	Method string
	Type   adminPKG.AccessType
}

func deleteGeneratedModelMenus(ctx *gin.Context, db *gorm.DB, _ schema.Tabler) error {
	idsValue, ok := ctx.Get("ids")
	if !ok {
		return nil
	}
	modelIDs, ok := idsValue.([]string)
	if !ok || len(modelIDs) == 0 {
		return nil
	}

	modelsToClean := make([]*models.Model, 0, len(modelIDs))
	if err := db.Unscoped().Where("id IN ?", modelIDs).Find(&modelsToClean).Error; err != nil {
		return err
	}

	changed := false
	if err := db.Transaction(func(tx *gorm.DB) error {
		rootPaths, err := generatedMenuRootPaths(tx, modelsToClean)
		if err != nil {
			return err
		}
		if len(rootPaths) == 0 {
			return nil
		}

		rootIDs := make([]string, 0, len(rootPaths))
		if err := tx.Model(&models.Menu{}).
			Where("type = ? AND path IN ?", adminPKG.MenuAccessType, rootPaths).
			Pluck("id", &rootIDs).Error; err != nil {
			return err
		}
		if len(rootIDs) == 0 {
			return nil
		}

		menuTargets, err := collectGeneratedMenuHierarchy(tx, rootIDs)
		if err != nil {
			return err
		}
		if err := softDeleteGeneratedMenuHierarchy(tx, rootIDs); err != nil {
			return err
		}
		if err := deleteGeneratedMenuPolicies(tx, menuTargets); err != nil {
			return err
		}
		changed = true
		return nil
	}); err != nil {
		return err
	}
	if changed && gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			slog.Warn("reload casbin policy failed after model menu cleanup", "err", err)
		}
	}
	return nil
}

func generatedMenuRootPaths(db *gorm.DB, modelsToClean []*models.Model) ([]string, error) {
	modelPaths := make([]string, 0, len(modelsToClean))
	seenModelPaths := make(map[string]struct{}, len(modelsToClean))
	for i := range modelsToClean {
		if !modelsToClean[i].GeneratedData || modelsToClean[i].Path == "" {
			continue
		}
		if _, ok := seenModelPaths[modelsToClean[i].Path]; ok {
			continue
		}
		seenModelPaths[modelsToClean[i].Path] = struct{}{}
		modelPaths = append(modelPaths, modelsToClean[i].Path)
	}
	if len(modelPaths) == 0 {
		return nil, nil
	}

	activePaths := make([]string, 0)
	if err := db.Model(&models.Model{}).Where("path IN ?", modelPaths).Pluck("path", &activePaths).Error; err != nil {
		return nil, err
	}
	activePathSet := make(map[string]struct{}, len(activePaths))
	for i := range activePaths {
		activePathSet[activePaths[i]] = struct{}{}
	}

	paths := make([]string, 0, len(modelsToClean))
	seen := make(map[string]struct{}, len(modelsToClean))
	for i := range modelsToClean {
		if !modelsToClean[i].GeneratedData || modelsToClean[i].Path == "" {
			continue
		}
		if _, ok := activePathSet[modelsToClean[i].Path]; ok {
			continue
		}
		path := "/virtual/" + modelsToClean[i].Path
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths, nil
}

func collectGeneratedMenuHierarchy(db *gorm.DB, rootIDs []string) ([]generatedMenuPolicyTarget, error) {
	rows := make([]generatedMenuPolicyTarget, 0)
	menu := &models.Menu{}
	sqlTemp := fmt.Sprintf(`WITH RECURSIVE MenuHierarchy AS (
    SELECT id, path, method, type
    FROM %s
    WHERE id IN ? AND deleted_at IS NULL
    UNION ALL
    SELECT m.id, m.path, m.method, m.type
    FROM %s m
    INNER JOIN MenuHierarchy mh ON m.parent_id = mh.id
    WHERE m.deleted_at IS NULL
)
SELECT id, path, method, type FROM MenuHierarchy`, menu.TableName(), menu.TableName())
	return rows, db.Raw(sqlTemp, rootIDs).Scan(&rows).Error
}

func softDeleteGeneratedMenuHierarchy(db *gorm.DB, rootIDs []string) error {
	menu := &models.Menu{}
	sqlTemp := fmt.Sprintf(`WITH RECURSIVE MenuHierarchy AS (
    SELECT id
    FROM %s
    WHERE id IN ? AND deleted_at IS NULL
    UNION ALL
    SELECT m.id
    FROM %s m
    INNER JOIN MenuHierarchy mh ON m.parent_id = mh.id
    WHERE m.deleted_at IS NULL
)
UPDATE %s
SET deleted_at = ?
WHERE id IN (SELECT id FROM MenuHierarchy)`, menu.TableName(), menu.TableName(), menu.TableName())
	return db.Exec(sqlTemp, rootIDs, time.Now()).Error
}

func deleteGeneratedMenuPolicies(db *gorm.DB, menuTargets []generatedMenuPolicyTarget) error {
	seen := make(map[string]struct{}, len(menuTargets))
	for i := range menuTargets {
		if menuTargets[i].Path == "" {
			continue
		}
		method := menuTargets[i].Method
		if method == "" {
			method = http.MethodGet
		}
		key := fmt.Sprintf("%s|%s|%s", menuTargets[i].Type.String(), menuTargets[i].Path, method)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := db.Where("ptype = ? AND v1 = ? AND v2 = ? AND v3 = ?",
			"p", menuTargets[i].Type.String(), menuTargets[i].Path, method).
			Delete(&models.CasbinRule{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Create 创建模型
// @Summary 创建模型
// @Description 创建模型
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param data body models.Model true "data"
// @Success 201 {object} models.Model
// @Router /admin/api/models [post]
// @Security Bearer
func (e *Model) Create(*gin.Context) {}

// Update 更新模型
// @Summary 更新模型
// Description 更新模型
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param model body models.Model true "model"
// @Success 200 {object} models.Model
// @Router /admin/api/models/{id} [put]
// @Security Bearer
func (e *Model) Update(*gin.Context) {}

// Delete 删除模型
// @Summary 删除模型
// @Description 删除模型
// @Tags model
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/models/{id} [delete]
// @Security Bearer
func (e *Model) Delete(*gin.Context) {}

// Get 获取模型
// @Summary 获取模型
// @Description 获取模型
// @Tags model
// @Param id path string true "id"
// @Success 200 {object} models.Model
// @Router /admin/api/models/{id} [get]
// @Security Bearer
func (e *Model) Get(*gin.Context) {}

// List 模型列表
// @Summary 模型列表
// @Description 模型列表
// @Tags model
// @Accept application/json
// @Produce application/json
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Param preloads query []string false "preloads"
// @Success 200 {object} response.Page{data=[]models.Model}
// @Router /admin/api/models [get]
// @Security Bearer
func (e *Model) List(*gin.Context) {}
