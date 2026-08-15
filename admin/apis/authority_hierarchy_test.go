package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	responsegorm "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestDepartmentAPIProtectsIdentityAndHierarchy(t *testing.T) {
	router, db := authorityHierarchyAPITestRouter(t, "identity-department")

	result := authorityHierarchyRequest(
		t,
		router,
		http.MethodPost,
		"/departments",
		`{
			"id":"client-owned-department",
			"createdAt":"2000-01-01T00:00:00Z",
			"name":"Platform",
			"code":"PLATFORM",
			"status":"enabled",
			"children":[{
				"id":"injected-child",
				"name":"Injected",
				"code":"INJECTED",
				"status":"enabled"
			}]
		}`,
	)
	require.Equal(t, http.StatusCreated, result.Code, result.Body.String())
	var created models.Department
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.NotEqual(t, "client-owned-department", created.ID)
	require.False(t, created.CreatedAt.IsZero())
	require.True(t, created.CreatedAt.After(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)))
	require.Empty(t, created.Children)

	var injectedChildren int64
	require.NoError(t, db.Model(&models.Department{}).
		Where("id = ? OR parent_id = ?", "injected-child", created.ID).
		Count(&injectedChildren).Error)
	require.Zero(t, injectedChildren)

	root := authorityAPIDepartment("department-root", "")
	child := authorityAPIDepartment("department-child", root.ID)
	grandchild := authorityAPIDepartment("department-grandchild", child.ID)
	require.NoError(t, db.Create(root).Error)
	require.NoError(t, db.Create(child).Error)
	require.NoError(t, db.Create(grandchild).Error)

	result = authorityHierarchyRequest(
		t,
		router,
		http.MethodPut,
		"/departments/"+root.ID,
		`{"parentID":"department-grandchild"}`,
	)
	require.Equal(t, http.StatusConflict, result.Code, result.Body.String())
	require.Contains(t, result.Body.String(), `"errorCode":"DEPARTMENT_HIERARCHY_CYCLE"`)
	require.NotContains(t, result.Body.String(), "mss_boot_departments")
	require.NotContains(t, result.Body.String(), "SELECT")

	var persisted models.Department
	require.NoError(t, db.First(&persisted, "id = ?", root.ID).Error)
	require.Empty(t, persisted.ParentID)
}

func TestDepartmentAPIRejectsReferencedDeletesAndBatchBypass(t *testing.T) {
	router, db := authorityHierarchyAPITestRouter(t, "delete-department")

	parent := authorityAPIDepartment("department-parent", "")
	child := authorityAPIDepartment("department-child", parent.ID)
	userDepartment := authorityAPIDepartment("department-user-reference", "")
	scopeDepartment := authorityAPIDepartment("department-scope-reference", "")
	safeDepartment := authorityAPIDepartment("department-batch-safe", "")
	require.NoError(t, db.Create(parent).Error)
	require.NoError(t, db.Create(child).Error)
	require.NoError(t, db.Create(userDepartment).Error)
	require.NoError(t, db.Create(scopeDepartment).Error)
	require.NoError(t, db.Create(safeDepartment).Error)

	user := authorityAPIUser("department-reference-user")
	user.DepartmentID = userDepartment.ID
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).
		Omit(clause.Associations).
		Create(user).Error)
	scopedPost := authorityAPIPost("department-reference-post", "")
	scopedPost.DataScope = models.DataScopeCustomDept
	scopedPost.DeptIDS = scopeDepartment.ID
	scopedPost.DeptIDSArr = []string{scopeDepartment.ID}
	require.NoError(t, db.Create(scopedPost).Error)

	for _, target := range []string{parent.ID, userDepartment.ID, scopeDepartment.ID} {
		result := authorityHierarchyRequest(
			t,
			router,
			http.MethodDelete,
			"/departments/"+target,
			"",
		)
		require.Equal(t, http.StatusConflict, result.Code, result.Body.String())
		require.Contains(t, result.Body.String(), `"errorCode":"DEPARTMENT_IN_USE"`)
	}

	result := authorityHierarchyRequest(
		t,
		router,
		http.MethodDelete,
		"/departments/batch",
		`["department-batch-safe","department-scope-reference"]`,
	)
	require.Equal(t, http.StatusConflict, result.Code, result.Body.String())
	require.Contains(t, result.Body.String(), `"errorCode":"DEPARTMENT_IN_USE"`)

	var remaining int64
	require.NoError(t, db.Model(&models.Department{}).
		Where("id IN ?", []string{parent.ID, userDepartment.ID, scopeDepartment.ID, safeDepartment.ID}).
		Count(&remaining).Error)
	require.EqualValues(t, 4, remaining)
}

func TestPostAPIResolvesCustomScopeAndProtectsReferences(t *testing.T) {
	router, db := authorityHierarchyAPITestRouter(t, "post-authority")
	actorDepartment := authorityAPIDepartment("actor-department", "")
	selectedDepartment := authorityAPIDepartment("selected-department", "")
	require.NoError(t, db.Create(actorDepartment).Error)
	require.NoError(t, db.Create(selectedDepartment).Error)

	result := authorityHierarchyRequest(
		t,
		router,
		http.MethodPost,
		"/posts",
		`{
			"id":"client-owned-post",
			"name":"Regional operator",
			"code":"REGIONAL_OPERATOR",
			"status":"enabled",
			"dataScope":"customDept",
			"deptIDS":["selected-department"]
		}`,
	)
	require.Equal(t, http.StatusCreated, result.Code, result.Body.String())
	var created models.Post
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.NotEqual(t, "client-owned-post", created.ID)
	require.Equal(t, []string{selectedDepartment.ID}, created.DeptIDSArr)

	var persisted models.Post
	require.NoError(t, db.First(&persisted, "id = ?", created.ID).Error)
	require.Equal(t, selectedDepartment.ID, persisted.DeptIDS)
	require.NotEqual(t, actorDepartment.ID, persisted.DeptIDS)

	result = authorityHierarchyRequest(
		t,
		router,
		http.MethodPost,
		"/posts",
		`{
			"name":"Invalid scope",
			"code":"INVALID_SCOPE",
			"status":"enabled",
			"dataScope":"customDept",
			"deptIDS":["missing-department"]
		}`,
	)
	require.Equal(t, http.StatusUnprocessableEntity, result.Code, result.Body.String())
	require.Contains(t, result.Body.String(), `"errorCode":"POST_REFERENCE_INVALID"`)

	parent := authorityAPIPost("post-parent", "")
	child := authorityAPIPost("post-child", parent.ID)
	userPost := authorityAPIPost("post-user-reference", "")
	require.NoError(t, db.Create(parent).Error)
	require.NoError(t, db.Create(child).Error)
	require.NoError(t, db.Create(userPost).Error)
	user := authorityAPIUser("post-reference-user")
	user.PostID = userPost.ID
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).
		Omit(clause.Associations).
		Create(user).Error)

	for _, target := range []string{parent.ID, userPost.ID} {
		result = authorityHierarchyRequest(
			t,
			router,
			http.MethodDelete,
			"/posts/"+target,
			"",
		)
		require.Equal(t, http.StatusConflict, result.Code, result.Body.String())
		require.Contains(t, result.Body.String(), `"errorCode":"POST_IN_USE"`)
	}
}

func authorityHierarchyAPITestRouter(t *testing.T, identityKey string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousAuthHandler := response.AuthHandler
	previousDB := gormdb.DB
	previousCleaner := responsegorm.CleanCacheFromTag
	config.Cfg.Auth.IdentityKey = identityKey
	response.AuthHandler = func(c *gin.Context) { c.Next() }
	responsegorm.CleanCacheFromTag = nil
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		response.AuthHandler = previousAuthHandler
		gormdb.DB = previousDB
		responsegorm.CleanCacheFromTag = previousCleaner
	})

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) +
		"?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(
		t,
		db.AutoMigrate(
			&models.Department{},
			&models.Post{},
			&models.Role{},
			&models.User{},
		),
	)
	gormdb.DB = db

	rootPrincipal := &models.User{UserLogin: models.UserLogin{
		RoleID:       "root-role",
		Role:         &models.Role{Root: true},
		DepartmentID: "actor-department",
		Status:       enum.Enabled,
	}}
	rootPrincipal.ID = "root-user"

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, rootPrincipal)
		c.Next()
	})
	departmentController := newDepartmentController()
	postController := newPostController()
	router.POST("/departments", departmentController.GetAction(response.Control).Handler()...)
	router.PUT("/departments/:id", departmentController.GetAction(response.Control).Handler()...)
	router.DELETE("/departments/:id", departmentController.GetAction(response.Delete).Handler()...)
	router.POST("/posts", postController.GetAction(response.Control).Handler()...)
	router.PUT("/posts/:id", postController.GetAction(response.Control).Handler()...)
	router.DELETE("/posts/:id", postController.GetAction(response.Delete).Handler()...)
	return router, db
}

func authorityHierarchyRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)
	return result
}

func authorityAPIDepartment(id, parentID string) *models.Department {
	department := &models.Department{
		ParentID: parentID,
		Name:     id,
		Code:     id,
		Status:   enum.Enabled,
	}
	department.ID = id
	return department
}

func authorityAPIPost(id, parentID string) *models.Post {
	post := &models.Post{
		ParentID:  parentID,
		Name:      id,
		Code:      id,
		Status:    enum.Enabled,
		DataScope: models.DataScopeSelf,
	}
	post.ID = id
	return post
}

func authorityAPIUser(id string) *models.User {
	user := &models.User{UserLogin: models.UserLogin{
		Username: id,
		Status:   enum.Enabled,
	}}
	user.ID = id
	return user
}
