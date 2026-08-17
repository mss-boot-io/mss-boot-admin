package apis

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthstate"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPersistOAuthBindingConcurrentIdentityClaimIsUnique(t *testing.T) {
	db := setupOAuthBindingTestDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)

	for _, userID := range []string{"user-a", "user-b"} {
		userID := userID
		go func() {
			ctx := newOAuthBindingTestContext()
			identity := &models.UserOAuth2{
				Provider: pkg.GithubLoginProvider,
				OpenID:   "42",
			}
			ready.Done()
			<-start
			results <- persistOAuthBinding(ctx, userID, identity)
		}()
	}
	ready.Wait()
	close(start)

	var succeeded, conflicted int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, errOAuthIdentityAlreadyBound):
			conflicted++
		default:
			t.Fatalf("concurrent bind returned unstable error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent results success=%d conflict=%d, want 1/1", succeeded, conflicted)
	}

	var rows []models.UserOAuth2
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].IdentityKey == nil || *rows[0].IdentityKey != "github:42" {
		t.Fatalf("persisted concurrent identities = %#v", rows)
	}
}

func TestOAuthCallbackMapsDatabaseIdentityConflictToHTTP409(t *testing.T) {
	setupOAuthBindingTestDB(t)
	installOAuthTestVerifier(t)
	configureOAuthBrowserSession(t)
	if err := persistOAuthBinding(
		newOAuthBindingTestContext(),
		"identity-owner",
		&models.UserOAuth2{Provider: pkg.GithubLoginProvider, OpenID: "42"},
	); err != nil {
		t.Fatalf("seed OAuth identity owner: %v", err)
	}

	exchanges := 0
	user := newOAuthTestUser(&exchanges)
	user.oauthBindingComplete = func(
		c *gin.Context,
		userID string,
		provider pkg.LoginProvider,
		_ string,
	) error {
		return persistOAuthBinding(c, userID, &models.UserOAuth2{
			Provider: provider,
			OpenID:   "42",
		})
	}
	verifier := oauthTestUser("identity-contender", false)
	state, cookie := issueOAuthState(
		t,
		user,
		pkg.GithubLoginProvider,
		oauthstate.IntentBinding,
		testOAuthCredential,
		verifier,
	)
	callback := executeOAuthCallback(
		user,
		pkg.GithubLoginProvider,
		state,
		testOAuthCredential,
		verifier,
		cookie,
	)
	if callback.Code != http.StatusConflict || exchanges != 1 {
		t.Fatalf("database identity conflict status=%d exchanges=%d body=%s, want 409/1",
			callback.Code, exchanges, callback.Body.String())
	}
}

func TestPersistOAuthBindingIdempotentAndProviderScoped(t *testing.T) {
	db := setupOAuthBindingTestDB(t)
	ctx := newOAuthBindingTestContext()

	first := &models.UserOAuth2{Provider: pkg.LarkLoginProvider, UnionID: "on_same"}
	if err := persistOAuthBinding(ctx, "same-user", first); err != nil {
		t.Fatalf("first bind: %v", err)
	}
	retry := &models.UserOAuth2{Provider: pkg.LarkLoginProvider, UnionID: "on_same"}
	if err := persistOAuthBinding(ctx, "same-user", retry); err != nil {
		t.Fatalf("same-user idempotent bind: %v", err)
	}

	providerScoped := &models.UserOAuth2{Provider: pkg.GithubLoginProvider, OpenID: "on_same"}
	if err := persistOAuthBinding(ctx, "github-user", providerScoped); err != nil {
		t.Fatalf("provider-scoped identity bind: %v", err)
	}
	caseDistinct := &models.UserOAuth2{Provider: pkg.LarkLoginProvider, UnionID: "on_SAME"}
	if err := persistOAuthBinding(ctx, "case-distinct-user", caseDistinct); err != nil {
		t.Fatalf("case-distinct opaque identity bind: %v", err)
	}

	invalid := &models.UserOAuth2{Provider: pkg.LarkLoginProvider, OpenID: "wrong-field"}
	if err := persistOAuthBinding(ctx, "invalid-user", invalid); err == nil {
		t.Fatal("binding without the provider's critical ID succeeded")
	}

	var count int64
	if err := db.Model(&models.UserOAuth2{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("OAuth binding count = %d, want 3", count)
	}
}

func TestOAuthIdentityUniqueViolationClassificationWithoutTranslateError(t *testing.T) {
	tests := []error{
		errors.New("UNIQUE constraint failed: mss_boot_user_oauth2.identity_key"),
		errors.New("Error 1062 (23000): Duplicate entry 'github:42' for key 'ux_user_oauth2_identity_key'"),
		fakeOAuthSQLStateError{state: "23505"},
	}
	for _, err := range tests {
		if !isOAuthIdentityUniqueViolation(err) {
			t.Fatalf("unique violation %q was not classified", err)
		}
	}
	if isOAuthIdentityUniqueViolation(errors.New("database is unavailable")) {
		t.Fatal("non-unique database failure was classified as identity conflict")
	}
}

type fakeOAuthSQLStateError struct {
	state string
}

func (e fakeOAuthSQLStateError) Error() string    { return fmt.Sprintf("sqlstate %s", e.state) }
func (e fakeOAuthSQLStateError) SQLState() string { return e.state }

func setupOAuthBindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousDB := gormdb.DB
	dsn := filepath.Join(t.TempDir(), "oauth-binding.db") + "?_busy_timeout=10000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	if err := db.AutoMigrate(&models.UserOAuth2{}); err != nil {
		t.Fatalf("migrate OAuth binding test database: %v", err)
	}
	gormdb.DB = db
	t.Cleanup(func() {
		gormdb.DB = previousDB
		_ = sqlDB.Close()
	})
	return db
}

func newOAuthBindingTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/api/user/session/github/callback", nil)
	return c
}
