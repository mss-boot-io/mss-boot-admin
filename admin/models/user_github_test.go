package models

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	config := githubTestAppConfig{
		"security:githubEnabled":    "true",
		"security:githubAllowGroup": " allowed-org ",
	}
	database := setupGithubVerifyTest(t, config)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"id":42,"login":"octocat","name":"Octo Cat"}`))
		case "/user/orgs":
			_, _ = w.Write([]byte(`[{"login":"Allowed-Org"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	login := &UserLogin{Provider: pkg.GithubLoginProvider, Password: "provider-token"}
	ok, verifier, err := login.Verify(newGithubVerifyContext(server.URL))
	require.NoError(t, err)
	require.True(t, ok)
	user, ok := verifier.(*User)
	require.True(t, ok)
	require.Equal(t, "octocat", user.Username, "login is the safe fallback when GitHub email is private")
	requireGithubRegistrationCount(t, database, 1, 1)
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
	require.ErrorContains(t, err, "email register not support")
	require.False(t, ok)
	require.Nil(t, verifier)
	requireGithubRegistrationCount(t, database, 0, 0)
}

func TestEmailRegistrationRejectsClientSuppliedRefreshIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
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

	dsn := fmt.Sprintf("file:github-verify-%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Role{}, &User{}, &UserOAuth2{}))
	gormdb.DB = database
	center.SetAppConfig(config)
	BeforeGithubVerify = nil

	role := &Role{Name: "default", Default: true, Status: enum.Enabled}
	require.NoError(t, database.Create(role).Error)
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
