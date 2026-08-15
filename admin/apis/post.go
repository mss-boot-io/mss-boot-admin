package apis

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
	"gorm.io/gorm"
)

func init() {
	response.AppendController(newPostController())
}

func newPostController() *Post {
	return &Post{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.Post{}),
			controller.WithSearch(&dto.PostSearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithTreeField("Children"),
			controller.WithDepth(5),
			// Post.DataScope defines downstream row authority. Until delegated
			// grant-subset validation exists, changing that scope is root-only.
			controller.WithCreateHandlers(gin.HandlersChain{requireRootManagement}),
			controller.WithDeleteHandlers(gin.HandlersChain{requireRootManagement}),
			controller.WithBeforeCreate(preparePostCreate),
			controller.WithBeforeUpdate(preparePostUpdate),
			controller.WithBeforeDelete(validatePostDelete),
			controller.WithWriteErrorMapper(
				authorityHierarchyWriteErrorMapper("POST", "post"),
			),
		),
	}
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

func (e *Post) Create(c *gin.Context) {}

func (e *Post) Update(c *gin.Context) {}

func (e *Post) Delete(c *gin.Context) {}

func (e *Post) Get(c *gin.Context) {}
