package apis

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserControllerMapsEmailIdentityWriteFailuresWithoutDisclosure(t *testing.T) {
	db := openUserIdentityWriteTestDB(t)
	if err := db.AutoMigrate(&models.Role{}, &models.User{}); err != nil {
		t.Fatalf("migrate user identity test schema: %v", err)
	}
	if err := db.Exec(
		"CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
			" ON mss_boot_users (LOWER(TRIM(email)))" +
			" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
	).Error; err != nil {
		t.Fatalf("install canonical email index: %v", err)
	}
	if err := db.Exec(
		"CREATE UNIQUE INDEX ux_mss_boot_users_test_username" +
			" ON mss_boot_users (username) WHERE deleted_at IS NULL",
	).Error; err != nil {
		t.Fatalf("install username test index: %v", err)
	}
	for _, user := range []struct {
		id       string
		username string
		email    string
	}{
		{id: "identity-owner", username: "identity-owner", email: "owner@example.com"},
		{id: "identity-target", username: "identity-target", email: "target@example.com"},
	} {
		if err := db.Exec(
			"INSERT INTO mss_boot_users (id, username, email) VALUES (?, ?, ?)",
			user.id,
			user.username,
			user.email,
		).Error; err != nil {
			t.Fatalf("seed user %q: %v", user.id, err)
		}
	}

	var databaseLogs bytes.Buffer
	observedDB := db.Session(&gorm.Session{Logger: logger.New(
		log.New(&databaseLogs, "", 0),
		logger.Config{LogLevel: logger.Info, Colorful: false},
	)})
	var applicationLogs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&applicationLogs, nil)))

	previousDB := gormdb.DB
	previousAuth := response.AuthHandler
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = observedDB
	config.Cfg.Auth.IdentityKey = "test-user-write-identity"
	root := &models.User{UserLogin: models.UserLogin{Role: &models.Role{Root: true}}}
	currentPrincipal := root
	response.AuthHandler = func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, currentPrincipal)
		c.Next()
	}
	t.Cleanup(func() {
		gormdb.DB = previousDB
		response.AuthHandler = previousAuth
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		slog.SetDefault(previousLogger)
	})

	gin.SetMode(gin.TestMode)
	var registered *User
	for _, candidate := range response.Controllers {
		if userController, ok := candidate.(*User); ok {
			registered = userController
			break
		}
	}
	if registered == nil {
		t.Fatal("registered controller list did not contain the user controller")
	}
	if registered.Path() != "users" || registered.GetKey() != "id" {
		t.Fatalf("registered user route = %q/: %q, want users/:id", registered.Path(), registered.GetKey())
	}
	router := gin.New()
	action := registered.GetAction(response.Control)
	if action == nil {
		t.Fatal("user controller did not expose the GORM control action")
	}
	userRoutes := router.Group("/admin/api/"+registered.Path(), registered.Handlers()...)
	userRoutes.POST("", action.Handler()...)
	userRoutes.PUT("/:"+registered.GetKey(), action.Handler()...)

	tests := []struct {
		name      string
		method    string
		path      string
		body      string
		status    int
		errorCode string
		message   string
		forbidden []string
	}{
		{
			name:      "invalid canonical identity",
			method:    http.MethodPost,
			path:      "/admin/api/users",
			body:      `{"username":"invalid-email","email":"not-an-address"}`,
			status:    http.StatusUnprocessableEntity,
			errorCode: "INVALID_EMAIL_IDENTITY",
			message:   "email identity is invalid",
			forbidden: []string{"not-an-address", "user email identity"},
		},
		{
			name:      "create canonical identity conflict",
			method:    http.MethodPost,
			path:      "/admin/api/users",
			body:      `{"username":"identity-contender","email":" OWNER@EXAMPLE.COM "}`,
			status:    http.StatusConflict,
			errorCode: "EMAIL_IDENTITY_UNAVAILABLE",
			message:   "email identity is unavailable",
			forbidden: []string{"owner@example.com", "OWNER@EXAMPLE.COM", "UNIQUE", models.EmailIdentityUniqueIndex},
		},
		{
			name:      "update canonical identity conflict",
			method:    http.MethodPut,
			path:      "/admin/api/users/identity-target",
			body:      `{"email":" OWNER@EXAMPLE.COM "}`,
			status:    http.StatusConflict,
			errorCode: "EMAIL_IDENTITY_UNAVAILABLE",
			message:   "email identity is unavailable",
			forbidden: []string{"owner@example.com", "OWNER@EXAMPLE.COM", "UNIQUE", models.EmailIdentityUniqueIndex},
		},
		{
			name:      "unrelated unique constraint uses redacted fallback",
			method:    http.MethodPost,
			path:      "/admin/api/users",
			body:      `{"username":"identity-owner","email":"unrelated@example.com"}`,
			status:    http.StatusInternalServerError,
			errorCode: "WRITE_FAILED",
			message:   "request could not be completed",
			forbidden: []string{"identity-owner", "unrelated@example.com", "UNIQUE", "ux_mss_boot_users_test_username"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			result := httptest.NewRecorder()
			router.ServeHTTP(result, request)

			if result.Code != test.status {
				t.Fatalf("response = %d: %s", result.Code, result.Body.String())
			}
			body := result.Body.String()
			if !strings.Contains(body, test.errorCode) || !strings.Contains(body, test.message) {
				t.Fatalf("response did not contain fixed public error: %s", body)
			}
			for _, forbidden := range test.forbidden {
				if strings.Contains(body, forbidden) {
					t.Fatalf("response disclosed %q: %s", forbidden, body)
				}
			}
		})
	}

	t.Run("delegated administrator remains blocked by registered root guard", func(t *testing.T) {
		currentPrincipal = &models.User{UserLogin: models.UserLogin{Role: &models.Role{Root: false}}}
		t.Cleanup(func() { currentPrincipal = root })
		request := httptest.NewRequest(
			http.MethodPost,
			"/admin/api/users",
			bytes.NewBufferString(`{"username":"delegated-create","email":"delegated@example.com"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, request)
		if result.Code != http.StatusForbidden {
			t.Fatalf("delegated create response = %d: %s", result.Code, result.Body.String())
		}
	})

	var users int64
	if err := db.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatalf("count users after rejected writes: %v", err)
	}
	if users != 2 {
		t.Fatalf("users after rejected writes = %d, want 2", users)
	}

	diagnostics := strings.ToLower(databaseLogs.String() + "\n" + applicationLogs.String())
	for _, sensitive := range []string{
		"not-an-address",
		"owner@example.com",
		"unrelated@example.com",
		models.EmailIdentityUniqueIndex,
		"ux_mss_boot_users_test_username",
		"unique constraint",
		"constraint failed",
		"sqlstate 23505",
		"error 1062",
		"insert into mss_boot_users",
	} {
		if strings.Contains(diagnostics, strings.ToLower(sensitive)) {
			t.Fatalf("write diagnostics disclosed %q: %s", sensitive, diagnostics)
		}
	}
}

func TestUserWriteErrorMapperFallsBackForUnrelatedDatabaseErrors(t *testing.T) {
	publicError, matched := mapUserWriteError(
		nil,
		"create",
		gorm.ErrInvalidDB,
	)
	if matched || publicError.Status != 0 || publicError.Error != nil {
		t.Fatalf("unrelated database error mapped to %#v, matched=%t", publicError, matched)
	}
}

func openUserIdentityWriteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "user-identity-write.db") + "?_busy_timeout=10000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open user identity write database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap user identity write database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
