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
	"gorm.io/gorm/schema"
)

func init() {
	e := &Post{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(&models.Post{}),
			controller.WithSearch(&dto.PostSearch{}),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithTreeField("Children"),
			controller.WithDepth(5),
			controller.WithBeforeCreate(beforeCreate),
			controller.WithBeforeUpdate(beforeUpdate),
		),
	}
	response.AppendController(e)
}

func beforeCreate(c *gin.Context, db *gorm.DB, m schema.Tabler) error {
	post, ok := m.(*models.Post)
	if !ok {
		return nil
	}
	if post.DataScope == models.DataScopeCustomDept {
		post.DeptIDSArr = getUserDeptIDS(c, db)
	}
	return nil
}

func beforeUpdate(c *gin.Context, db *gorm.DB, m schema.Tabler) error {
	post, ok := m.(*models.Post)
	if !ok {
		return nil
	}
	if post.DataScope == models.DataScopeCustomDept {
		post.DeptIDSArr = getUserDeptIDS(c, db)
	} else {
		post.DeptIDSArr = nil
	}
	return nil
}

func getUserDeptIDS(c *gin.Context, db *gorm.DB) []string {
	verify := response.VerifyHandler(c)
	if verify == nil {
		return nil
	}

	userID := verify.GetUserID()
	if userID == "" {
		return nil
	}

	user := &models.User{}
	if err := db.First(user, "id = ?", userID).Error; err != nil {
		return nil
	}

	if user.DepartmentID == "" {
		return nil
	}

	deptIDS := []string{user.DepartmentID}
	getChildDeptIDS(db, user.DepartmentID, &deptIDS)

	return deptIDS
}

func getChildDeptIDS(db *gorm.DB, parentID string, deptIDS *[]string) {
	var children []models.Department
	db.Where("parent_id = ?", parentID).Find(&children)
	for _, child := range children {
		*deptIDS = append(*deptIDS, child.ID)
		getChildDeptIDS(db, child.ID, deptIDS)
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
