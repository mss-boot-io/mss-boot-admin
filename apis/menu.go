package apis

import (
	"net/http"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/search/gorms"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
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
			V2:    models.MenuAccessType.String(),
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
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Error("get menu tree error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	canList := make([]*models.Menu, 0)
	// check select menu
	for i := range list {
		ok, err := gormdb.Enforcer.Enforce(
			verify.GetRoleID(), models.MenuAccessType.String(), list[i].Path)
		if err != nil {
			api.AddError(err).Log.Error("get menu tree error", "err", err)
			api.Err(http.StatusInternalServerError)
			return
		}
		if ok {
			canList = append(canList, list[i])
		}
	}
	api.OK(models.GetMenuTree(canList))
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
	err := gormdb.DB.WithContext(ctx).Find(&list).Error
	if err != nil {
		api.Log.Error("get menu tree error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	api.OK(models.CompleteName(models.GetMenuTree(list)))
}

// Create 创建菜单
// @Summary 创建菜单
// @Description 创建菜单
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param data body models.Menu true "data"
// @Success 201
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
// @Success 200
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

// List 菜单列表数据
// @Summary 菜单列表数据
// @Description 菜单列表数据
// @Tags menu
// @Accept  application/json
// @Product application/json
// @Param name query string false "name"
// @Param status query string false "status"
// @Param parentID query string false "parentID"
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
	query := gormdb.DB.Model(&models.Menu{}).WithContext(ctx).
		Where("parent_id = ?", req.ParentID).
		Order("sort desc").Scopes(
		gorms.MakeCondition(req),
		gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())),
	)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}
	var count int64
	if err := query.Limit(-1).Offset(-1).Count(&count).Error; err != nil {
		api.AddError(err).Log.Error("get menu list error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}

	if err := query.Preload("Children").Find(&list).Error; err != nil {
		api.AddError(err).Log.Error("get menu list error", "err", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	list = models.CompleteName(list)
	api.PageOK(list, count, req.GetPage(), req.GetPageSize())
}
