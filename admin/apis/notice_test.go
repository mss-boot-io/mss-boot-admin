package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNoticeUnreadReturnsNewestUnreadNoticesBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:notice-unread?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Notice{}); err != nil {
		t.Fatalf("migrate notices: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	previousVerifyHandler := response.VerifyHandler
	t.Cleanup(func() { response.VerifyHandler = previousVerifyHandler })

	userID := "current-user"
	verifier := &models.User{}
	verifier.ID = userID
	response.VerifyHandler = func(*gin.Context) security.Verifier { return verifier }

	base := time.Date(2026, time.August, 6, 8, 0, 0, 0, time.UTC)
	for index := 0; index < unreadNoticeLimit+2; index++ {
		createdAt := base.Add(time.Duration(index) * time.Minute)
		if index == unreadNoticeLimit || index == unreadNoticeLimit+1 {
			createdAt = base.Add(200 * time.Minute)
		}
		notice := &models.Notice{
			UserID: userID,
			Title:  fmt.Sprintf("unread-%03d", index),
			Read:   false,
		}
		notice.ID = fmt.Sprintf("notice-%03d", index)
		notice.CreatedAt = createdAt
		notice.UpdatedAt = createdAt
		if err := db.Create(notice).Error; err != nil {
			t.Fatalf("create unread notice %d: %v", index, err)
		}
	}

	for _, notice := range []*models.Notice{
		{UserID: userID, Title: "read", Read: true},
		{UserID: "another-user", Title: "another user", Read: false},
		{UserID: userID, Title: "deleted", Read: false},
	} {
		notice.ID = "excluded-" + notice.Title
		notice.CreatedAt = base.Add(300 * time.Minute)
		notice.UpdatedAt = notice.CreatedAt
		if err := db.Create(notice).Error; err != nil {
			t.Fatalf("create excluded notice %q: %v", notice.Title, err)
		}
		if notice.Title == "deleted" {
			if err := db.Delete(notice).Error; err != nil {
				t.Fatalf("soft delete notice: %v", err)
			}
		}
	}

	router := gin.New()
	router.GET("/admin/api/notice/unread", (&Notice{}).Unread)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/notice/unread", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var notices []models.Notice
	if err := json.Unmarshal(recorder.Body.Bytes(), &notices); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(notices) != unreadNoticeLimit {
		t.Fatalf("notice count = %d, want %d", len(notices), unreadNoticeLimit)
	}
	if notices[0].ID != "notice-101" || notices[1].ID != "notice-100" {
		t.Fatalf("stable newest order = [%s, %s], want [notice-101, notice-100]", notices[0].ID, notices[1].ID)
	}
	if notices[len(notices)-1].ID != "notice-002" {
		t.Fatalf("oldest bounded notice = %s, want notice-002", notices[len(notices)-1].ID)
	}
	for _, notice := range notices {
		if notice.UserID != userID || notice.Read {
			t.Fatalf("unexpected notice returned: %+v", notice)
		}
	}
}
