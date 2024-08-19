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
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/28 22:44:21
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/28 22:44:21
 */

func init() {
	e := &Post{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.Post{}),
			controller.WithSearch(&dto.PostSearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithScope(center.Default.Scope),
			controller.WithTreeField("Children"),
			controller.WithDepth(5),
		),
	}
	response.AppendController(e)
}

type Post struct {
	*controller.Simple
}

func (e *Post) GetAction(key string) response.Action {
	if key == response.Search {
		return nil
	}
	return e.Simple.GetAction(key)

}

func (e *Post) Other(r *gin.RouterGroup) {
	r.GET("/posts", response.AuthHandler, e.List)
}

// List 岗位列表
// @Summary 岗位列表
// @Description 岗位列表
// @Tags post
// @Accept application/json
// @Produce application/json
// @Param name query string false "岗位名称"
// @Param parentID query string false "父级岗位ID"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param pageSize query int false "每页条数"
// @Success 200 {object} response.Page{data=[]models.Post}
// @Router /admin/api/posts [get]
// @Security Bearer
func (e *Post) List(c *gin.Context) {
	api := response.Make(c)
	req := &dto.PostSearch{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	items := make([]models.Post, 0)
	m := &models.Post{}
	query := center.Default.GetDB(c, m).
		Model(m).
		Scopes(center.Default.Scope(c, m)).
		Preload("Children").
		Scopes(
			gorms.MakeCondition(req),
			gorms.Paginate(int(req.GetPageSize()), int(req.GetPage())),
		).Where(fmt.Sprintf("%s.parent_id = ?", m.TableName()), "")

	var count int64
	if err := query.Scopes(func(db *gorm.DB) *gorm.DB {
		return db.Limit(-1).Offset(-1)
	}).
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

// Create 创建岗位
// @Summary 创建岗位
// @Description 创建岗位
// @Tags post
// @Accept application/json
// @Produce application/json
// @Param data body models.Post true "data"
// @Success 201 {object} models.Post
// @Router /admin/api/posts [post]
// @Security Bearer
func (e *Post) Create(c *gin.Context) {}

// Update 更新岗位
// @Summary 更新岗位
// @Description 更新岗位
// @Tags post
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Param data body models.Post true "data"
// @Success 200 {object} models.Post
// @Router /admin/api/posts/{id} [put]
// @Security Bearer
func (e *Post) Update(c *gin.Context) {}

// Delete 删除岗位
// @Summary 删除岗位
// @Description 删除岗位
// @Tags post
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/posts/{id} [delete]
// @Security Bearer
func (e *Post) Delete(c *gin.Context) {}

// Get 获取岗位
// @Summary 获取岗位
// @Description 获取岗位
// @Tags post
// @Accept application/json
// @Produce application/json
// @Param id path string true "id"
// @Success 200 {object} models.Post
// @Router /admin/api/posts/{id} [get]
// @Security Bearer
func (e *Post) Get(c *gin.Context) {}
