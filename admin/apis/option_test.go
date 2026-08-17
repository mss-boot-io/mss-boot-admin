package apis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOptionAPITest(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQLite handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.Option{}, &models.OptionVersion{}, &models.OptionUsage{}); err != nil {
		t.Fatalf("migrate option API schema: %v", err)
	}
	previousDB := gormdb.DB
	previousTenant := center.GetTenant()
	previousCache := center.GetCache()
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	center.SetTenant(&center.SingleTenant{})
	center.SetCache(nil)
	config.Cfg.Auth.IdentityKey = "option-api-test-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		center.SetTenant(previousTenant)
		center.SetCache(previousCache)
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		_ = sqlDB.Close()
	})

	principal := &models.User{}
	principal.ID = "actor-id"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
		ctx.Next()
	})
	controller := newOptionController()
	router.GET("/options", controller.ManagedList)
	router.GET("/options/:id", controller.ManagedGet)
	router.POST("/options", controller.ManagedCreate)
	router.PUT("/options/:id", controller.ManagedUpdate)
	router.DELETE("/options/:id", controller.ManagedDelete)
	return db, router
}

func optionAPIRequest(
	router *gin.Engine,
	method string,
	path string,
	body string,
	ifMatch string,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestOptionManagedCRUDOwnsIdentitySnapshotsAndRejectsStaleWriters(t *testing.T) {
	db, router := setupOptionAPITest(t)
	createdResponse := optionAPIRequest(router, http.MethodPost, "/options", `{
		"id":"client-owned",
		"builtIn":true,
		"version":99,
		"category":" custom ",
		"displayName":" Priority ",
		"description":" Before ",
		"name":" priority ",
		"status":"enabled",
		"items":[{"id":"client-item","key":"high","label":"High","value":"high","extra":{"rank":1}}]
	}`, "")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create response = %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created models.Option
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created option: %v", err)
	}
	if created.ID == "" || created.ID == "client-owned" || created.BuiltIn || created.Version != 1 {
		t.Fatalf("server-owned created option = %#v", created)
	}
	if created.Category != "custom" || created.Name != "priority" || created.Items == nil || len(*created.Items) != 1 {
		t.Fatalf("created option = %#v", created)
	}
	if (*created.Items)[0].ID == "" || (*created.Items)[0].ID == "client-item" {
		t.Fatalf("created item ID = %q, want server-owned", (*created.Items)[0].ID)
	}
	if createdResponse.Header().Get("ETag") != optionETag(created.ID, 1) {
		t.Fatalf("create ETag = %q", createdResponse.Header().Get("ETag"))
	}

	detail := optionAPIRequest(router, http.MethodGet, "/options/"+created.ID, "", "")
	if detail.Code != http.StatusOK || detail.Header().Get("ETag") != optionETag(created.ID, 1) {
		t.Fatalf("detail response = %d, ETag %q: %s", detail.Code, detail.Header().Get("ETag"), detail.Body.String())
	}

	updateBody := fmt.Sprintf(`{
		"displayName":"Priority values",
		"description":"After",
		"items":[
			{"id":%q,"key":"high","label":"High priority","value":"high","extra":{"rank":1}},
			{"id":"foreign-item","key":"low","label":"Low","value":"low"}
		]
	}`, (*created.Items)[0].ID)
	updatedResponse := optionAPIRequest(
		router,
		http.MethodPut,
		"/options/"+created.ID,
		updateBody,
		optionETag(created.ID, 1),
	)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update response = %d: %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	var updated models.Option
	if err := json.Unmarshal(updatedResponse.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated option: %v", err)
	}
	if updated.Version != 2 || updated.DisplayName != "Priority values" || len(*updated.Items) != 2 {
		t.Fatalf("updated option = %#v", updated)
	}
	if (*updated.Items)[0].ID != (*created.Items)[0].ID || (*updated.Items)[1].ID == "foreign-item" {
		t.Fatalf("updated item ownership = %#v", updated.Items)
	}

	var snapshot models.OptionVersion
	if err := db.Where("option_id = ? AND version = ?", created.ID, 1).First(&snapshot).Error; err != nil {
		t.Fatalf("load option snapshot: %v", err)
	}
	if snapshot.Category != "custom" || snapshot.Name != "priority" || snapshot.DisplayName != "Priority" ||
		snapshot.Description != "Before" || snapshot.ChangedBy != "actor-id" {
		t.Fatalf("full option snapshot = %#v", snapshot)
	}

	staleResponse := optionAPIRequest(
		router,
		http.MethodPut,
		"/options/"+created.ID,
		`{"remark":"stale draft"}`,
		optionETag(created.ID, 1),
	)
	if staleResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale response = %d: %s", staleResponse.Code, staleResponse.Body.String())
	}
	var conflict dto.OptionRevisionConflictResponse
	if err := json.Unmarshal(staleResponse.Body.Bytes(), &conflict); err != nil {
		t.Fatalf("decode stale conflict: %v", err)
	}
	if conflict.ErrorCode != "OPTION_REVISION_CONFLICT" || conflict.Data.Current == nil || conflict.Data.Current.Version != 2 {
		t.Fatalf("stale conflict = %#v", conflict)
	}

	deleteResponse := optionAPIRequest(
		router,
		http.MethodDelete,
		"/options/"+created.ID,
		"",
		optionETag(created.ID, 2),
	)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete response = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var remaining int64
	if err := db.Model(&models.Option{}).Where("id = ?", created.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("remaining options = %d, err = %v", remaining, err)
	}
	var snapshots int64
	if err := db.Model(&models.OptionVersion{}).Where("option_id = ?", created.ID).Count(&snapshots).Error; err != nil || snapshots != 2 {
		t.Fatalf("snapshot count = %d, err = %v", snapshots, err)
	}
}

func TestOptionManagedMutationRequiresCanonicalIfMatch(t *testing.T) {
	_, router := setupOptionAPITest(t)
	createdResponse := optionAPIRequest(router, http.MethodPost, "/options", `{
		"name":"canonical-precondition",
		"status":"enabled",
		"items":[{"key":"yes","label":"Yes","value":"yes"}]
	}`, "")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create response = %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created models.Option
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	missingRevision := optionAPIRequest(
		router,
		http.MethodPut,
		"/options/"+created.ID,
		`{"remark":"write without revision"}`,
		"",
	)
	if missingRevision.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing revision update = %d: %s", missingRevision.Code, missingRevision.Body.String())
	}
	bodyRevisionOnly := optionAPIRequest(
		router,
		http.MethodPut,
		"/options/"+created.ID,
		`{"expectedVersion":1,"version":1,"remark":"body revision is not a precondition"}`,
		"",
	)
	if bodyRevisionOnly.Code != http.StatusPreconditionRequired {
		t.Fatalf("body-only revision update = %d: %s", bodyRevisionOnly.Code, bodyRevisionOnly.Body.String())
	}
}

func TestOptionManagedSummaryIsBoundedAndReadsFailClosed(t *testing.T) {
	db, router := setupOptionAPITest(t)
	items := models.OptionItems{{ID: "one", Key: "active", Label: "Active", Value: "enabled"}}
	option := &models.Option{
		Category: "system", Name: "status", Description: "detail only", Status: enum.Enabled, Version: 1, Items: &items,
	}
	option.ID = "option-summary"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(option).Error; err != nil {
		t.Fatalf("seed option: %v", err)
	}

	summary := optionAPIRequest(router, http.MethodGet, "/options?current=1&pageSize=20", "", "")
	if summary.Code != http.StatusOK {
		t.Fatalf("summary response = %d: %s", summary.Code, summary.Body.String())
	}
	if bytes.Contains(summary.Body.Bytes(), []byte(`"items"`)) || bytes.Contains(summary.Body.Bytes(), []byte(`"description"`)) {
		t.Fatalf("summary disclosed detail payload: %s", summary.Body.String())
	}
	wildcard := optionAPIRequest(router, http.MethodGet, "/options?name=stat_&pageSize=20", "", "")
	var page struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(wildcard.Body.Bytes(), &page); err != nil || page.Total != 0 {
		t.Fatalf("escaped wildcard page = %#v, err = %v, body = %s", page, err, wildcard.Body.String())
	}
	unbounded := optionAPIRequest(router, http.MethodGet, "/options?pageSize=101", "", "")
	if unbounded.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unbounded response = %d: %s", unbounded.Code, unbounded.Body.String())
	}

	corrupt := `[
		{"id":"one","key":"duplicate","label":"One","value":"1"},
		{"id":"two","key":"duplicate","label":"Two","value":"2"}
	]`
	if err := db.Exec("UPDATE mss_boot_options SET items = ? WHERE id = ?", corrupt, option.ID).Error; err != nil {
		t.Fatalf("corrupt option: %v", err)
	}
	detail := optionAPIRequest(router, http.MethodGet, "/options/"+option.ID, "", "")
	if detail.Code != http.StatusServiceUnavailable {
		t.Fatalf("corrupt detail response = %d: %s", detail.Code, detail.Body.String())
	}
	var apiError response.Response
	if err := json.Unmarshal(detail.Body.Bytes(), &apiError); err != nil || apiError.ErrorCode != "OPTION_DATA_INVALID" {
		t.Fatalf("corrupt detail error = %#v, err = %v", apiError, err)
	}
}

func TestOptionManagedWriteRejectsChunkedOversizedPayload(t *testing.T) {
	_, router := setupOptionAPITest(t)
	payload := `{"name":"oversized","remark":"` + strings.Repeat("x", maxOptionWriteRequestSize) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/options", strings.NewReader(payload))
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized response = %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestOptionRoutesDisableGenericCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	previousAuth := response.AuthHandler
	response.AuthHandler = func(ctx *gin.Context) { ctx.Next() }
	t.Cleanup(func() { response.AuthHandler = previousAuth })
	controller := newOptionController()
	controller.Other(engine.Group("/admin/api"))
	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		"GET /admin/api/options",
		"GET /admin/api/options/:id",
		"POST /admin/api/options",
		"PUT /admin/api/options/:id",
		"DELETE /admin/api/options/:id",
	} {
		if !routes[expected] {
			t.Fatalf("expected option route %q: %#v", expected, routes)
		}
	}
	for _, action := range []string{response.Get, response.Search, response.Control, response.Delete} {
		if controller.GetAction(action) != nil {
			t.Fatalf("generic option action %q remains enabled", action)
		}
	}
}
