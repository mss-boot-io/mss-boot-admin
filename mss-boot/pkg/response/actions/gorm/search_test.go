package gorm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"gorm.io/gorm"
)

type searchRecord struct {
	ID   int64  `json:"id" gorm:"primaryKey"`
	Name string `json:"name"`
}

func (*searchRecord) TableName() string {
	return "search_records"
}

type searchRequest struct {
	actions.Pagination
}

func TestSearchAppliesPaginationWithoutCustomScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&searchRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	for _, record := range []searchRecord{{Name: "one"}, {Name: "two"}, {Name: "three"}} {
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	defer func() { gormdb.DB = previousDB }()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	action := NewSearch(
		WithModel(&searchRecord{}),
		WithSearch(&searchRequest{}),
	)
	router.GET("/records", action.Handler()...)

	request := httptest.NewRequest(http.MethodGet, "/records?current=2&pageSize=1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var body struct {
		Total    int64          `json:"total"`
		Current  int64          `json:"current"`
		PageSize int64          `json:"pageSize"`
		Data     []searchRecord `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Total != 3 || body.Current != 2 || body.PageSize != 1 {
		t.Fatalf("unexpected page metadata: %+v", body)
	}
	if len(body.Data) != 1 || body.Data[0].ID == 0 {
		t.Fatalf("pagination was not applied: %+v", body.Data)
	}
}

func TestSearchRequiresModelAndRequestConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/records", NewSearch().Handler()...)
	request := httptest.NewRequest(http.MethodGet, "/records", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", response.Code)
	}
}
