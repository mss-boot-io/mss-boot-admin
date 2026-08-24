package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPresentationAuditNeverPersistsEditableValuesSubjectsOrIdempotencyKeys(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AuditLog{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = "presentation-audit-test-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		_ = sqlDB.Close()
	})

	principal := &models.User{UserLogin: models.UserLogin{Username: "presentation-auditor"}}
	principal.ID = "presentation-auditor-id"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(config.Cfg.Auth.IdentityKey, principal)
		ctx.Next()
	})
	router.Use(AuditLogMiddleware())
	router.POST("/admin/api/presentation-profiles", func(ctx *gin.Context) {
		ctx.Status(http.StatusForbidden)
	})
	router.POST("/admin/api/presentation-profiles/validate", func(ctx *gin.Context) {
		ctx.Status(http.StatusForbidden)
	})
	router.PUT("/admin/api/presentation-profiles/:id/draft", func(ctx *gin.Context) {
		ctx.Status(http.StatusForbidden)
	})
	router.POST("/admin/api/presentation-profiles/:id/publish", func(ctx *gin.Context) {
		SetPresentationAuditMetadata(ctx, PresentationAuditMetadata{
			AggregateID:        ctx.Param("id"),
			PageKey:            "orders.list",
			Scope:              "role",
			SubjectPresent:     true,
			SubjectFingerprint: "sha256:subject-fingerprint-only",
			Transition:         "publish",
			Outcome:            "success",
			AggregateVersion:   7,
			PublishedRevision:  3,
			DefinitionHash:     "sha256:definition-only",
			ContentDigest:      "sha256:content-only",
		})
		ctx.Status(http.StatusOK)
	})
	router.POST("/admin/api/presentation-profiles/:id/rollback", func(ctx *gin.Context) {
		ctx.Status(http.StatusForbidden)
	})

	const (
		profileID      = "profile-audit-1"
		sensitiveLabel = "Executive Revenue Board"
		sensitiveDesc  = "Acquisition target details"
		sensitiveRule  = "customer.margin > 9000"
		rawSubject     = "role-merger-secret"
		rawKey         = "publish-secret-idempotency-key"
	)
	document := fmt.Sprintf(`{
		"scope":"role",
		"subjectID":%q,
		"pageKey":"orders.list",
		"document":{
			"metadata":{"label":%q,"description":%q},
			"spec":{"condition":%q}
		}
	}`, rawSubject, sensitiveLabel, sensitiveDesc, sensitiveRule)

	requests := []struct {
		method string
		path   string
		body   string
		key    string
		status int
	}{
		{method: http.MethodPost, path: "/admin/api/presentation-profiles", body: document, status: http.StatusForbidden},
		{method: http.MethodPost, path: "/admin/api/presentation-profiles/validate", body: document, status: http.StatusForbidden},
		{method: http.MethodPut, path: "/admin/api/presentation-profiles/" + profileID + "/draft", body: document, status: http.StatusForbidden},
		{method: http.MethodPost, path: "/admin/api/presentation-profiles/" + profileID + "/publish", key: rawKey, status: http.StatusOK},
		{method: http.MethodPost, path: "/admin/api/presentation-profiles/" + profileID + "/rollback", body: `{"revision":1}`, key: "rollback-secret-idempotency-key", status: http.StatusForbidden},
	}
	for _, test := range requests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		if test.key != "" {
			request.Header.Set("Idempotency-Key", test.key)
		}
		router.ServeHTTP(recorder, request)
		require.Equal(t, test.status, recorder.Code, "%s %s", test.method, test.path)
	}

	var logs []models.AuditLog
	require.NoError(t, db.Order("created_at ASC, id ASC").Find(&logs).Error)
	require.Len(t, logs, len(requests))
	for _, audit := range logs {
		require.Empty(t, audit.Request, "%s %s retained request values", audit.Method, audit.Path)
	}

	allEvidence, err := json.Marshal(logs)
	require.NoError(t, err)
	for _, secret := range []string{
		sensitiveLabel,
		sensitiveDesc,
		sensitiveRule,
		rawSubject,
		rawKey,
		"rollback-secret-idempotency-key",
	} {
		require.NotContains(t, string(allEvidence), secret)
	}

	want := []PresentationAuditMetadata{
		{Transition: "create-draft", Outcome: "authorization_failed"},
		{Transition: "validate", Outcome: "authorization_failed"},
		{AggregateID: profileID, Transition: "replace-draft", Outcome: "authorization_failed"},
		{
			AggregateID: profileID, PageKey: "orders.list", Scope: "role", SubjectPresent: true,
			SubjectFingerprint: "sha256:subject-fingerprint-only", Transition: "publish", Outcome: "success",
			AggregateVersion: 7, PublishedRevision: 3,
			DefinitionHash: "sha256:definition-only", ContentDigest: "sha256:content-only",
		},
		{AggregateID: profileID, Transition: "rollback", Outcome: "authorization_failed"},
	}
	for index := range logs {
		var metadata PresentationAuditMetadata
		require.NoError(t, json.Unmarshal([]byte(logs[index].Message), &metadata), logs[index].Message)
		require.Equal(t, want[index], metadata)
		if logs[index].Status != enum.Enabled && logs[index].Status != enum.Disabled {
			t.Fatalf("unexpected audit status %q", logs[index].Status)
		}
	}
}

func TestPresentationAuditBodyCaptureFailsClosedForEveryMutationRoute(t *testing.T) {
	paths := []string{
		"/admin/api/presentation-profiles",
		"/admin/api/presentation-profiles/validate",
		"/admin/api/presentation-profiles/profile-1/draft",
		"/admin/api/presentation-profiles/profile-1/publish",
		"/admin/api/presentation-profiles/profile-1/rollback",
	}
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"label":"never-capture"}`))
		request.Header.Set("Content-Type", "application/json")
		require.False(t, shouldCaptureAuditRequestBody(request), path)
	}
}
