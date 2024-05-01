package apis

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"github.com/mss-boot-io/mss-boot/pkg/search/gorms"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/28 22:44:14
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/28 22:44:14
 */

func init() {
	e := &Department{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.Department{}),
			controller.WithSearch(&dto.DepartmentSearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(center.Default.Scope),
			controller.WithTreeField("Children"),
			controller.WithDepth(5),
		),
	}
	response.AppendController(e)
}

type Department struct {
	*controller.Simple
}

func (e *Department) GetAction(key string) response.Action {
	if key == response.Search {
		return nil
	}
	return e.Simple.GetAction(key)
}

func (e *Department) Other(r *gin.RouterGroup) {
	r.GET("/departments", e.List)
}

// List 部门列表
// @Summary 部门列表
// @Description 部门列表
// @Tags department
// @Accept application/json
// @Produce application/json
// @Param name query string false "部门名称"
// @Param parentID query string false "父级部门ID"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param pageSize query int false "每页条数"
// @Success 200 {object} response.Page{data=[]models.Department}
// @Router /admin/api/departments [get]
// @Security Bearer
func (e *Department) List(c *gin.Context) {
	api := response.Make(c)
	req := &dto.DepartmentSearch{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	items := make([]models.Department, 0)
	m := &models.Department{}
	query := center.Default.GetDB(c, m).
		Model(m).
		Scopes(center.Default.Scope(c, m)).
		Preload("Children.Children.Children.Children.Children").
		Scopes(
			gorms.MakeCondition(req),
			gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())),
		).
		Where(fmt.Sprintf("%s.parent_id = ?", m.TableName()), "")

	var count int64
	if err := query.Limit(-1).Offset(-1).
		Count(&count).Error; err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	if err := query.Find(&items).Error; err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	api.PageOK(items, count, req.GetPage(), req.GetPageSize())
}

// Create 创建部门
// @Summary 创建部门
// @Description 创建部门
// @Tags department
// @Accept application/json
// @Produce application/json
// @Param data body models.Department true "data"
// @Success 201 {object} models.Department
// @Router /admin/api/departments [post]
// @Security Bearer
func (e *Department) Create(c *gin.Context) {}

// Update 更新部门
// @Summary 更新部门
// @Description 更新部门
// @Tags department
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param data body models.Department true "data"
// @Success 200 {object} models.Department
// @Router /admin/api/departments/{id} [put]
// @Security Bearer
func (e *Department) Update(c *gin.Context) {}

// Delete 删除部门
// @Summary 删除部门
// @Description 删除部门
// @Tags department
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/departments/{id} [delete]
// @Security Bearer
func (e *Department) Delete(c *gin.Context) {}

// Get 获取部门
// @Summary 获取部门
// @Description 获取部门
// @Tags department
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 200 {object} models.Department
// @Router /admin/api/departments/{id} [get]
// @Security Bearer
func (e *Department) Get(c *gin.Context) {}
