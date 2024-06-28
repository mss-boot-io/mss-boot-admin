package apis

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/search/gorms"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/15 13:41:22
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/15 13:41:22
 */

func init() {
	e := &Menu{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Menu)),
			controller.WithSearch(new(dto.RoleSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(center.Default.Scope),
		),
	}
	response.AppendController(e)
}

type Menu struct {
	*controller.Simple
}

// GetAction get action
func (e *Menu) GetAction(key string) response.Action {
	if key == response.Search {
		return nil
	}
	return e.Simple.GetAction(key)
}

func (e *Menu) Other(r *gin.RouterGroup) {
	r.GET("/menu/tree", middleware.Auth.MiddlewareFunc(), e.Tree)
	r.GET("/menu/authorize", middleware.Auth.MiddlewareFunc(), e.GetAuthorize)
	r.PUT("/menu/authorize/:roleID", middleware.Auth.MiddlewareFunc(), e.UpdateAuthorize)
	r.GET("/menu/api/:id", middleware.Auth.MiddlewareFunc(), e.GetAPI)
	r.POST("/menu/bind-api", middleware.Auth.MiddlewareFunc(), e.BindAPI)
	r.GET("/menus", middleware.Auth.MiddlewareFunc(), e.List)
}

// UpdateAuthorize 更新菜单权限
// @Summary 更新菜单权限
// @Description 更新菜单权限
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body dto.UpdateAuthorizeRequest true "data"
// @Success 200
// @Router /admin/api/menu/authorize/{id} [put]
// @Security Bearer
func (e *Menu) UpdateAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.UpdateAuthorizeRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	// todo check roleID
	// todo check menu keys

	// todo commit transaction

	// delete all policy for role
	err := gormdb.DB.Where(&models.CasbinRule{
		PType: "p",
		V0:    req.RoleID,
	}).Delete(&models.CasbinRule{}).Error
	if err != nil {
		api.AddError(err).Log.Error("delete role error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = gormdb.Enforcer.LoadPolicy()
	}()
	if err != nil {
		api.AddError(err).Log.Error("delete role error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	rules := make([]*models.CasbinRule, len(req.Keys))
	for i := range req.Keys {
		rules[i] = &models.CasbinRule{
			PType: "p",
			V0:    req.RoleID,
			V1:    req.Keys[i],
			V2:    pkg.MenuAccessType.String(),
		}
	}
	if err = gormdb.DB.Create(&rules).Error; err != nil {
		api.AddError(err).Log.Error("create casbin rule error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// GetAuthorize 获取菜单权限
// @Summary 获取菜单权限
// @Description 获取菜单权限
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Success 200 {object} []models.Menu{children=[]models.Menu}
// @Router /admin/api/menu/authorize [get]
// @Security Bearer
func (e *Menu) GetAuthorize(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	list := make([]*models.Menu, 0)
	err := center.Default.GetDB(ctx, &models.Menu{}).
		Where("type = ? OR type = ?", pkg.MenuAccessType, pkg.DirectoryAccessType).
		Order("sort desc").
		Find(&list).Error
	if err != nil {
		api.Log.Error("get menu tree error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	canList := make([]*models.Menu, 0)
	// check select menu
	for i := range list {
		if list[i].Type == pkg.DirectoryAccessType {
			canList = append(canList, list[i])
			continue
		}
		ok, err := gormdb.Enforcer.Enforce(
			verify.GetRoleID(), pkg.MenuAccessType.String(), list[i].Path, list[i].Method)
		if err != nil {
			api.AddError(err).Log.Error("get menu tree error", "err", err)
			api.Err(http.StatusInternalServerError)
			return
		}
		if ok || verify.Root() {
			canList = append(canList, list[i])
		}
	}
	result := make([]*models.Menu, 0)
	for _, m := range pkg.BuildTree(models.MenuTransferToTreeSlice(canList), "") {
		menu := m.(*models.Menu)
		if len(menu.Children) == 0 && menu.Type == pkg.DirectoryAccessType {
			continue
		}
		result = append(result, menu)
	}
	api.OK(result)
}

// Tree 获取菜单树
// @Summary 获取菜单树
// @Description 获取菜单树
// @Tags menu
// @Success 200 {object} []models.Menu{children=[]models.Menu}
// @Router /admin/api/menu/tree [get]
// @Security Bearer
func (e *Menu) Tree(ctx *gin.Context) {
	api := response.Make(ctx)
	list := make([]*models.Menu, 0)
	err := center.Default.GetDB(ctx, &models.Menu{}).WithContext(ctx).
		Where("type <> ?", pkg.APIAccessType).
		Find(&list).Error
	if err != nil {
		api.AddError(err).Log.Error("get menu tree error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(models.CompleteName(
		models.TreeTransferToMenuSlice(
			pkg.BuildTree(
				models.MenuTransferToTreeSlice(list), ""))))
}

// GetAPI 获取菜单下的接口
// @Summary 获取菜单下的接口
// @Description 获取菜单下的接口
// @Tags menu
// @Param id path string true "id"
// @Success 200 {object} []models.Menu
// @Router /admin/api/menu/api/{id} [get]
// @Security Bearer
func (e *Menu) GetAPI(ctx *gin.Context) {
	api := response.Make(ctx)
	id := ctx.Param("id")
	m := &models.Menu{}
	err := center.Default.GetDB(ctx, &models.Menu{}).Model(&models.Menu{}).
		Where("id = ?", id).First(m).Error
	if err != nil {
		api.AddError(err).Log.Error("get menu error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	list := make([]*models.Menu, 0)
	err = center.Default.GetDB(ctx, &models.Menu{}).Where(&models.Menu{
		Type:     pkg.APIAccessType,
		ParentID: m.ID,
	}).Find(&list).Error
	if err != nil {
		api.AddError(err).Log.Error("get menu error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(list)
}

// BindAPI 绑定菜单下的接口
// @Summary 绑定菜单下的接口
// @Description 绑定菜单下的接口
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param data body dto.MenuBindAPIRequest true "data"
// @Success 200
// @Router /admin/api/menu/bind-api [post]
// @Security Bearer
func (e *Menu) BindAPI(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.MenuBindAPIRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	menu := &models.Menu{}
	err := center.Default.GetDB(ctx, &models.Menu{}).Model(menu).
		Where("id = ?", req.MenuID).
		First(menu).Error
	if err != nil {
		api.AddError(err).Log.Error("get menu error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	apis := make([]*models.API, len(req.Paths))
	for i := range req.Paths {
		arr := strings.Split(req.Paths[i], "---")
		if len(arr) > 1 {
			a := &models.API{}
			err = gormdb.DB.Model(a).
				Where("method = ?", arr[0]).
				Where("path = ?", arr[1]).
				First(a).Error
			if err != nil {
				api.AddError(err).Log.Error("get api error", "err", err)
				api.Err(http.StatusInternalServerError)
				return
			}
			apis[i] = a
		}
	}
	menuApis := make([]*models.Menu, len(apis))
	for i := range apis {
		menuApis[i] = &models.Menu{
			ParentID: menu.ID,
			Name:     apis[i].Name,
			Path:     apis[i].Path,
			Method:   apis[i].Method,
			Type:     pkg.APIAccessType,
		}
	}

	err = center.Default.GetDB(ctx, &models.Menu{}).Transaction(func(tx *gorm.DB) error {
		err = tx.Where(&models.Menu{
			ParentID: menu.ID,
			Type:     pkg.APIAccessType,
		}).Unscoped().Delete(&models.Menu{}).Error
		if err != nil {
			return err
		}
		return tx.Create(&menuApis).Error
	})
	if err != nil {
		api.AddError(err).Log.Error("create menu error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

// List 菜单列表数据
// @Summary 菜单列表数据
// @Description 菜单列表数据
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param name query string false "name"
// @Param status query string false "status"
// @Param show query bool false "show"
// @Param parentID query string false "parentID"
// @Param type query []string false "type"
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.Menu}
// @Router /admin/api/menus [get]
// @Security Bearer
func (*Menu) List(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.MenuSearch{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	list := make([]*models.Menu, 0)
	query := center.Default.GetDB(ctx, &models.Menu{}).Model(&models.Menu{}).WithContext(ctx).
		Where("parent_id = ?", req.ParentID).
		Order("sort desc").Scopes(
		gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())),
	)

	types := []pkg.AccessType{
		pkg.MenuAccessType,
		pkg.ComponentAccessType,
		pkg.DirectoryAccessType,
	}
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if len(req.Type) > 0 {
		types = make([]pkg.AccessType, len(req.Type))
		for i := range req.Type {
			types[i] = pkg.AccessType(req.Type[i])
		}
	}
	query = query.Where("type in ?", types)
	if req.Show {
		query = query.Where("hide_in_menu = ?", 0)
	}
	var count int64
	if err := query.Limit(-1).Offset(-1).Count(&count).Error; err != nil {
		api.AddError(err).Log.Error("get menu list error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}

	if err := query.
		Preload("Children", "type IN ?", types).
		Preload("Children.Children", "type IN ?", types).
		Preload("Children.Children.Children", "type IN ?", types).
		Find(&list).Error; err != nil {
		api.AddError(err).Log.Error("get menu list error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	list = models.CompleteName(list)
	api.PageOK(list, count, req.GetPage(), req.GetPageSize())
}

// Create 创建菜单
// @Summary 创建菜单
// @Description 创建菜单
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param data body models.Menu true "data"
// @Success 201 {object} models.Menu
// @Router /admin/api/menus [post]
// @Security Bearer
func (*Menu) Create(*gin.Context) {}

// Update 更新菜单
// @Summary 更新菜单
// @Description 更新菜单
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Menu true "data"
// @Success 200 {object} models.Menu
// @Router /admin/api/menus/{id} [put]
// @Security Bearer
func (*Menu) Update(*gin.Context) {}

// Get 获取菜单
// @Summary 获取菜单
// @Description 获取菜单
// @Tags menu
// @Param id path string true "id"
// @Param preloads query []string false "preloads"
// @Success 200 {object} models.Menu
// @Router /admin/api/menus/{id} [get]
// @Security Bearer
func (*Menu) Get(*gin.Context) {}

// Delete 删除菜单
// @Summary 删除菜单
// @Description 删除菜单
// @Tags menu
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/menus/{id} [delete]
// @Security Bearer
func (*Menu) Delete(*gin.Context) {}
