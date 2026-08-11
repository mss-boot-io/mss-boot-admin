package models

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type githubTestAppConfig map[string]string

func (c githubTestAppConfig) SetAppConfig(*gin.Context, string, bool, string) error {
	return nil
}

func (c githubTestAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := c[key]
	return value, ok
}

func TestGithubVerifyRejectsUntrustedProviderIdentityBeforeRegistration(t *testing.T) {
	tests := []struct {
		name         string
		config       githubTestAppConfig
		userStatus   int
		userBody     string
		orgBody      string
		wantError    string
		wantRequests int
	}{
		{
			name:         "provider disabled",
			config:       githubTestAppConfig{"security:githubEnabled": "false"},
			userStatus:   http.StatusOK,
			userBody:     `{"id":42,"login":"octocat"}`,
			wantError:    "github login is disabled",
			wantRequests: 0,
		},
		{
			name:         "invalid access token",
			config:       githubTestAppConfig{"security:githubEnabled": "true"},
			userStatus:   http.StatusUnauthorized,
			userBody:     `{"id":42,"login":"must-not-register"}`,
			wantError:    "status 401",
			wantRequests: 1,
		},
		{
			name: "organization must match exactly",
			config: githubTestAppConfig{
				"security:githubEnabled":    "true",
				"security:githubAllowGroup": "allowed-org",
			},
			userStatus:   http.StatusOK,
			userBody:     `{"id":42,"login":"octocat"}`,
			orgBody:      `[{"login":"allowed-org-extra"}]`,
			wantError:    "user not in allow group",
			wantRequests: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := setupGithubVerifyTest(t, tt.config)
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				switch r.URL.Path {
				case "/user":
					w.WriteHeader(tt.userStatus)
					_, _ = w.Write([]byte(tt.userBody))
				case "/user/orgs":
					_, _ = w.Write([]byte(tt.orgBody))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			ctx := newGithubVerifyContext(server.URL)
			login := &UserLogin{Provider: pkg.GithubLoginProvider, Password: "provider-token"}
			ok, verifier, err := login.Verify(ctx)
			require.ErrorContains(t, err, tt.wantError)
			require.False(t, ok)
			require.Nil(t, verifier)
			require.Equal(t, tt.wantRequests, requestCount)
			requireGithubRegistrationCount(t, database, 0, 0)
		})
	}
}

func TestGithubVerifyPreservesValidLoginWithExactCaseInsensitiveOrganization(t *testing.T) {
	const providerToken = "github-provider-token-must-not-become-local-password"
	const providerUsername = "octocat-provider-name-too-long"
	config := githubTestAppConfig{
		"security:githubEnabled":    "true",
		"security:githubAllowGroup": " allowed-org ",
		"security:registerEnabled":  "true",
	}
	database := setupGithubVerifyTest(t, config)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"id":42,"login":"` + providerUsername + `","name":"Octo Cat"}`))
		case "/user/orgs":
			_, _ = w.Write([]byte(`[{"login":"Allowed-Org"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	login := &UserLogin{Provider: pkg.GithubLoginProvider, Password: providerToken}
	ok, verifier, err := login.Verify(newGithubVerifyContext(server.URL))
	require.NoError(t, err)
	require.True(t, ok)
	user, ok := verifier.(*User)
	require.True(t, ok)
	require.Len(t, user.Username, 20)
	require.Regexp(t, `^[a-f0-9]{20}$`, user.Username)
	require.NotEqual(t, providerUsername, user.Username)
	require.NotContains(t, user.Username, "@")
	require.True(t, user.LocalPasswordDisabled)
	require.NotEqual(t, providerToken, user.Password)
	requireGithubRegistrationCount(t, database, 1, 1)

	var persisted User
	require.NoError(t, database.First(&persisted, "id = ?", user.ID).Error)
	require.Equal(t, user.Username, persisted.Username)
	require.True(t, persisted.LocalPasswordDisabled)
	providerDerivedHash, err := security.SetPassword(providerToken, persisted.Salt)
	require.NoError(t, err)
	require.NotEqual(t, providerDerivedHash, persisted.PasswordHash,
		"provider token must never become a durable local credential")

	for _, localPassword := range []string{"", providerToken} {
		localLogin := &UserLogin{Username: user.Username, Password: localPassword}
		localOK, _, localErr := localLogin.Verify(newGithubVerifyContext(server.URL))
		require.NoError(t, localErr)
		require.False(t, localOK, "OAuth-only user accepted local password %q", localPassword)
	}
}

func TestOpaqueOAuthUsernameGenerationIsDeterministicAndBounded(t *testing.T) {
	username, err := newOpaqueOAuthUsername(bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
	}))
	require.NoError(t, err)
	require.Equal(t, "00010203040506070809", username)
	require.LessOrEqual(t, len(username), 20)
	require.NotContains(t, username, "@")

	const entropyFailureDetail = "entropy-source-secret-detail"
	_, err = newOpaqueOAuthUsername(oauthEntropyReaderFunc(func([]byte) (int, error) {
		return 0, errors.New(entropyFailureDetail)
	}))
	require.Error(t, err)
	require.NotContains(t, err.Error(), entropyFailureDetail)
}

type oauthEntropyReaderFunc func([]byte) (int, error)

func (f oauthEntropyReaderFunc) Read(buffer []byte) (int, error) {
	return f(buffer)
}

func TestOpaqueOAuthUsernameRetriesExistingCollision(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{})
	const collision = "00000000000000000000"
	existing := &User{UserLogin: UserLogin{
		Username: collision,
		Password: "local-password",
		Status:   enum.Enabled,
	}}
	require.NoError(t, database.Create(existing).Error)

	entropy := append(make([]byte, oauthUsernameEntropySize), bytes.Repeat([]byte{0x01}, oauthUsernameEntropySize)...)
	username, err := newAvailableOpaqueOAuthUsername(database, bytes.NewReader(entropy))
	require.NoError(t, err)
	require.Equal(t, "01010101010101010101", username)
	require.NotEqual(t, collision, username)
}

func TestGithubFirstProvisioningNeverMergesByCanonicalEmail(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:githubEnabled":   "true",
		"security:registerEnabled": "true",
	})
	installCanonicalEmailIdentitySQLiteIndex(t, database)
	owner := &User{UserLogin: UserLogin{
		Username: "local-email-owner",
		Email:    "owner@example.com",
		Password: "local-password",
		Status:   enum.Enabled,
	}}
	require.NoError(t, database.Create(owner).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":4242,"login":"provider-identity","email":"OWNER@EXAMPLE.COM"}`))
	}))
	t.Cleanup(server.Close)

	ok, verifier, err := (&UserLogin{
		Provider: pkg.GithubLoginProvider,
		Password: "provider-token",
	}).Verify(newGithubVerifyContext(server.URL))
	require.ErrorIs(t, err, ErrEmailIdentityExists)
	require.False(t, ok)
	require.Nil(t, verifier)
	requireGithubRegistrationCount(t, database, 1, 0)

	var unchanged User
	require.NoError(t, database.First(&unchanged, "id = ?", owner.ID).Error)
	require.Equal(t, owner.Username, unchanged.Username)
	require.Equal(t, owner.Email, unchanged.Email)
}

func TestGithubFirstProvisioningRequiresRegistrationAndSafeDefaultRole(t *testing.T) {
	tests := []struct {
		name       string
		config     githubTestAppConfig
		mutateRole func(*testing.T, *gorm.DB)
		wantError  string
	}{
		{
			name: "registration disabled",
			config: githubTestAppConfig{
				"security:githubEnabled":   "true",
				"security:registerEnabled": "false",
			},
			wantError: "public registration is disabled",
		},
		{
			name: "registration switch missing",
			config: githubTestAppConfig{
				"security:githubEnabled": "true",
			},
			wantError: "public registration is disabled",
		},
		{
			name: "root default",
			config: githubTestAppConfig{
				"security:githubEnabled":   "true",
				"security:registerEnabled": "true",
			},
			mutateRole: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				if err := db.Exec(`UPDATE mss_boot_roles SET root = ? WHERE "default" = ?`, true, true).Error; err != nil {
					t.Fatalf("mark default role root: %v", err)
				}
			},
			wantError: "enabled and non-root",
		},
		{
			name: "disabled default",
			config: githubTestAppConfig{
				"security:githubEnabled":   "true",
				"security:registerEnabled": "true",
			},
			mutateRole: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				if err := db.Model(&Role{}).Where(map[string]any{"default": true}).
					Update("status", enum.Disabled).Error; err != nil {
					t.Fatalf("disable default role: %v", err)
				}
			},
			wantError: "enabled and non-root",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database := setupGithubVerifyTest(t, test.config)
			if test.mutateRole != nil {
				test.mutateRole(t, database)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":42,"login":"new-user"}`))
			}))
			t.Cleanup(server.Close)

			ok, verifier, err := (&UserLogin{
				Provider: pkg.GithubLoginProvider,
				Password: "provider-token",
			}).Verify(newGithubVerifyContext(server.URL))
			require.ErrorContains(t, err, test.wantError)
			require.False(t, ok)
			require.Nil(t, verifier)
			requireGithubRegistrationCount(t, database, 0, 0)
		})
	}
}

func TestGithubExistingIdentityLoginIgnoresClosedRegistration(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:githubEnabled":   "true",
		"security:registerEnabled": "false",
	})
	var role Role
	require.NoError(t, database.Where(map[string]any{"default": true}).First(&role).Error)
	user := &User{UserLogin: UserLogin{
		RoleID:   role.ID,
		Username: "existing-oauth-user",
		Password: "unusable-local-password",
		Status:   enum.Enabled,
	}}
	require.NoError(t, database.Create(user).Error)
	identity := &UserOAuth2{
		UserID:   user.ID,
		OpenID:   "42",
		Provider: pkg.GithubLoginProvider,
	}
	require.NoError(t, database.Create(identity).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"login":"existing-oauth-user"}`))
	}))
	t.Cleanup(server.Close)
	ok, verifier, err := (&UserLogin{
		Provider: pkg.GithubLoginProvider,
		Password: "provider-token",
	}).Verify(newGithubVerifyContext(server.URL))
	require.NoError(t, err)
	require.True(t, ok)
	current, typeOK := verifier.(*User)
	require.True(t, typeOK)
	require.Equal(t, user.ID, current.ID)
	requireGithubRegistrationCount(t, database, 1, 1)
}

func TestGithubVerifyDoesNotLogUntrustedHookErrorDetail(t *testing.T) {
	const providerToken = "github-provider-token-log-sentinel"
	const hookDetail = "hook-secret-detail"
	setupGithubVerifyTest(t, githubTestAppConfig{
		"security:githubEnabled": "true",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"login":"octocat"}`))
	}))
	defer server.Close()

	BeforeGithubVerify = func(context.Context, *pkg.GithubUser, string) error {
		return errors.New(hookDetail + ": " + providerToken)
	}
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	login := &UserLogin{Provider: pkg.GithubLoginProvider, Password: providerToken}
	ok, verifier, err := login.Verify(newGithubVerifyContext(server.URL))
	if err == nil || !strings.Contains(err.Error(), hookDetail) {
		t.Fatalf("Verify() error = %v, want unmodified hook error", err)
	}
	if ok || verifier != nil {
		t.Fatalf("Verify() = (%v, %#v), want rejected hook", ok, verifier)
	}
	if strings.Contains(logs.String(), providerToken) || strings.Contains(logs.String(), hookDetail) {
		t.Fatalf("OAuth verification log leaked untrusted detail: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "github identity verification failed") {
		t.Fatalf("OAuth verification log did not contain generic diagnostic: %s", logs.String())
	}
}

func TestEmailRegistrationUsesPublicRegistrationSwitch(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "false",
	})

	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Email:    "disabled-registration@example.com",
		Captcha:  "unused",
	}
	ok, verifier, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	require.ErrorContains(t, err, "public registration is disabled")
	require.False(t, ok)
	require.Nil(t, verifier)
	requireGithubRegistrationCount(t, database, 0, 0)
}

func TestEmailRegistrationRequiresExplicitRegistrationSetting(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{})
	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Email:    "missing-registration-setting@example.com",
		Captcha:  "unused",
	}
	ok, verifier, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	require.ErrorContains(t, err, "public registration is disabled")
	require.False(t, ok)
	require.Nil(t, verifier)
	requireGithubRegistrationCount(t, database, 0, 0)
}

func TestEmailRegistrationRejectsClientSuppliedRefreshIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	victim := &User{
		UserLogin: UserLogin{
			Username: "known-user",
			Password: "stored-password",
			RoleID:   "existing-role",
			Status:   enum.Enabled,
		},
	}
	require.NoError(t, database.Create(victim).Error)

	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Username: victim.Username,
		RoleID:   victim.RoleID,
	}
	ok, verifier, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, verifier)
	requireGithubRegistrationCount(t, database, 1, 0)
}

func setupGithubVerifyTest(t *testing.T, config githubTestAppConfig) *gorm.DB {
	t.Helper()
	oldDB := gormdb.DB
	oldConfig := center.GetAppConfig()
	oldBeforeVerify := BeforeGithubVerify
	t.Cleanup(func() {
		gormdb.DB = oldDB
		center.SetAppConfig(oldConfig)
		BeforeGithubVerify = oldBeforeVerify
	})

	dsn := filepath.Join(t.TempDir(), "github-verify.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, database.AutoMigrate(&Role{}, &User{}, &UserOAuth2{}))
	gormdb.DB = database
	center.SetAppConfig(config)
	BeforeGithubVerify = nil

	role := &Role{Name: "default", Default: true, Status: enum.Enabled}
	require.NoError(t, database.Create(role).Error)
	require.NoError(t, database.Exec(
		`UPDATE mss_boot_roles SET "default" = ? WHERE id = ?`,
		true,
		role.ID,
	).Error)
	return database
}

func newGithubVerifyContext(baseURL string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext := pkg.WithGithubAPIBaseURL(context.Background(), baseURL)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", nil).WithContext(requestContext)
	return c
}

func requireGithubRegistrationCount(t *testing.T, database *gorm.DB, users, oauthIdentities int64) {
	t.Helper()
	var userCount int64
	var oauthCount int64
	require.NoError(t, database.Model(&User{}).Count(&userCount).Error)
	require.NoError(t, database.Model(&UserOAuth2{}).Count(&oauthCount).Error)
	require.Equal(t, users, userCount)
	require.Equal(t, oauthIdentities, oauthCount)
}
