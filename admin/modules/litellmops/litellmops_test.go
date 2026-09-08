package litellmops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "litellmops.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func migrateTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := migrateSnapshots(db, SnapshotMigrationID.String()); err != nil {
		t.Fatalf("migrate snapshots: %v", err)
	}
	if err := db.AutoMigrate(&models.CasbinRule{}); err != nil {
		t.Fatalf("migrate casbin: %v", err)
	}
	if err := migrateAuthorization(db, AuthorizationMigrationID.String()); err != nil {
		t.Fatalf("migrate authorization: %v", err)
	}
}

const fullTestToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func fakeLiteLLM(t *testing.T, users []map[string]any, keys []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-master-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"users": users, "total": len(users)})
		case "/key/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys, "total_count": len(keys)})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func baseUsers() []map[string]any {
	return []map[string]any{
		{
			"user_id":         "u-1",
			"user_email":      "lwnmengjing@gmail.com",
			"user_role":       "internal_user",
			"models":          []any{"grok-4.6", "gpt-5.6-luna"},
			"max_budget":      10.0,
			"budget_duration": "1mo",
			"budget_reset_at": "2026-10-01T00:00:00Z",
			"spend":           3.8331,
		},
		{
			"user_id":    "u-2",
			"user_email": "newuser@example.com",
			"user_role":  "internal_user",
			"models":     []any{"grok-4.6"},
			"max_budget": 5.0,
			"spend":      0.0,
		},
	}
}

func baseKeys() []map[string]any {
	created := time.Now().UTC().Add(-time.Hour)
	sessionCreated := time.Now().UTC().Add(-2 * time.Hour)
	return []map[string]any{
		{
			"token":                 fullTestToken,
			"key_alias":             "test",
			"user_id":               "u-1",
			"max_budget":            10.0,
			"spend":                 3.8331,
			"tpm_limit":             1000000,
			"rpm_limit":             60,
			"max_parallel_requests": 10,
			"expires":               created.Add(90 * 24 * time.Hour).Format(time.RFC3339),
			"created_at":            created.Format(time.RFC3339),
		},
		{
			"token":      "abcdef0123456789abcdef0123456789",
			"key_alias":  "",
			"user_id":    "u-1",
			"max_budget": 1.0,
			"spend":      0.0,
			"expires":    sessionCreated.Add(24 * time.Hour).Format(time.RFC3339),
			"created_at": sessionCreated.Format(time.RFC3339),
		},
	}
}

func TestSyncSnapshotsWritesReadOnlyViews(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	server := fakeLiteLLM(t, baseUsers(), baseKeys())
	defer server.Close()
	client, err := NewClient(server.URL, "test-master-key", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	report, err := SyncSnapshots(context.Background(), db, client)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if report.Users != 2 || report.Keys != 2 || report.UsersRetired != 0 || report.KeysRetired != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}

	var user UserSnapshot
	if err := db.First(&user, "user_id = ?", "u-1").Error; err != nil {
		t.Fatalf("load user snapshot: %v", err)
	}
	if user.Email != "lwnmengjing@gmail.com" || user.UserRole != "internal_user" {
		t.Fatalf("unexpected user snapshot: %+v", user)
	}
	if user.MaxBudget == nil || *user.MaxBudget != 10.0 {
		t.Fatalf("unexpected max budget: %+v", user.MaxBudget)
	}
	if !strings.Contains(user.Models, "gpt-5.6-luna") {
		t.Fatalf("models not stored as json: %s", user.Models)
	}

	var key KeySnapshot
	if err := db.First(&key, "user_id = ? AND alias = ?", "u-1", "test").Error; err != nil {
		t.Fatalf("load key snapshot: %v", err)
	}
	if key.KeyHashPrefix != fullTestToken[:keyHashPrefixLength] {
		t.Fatalf("unexpected key prefix: %s", key.KeyHashPrefix)
	}
	if len(key.KeyHashPrefix) > keyHashPrefixLength {
		t.Fatalf("prefix exceeds bound: %d", len(key.KeyHashPrefix))
	}
	if key.TPMLimit == nil || *key.TPMLimit != 1000000 || key.IsSessionKey {
		t.Fatalf("unexpected key snapshot: %+v", key)
	}

	// The complete key material must never be persisted.
	var stored int64
	if err := db.Model(&KeySnapshot{}).
		Where("key_hash_prefix = ?", fullTestToken).
		Count(&stored).Error; err != nil {
		t.Fatalf("count full token: %v", err)
	}
	if stored != 0 {
		t.Fatal("complete key material was persisted")
	}

	var sessionKey KeySnapshot
	if err := db.First(&sessionKey, "key_hash_prefix = ?", "abcdef0123456789").Error; err != nil {
		t.Fatalf("load session key snapshot: %v", err)
	}
	if !sessionKey.IsSessionKey {
		t.Fatal("session key was not flagged")
	}
	if sessionKey.Alias != nil {
		t.Fatalf("session key alias should be nil: %+v", sessionKey.Alias)
	}
}

func TestSyncSnapshotsRetiresAndRestores(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	server := fakeLiteLLM(t, baseUsers(), baseKeys())
	defer server.Close()
	client, err := NewClient(server.URL, "test-master-key", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := SyncSnapshots(context.Background(), db, client); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Upstream drops the session key.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"users": baseUsers()})
		case "/key/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": baseKeys()[:1]})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	report, err := SyncSnapshots(context.Background(), db, client)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if report.KeysRetired != 1 {
		t.Fatalf("expected 1 retired key, got %+v", report)
	}
	var visible int64
	if err := db.Model(&KeySnapshot{}).Where("key_hash_prefix = ?", "abcdef0123456789").Count(&visible).Error; err != nil {
		t.Fatalf("count visible: %v", err)
	}
	if visible != 0 {
		t.Fatal("retired key still visible")
	}

	// Upstream re-adds it: the row is restored, not duplicated.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"users": baseUsers()})
		case "/key/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": baseKeys()})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if _, err := SyncSnapshots(context.Background(), db, client); err != nil {
		t.Fatalf("third sync: %v", err)
	}
	var total int64
	if err := db.Unscoped().Model(&KeySnapshot{}).Where("key_hash_prefix = ?", "abcdef0123456789").Count(&total).Error; err != nil {
		t.Fatalf("count restored: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected single restored row, got %d", total)
	}
	if err := db.Model(&KeySnapshot{}).Where("key_hash_prefix = ?", "abcdef0123456789").Count(&visible).Error; err != nil {
		t.Fatalf("count visible after restore: %v", err)
	}
	if visible != 1 {
		t.Fatal("restored key not visible")
	}
}

func TestRemoteKeyIsSession(t *testing.T) {
	created := time.Now().UTC()
	one := 1.0
	session := RemoteKey{
		MaxBudget: &one,
		CreatedAt: &created,
		Expires:   ptrTime(created.Add(24 * time.Hour)),
	}
	if !session.IsSession() {
		t.Fatal("expected session key shape")
	}
	withAlias := session
	withAlias.Alias = "named"
	if withAlias.IsSession() {
		t.Fatal("alias must exclude session shape")
	}
	longLived := session
	longLived.Expires = ptrTime(created.Add(90 * 24 * time.Hour))
	if longLived.IsSession() {
		t.Fatal("long-lived key must exclude session shape")
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestBillsListAggregates(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec(`CREATE TABLE "LiteLLM_SpendLogs" (
		"request_id" VARCHAR(64) PRIMARY KEY,
		"model" VARCHAR(128),
		"status" VARCHAR(32),
		"spend" REAL,
		"prompt_tokens" BIGINT,
		"completion_tokens" BIGINT,
		"total_tokens" BIGINT,
		"startTime" DATETIME,
		"api_key" VARCHAR(128)
	)`).Error; err != nil {
		t.Fatalf("create spend table: %v", err)
	}
	now := time.Now().UTC()
	rows := []SpendLog{
		{RequestID: "r-1", Model: "grok-4.6", Status: "success", Spend: 0.2, TotalTokens: 100, StartTime: now.Add(-time.Hour), APIKey: "abc123fullhash"},
		{RequestID: "r-2", Model: "gpt-5.6-luna", Status: "success", Spend: 0.1, TotalTokens: 50, StartTime: now, APIKey: "abc123fullhash"},
		{RequestID: "r-3", Model: "grok-4.6", Status: "failure", Spend: 0.0, TotalTokens: 0, StartTime: now, APIKey: "deadbeef"},
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed spend log: %v", err)
		}
	}

	service, err := NewBillsService(db)
	if err != nil {
		t.Fatalf("new bills service: %v", err)
	}
	page, err := service.List(context.Background(), BillsQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list bills: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("unexpected total: %+v", page)
	}
	if page.SpendSum < 0.29 || page.SpendSum > 0.31 {
		t.Fatalf("unexpected spend sum: %v", page.SpendSum)
	}
	if page.TokenSum != 150 {
		t.Fatalf("unexpected token sum: %v", page.TokenSum)
	}

	filtered, err := service.List(context.Background(), BillsQuery{Model: "gpt-5.6-luna", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list filtered bills: %v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].RequestID != "r-2" {
		t.Fatalf("unexpected filtered page: %+v", filtered)
	}

	byKey, err := service.List(context.Background(), BillsQuery{KeyHashPrefix: "abc123", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by key: %v", err)
	}
	if byKey.Total != 2 {
		t.Fatalf("unexpected key-filtered page: %+v", byKey)
	}
}

// fakeVerifier implements security.Verifier for authorization tests.
type fakeVerifier struct {
	role string
	root bool
}

func (verifier *fakeVerifier) GetUserID() string             { return "test-user" }
func (verifier *fakeVerifier) GetTenantID() string           { return "" }
func (verifier *fakeVerifier) GetRoleID() string             { return verifier.role }
func (verifier *fakeVerifier) GetEmail() string              { return "" }
func (verifier *fakeVerifier) GetUsername() string           { return "test-user" }
func (verifier *fakeVerifier) GetRefreshTokenDisable() bool  { return false }
func (verifier *fakeVerifier) SetRefreshTokenDisable(bool)   {}
func (verifier *fakeVerifier) CheckToken(context.Context, string) error {
	return nil
}
func (verifier *fakeVerifier) Root() bool { return verifier.root }
func (verifier *fakeVerifier) Verify(context.Context) (bool, security.Verifier, error) {
	return true, verifier, nil
}
func (verifier *fakeVerifier) GetPersonAccessToken() string     { return "" }
func (verifier *fakeVerifier) SetPersonAccessToken(token string) {}

func performAuthorizedRequest(
	t *testing.T,
	authorizer *AdminAuthorizer,
	permission string,
	method string,
	path string,
) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, path, func(ctx *gin.Context) {
		if err := authorizer.Authorize(ctx, permission); err != nil {
			status := http.StatusForbidden
			if err == ErrAuthenticationRequired {
				status = http.StatusUnauthorized
			}
			ctx.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}
		ctx.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestAdminAuthorizerEnforcesPolicy(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db) // seeds casbin rows for admin/finance/readonly roles

	newAuthorizer := func(verifier *fakeVerifier) *AdminAuthorizer {
		authorizer, err := NewAdminAuthorizer(
			func(ctx context.Context) (*gorm.DB, bool) { return db, true },
			func(ctx *gin.Context) security.Verifier { return verifier },
		)
		if err != nil {
			t.Fatalf("new authorizer: %v", err)
		}
		return authorizer
	}
	const usersPath = "/admin/api/litellmops/users"
	const syncPath = "/admin/api/litellmops/sync"

	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "admin"}), PermissionUserList, "GET", usersPath); code != http.StatusOK {
		t.Fatalf("admin should be allowed, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "litellmops-readonly"}), PermissionUserList, "GET", usersPath); code != http.StatusOK {
		t.Fatalf("readonly should list users, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "litellmops-readonly"}), PermissionSync, "POST", syncPath); code != http.StatusForbidden {
		t.Fatalf("readonly must not sync, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "litellmops-finance"}), PermissionSync, "POST", syncPath); code != http.StatusOK {
		t.Fatalf("finance should sync, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "stranger"}), PermissionUserList, "GET", usersPath); code != http.StatusForbidden {
		t.Fatalf("unknown role must be denied, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: "stranger", root: true}), PermissionUserList, "GET", usersPath); code != http.StatusOK {
		t.Fatalf("root bypass should be allowed, got %d", code)
	}
	if code := performAuthorizedRequest(t, newAuthorizer(&fakeVerifier{role: ""}), PermissionUserList, "GET", usersPath); code != http.StatusUnauthorized {
		t.Fatalf("missing role must be unauthorized, got %d", code)
	}
}
