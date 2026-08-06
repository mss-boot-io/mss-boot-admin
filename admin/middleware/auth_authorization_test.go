package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
)

func TestAuthorizationObjectPrefersRegisteredRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	router := gin.New()
	router.PUT("/admin/api/user/:userID/password-reset", func(c *gin.Context) {
		got = authorizationObject(c)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/admin/api/user/user-123/password-reset", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got != "/admin/api/user/:userID/password-reset" {
		t.Fatalf("authorization object = %q, want registered route template", got)
	}
}

func TestAuthorizationObjectFallsBackToRequestPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/audit-logs/login?current=2", nil)

	if got := authorizationObject(ctx); got != "/admin/api/audit-logs/login" {
		t.Fatalf("authorization object = %q, want request path", got)
	}
}

func TestAuthorizationObjectHandlesMissingContext(t *testing.T) {
	if got := authorizationObject(nil); got != "" {
		t.Fatalf("nil-context authorization object = %q, want empty", got)
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if got := authorizationObject(ctx); got != "" {
		t.Fatalf("request-less authorization object = %q, want empty", got)
	}
}

func TestIsSelfServiceRequestUsesExactMethodAndRoute(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "notice unread", method: http.MethodGet, path: "/admin/api/notice/unread", want: true},
		{name: "notice detail", method: http.MethodGet, path: "/admin/api/notice/read/:id", want: true},
		{name: "notice mark read", method: http.MethodPut, path: "/admin/api/notice/read/:id", want: true},
		{name: "notice wrong method", method: http.MethodDelete, path: "/admin/api/notice/read/:id", want: false},
		{name: "notice lookalike", method: http.MethodGet, path: "/admin/api/notice/read/:id/export", want: false},

		{name: "user config profile", method: http.MethodGet, path: "/admin/api/user-configs/profile", want: true},
		{name: "user config group read", method: http.MethodGet, path: "/admin/api/user-configs/:group", want: true},
		{name: "user config group write", method: http.MethodPut, path: "/admin/api/user-configs/:group", want: true},
		{name: "user config group delete", method: http.MethodDelete, path: "/admin/api/user-configs/:group", want: false},

		{name: "personal token generate", method: http.MethodPost, path: "/admin/api/user-auth-tokens", want: true},
		{name: "personal token list", method: http.MethodGet, path: "/admin/api/user-auth-tokens", want: true},
		{name: "personal token revoke", method: http.MethodPut, path: "/admin/api/user-auth-token/:id/revoke", want: true},
		{name: "personal token refresh", method: http.MethodPut, path: "/admin/api/user-auth-token/:id/refresh", want: true},
		{name: "legacy personal token generate get", method: http.MethodGet, path: "/admin/api/user-auth-token/generate", want: false},
		{name: "personal token wrong method", method: http.MethodDelete, path: "/admin/api/user-auth-token/:id/revoke", want: false},

		{name: "password reset", method: http.MethodPost, path: "/admin/api/user/reset-password", want: true},
		{name: "user info read", method: http.MethodGet, path: "/admin/api/user/userInfo", want: true},
		{name: "user info update", method: http.MethodPut, path: "/admin/api/user/userInfo", want: true},
		{name: "user avatar", method: http.MethodPost, path: "/admin/api/user/avatar", want: true},
		{name: "user oauth bindings", method: http.MethodGet, path: "/admin/api/user/oauth2", want: true},
		{name: "user bind", method: http.MethodPost, path: "/admin/api/user/binding", want: true},
		{name: "user unbind", method: http.MethodDelete, path: "/admin/api/user/unbinding", want: true},
		{name: "user info wrong method", method: http.MethodDelete, path: "/admin/api/user/userInfo", want: false},
		{name: "public app profile with optional identity", method: http.MethodGet, path: "/admin/api/app-configs/profile", want: true},
		{name: "current session logout", method: http.MethodPost, path: "/admin/api/online-sessions/logout", want: true},
		{name: "current user websocket", method: http.MethodGet, path: "/admin/api/ws/connect", want: true},
		{name: "current user websocket wrong method", method: http.MethodPost, path: "/admin/api/ws/connect", want: false},
		{name: "current user websocket lookalike", method: http.MethodGet, path: "/admin/api/ws/connect/online", want: false},
		{name: "authenticated storage upload", method: http.MethodPost, path: "/admin/api/storage/upload", want: true},
		{name: "storage upload wrong method", method: http.MethodGet, path: "/admin/api/storage/upload", want: false},
		{name: "storage upload lookalike", method: http.MethodPost, path: "/admin/api/storage/upload/archive", want: false},

		{name: "department list no longer bypasses policy", method: http.MethodGet, path: "/admin/api/departments", want: false},
		{name: "department mutation no longer bypasses policy", method: http.MethodPut, path: "/admin/api/departments/:id", want: false},
		{name: "post list no longer bypasses policy", method: http.MethodGet, path: "/admin/api/posts", want: false},
		{name: "post mutation no longer bypasses policy", method: http.MethodDelete, path: "/admin/api/posts/:id", want: false},
		{name: "system config no longer bypasses policy", method: http.MethodGet, path: "/admin/api/system-configs", want: false},
		{name: "languages no longer bypasses policy", method: http.MethodGet, path: "/admin/api/languages", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isSelfServiceRequest(test.method, test.path); got != test.want {
				t.Fatalf("isSelfServiceRequest(%q, %q) = %t, want %t", test.method, test.path, got, test.want)
			}
		})
	}
}

func TestAuthorizeRequestDefaultsToDenyWithoutIdentityOrPolicyEngine(t *testing.T) {
	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = nil
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })

	selfContext := newAuthorizationTestContext(http.MethodGet, "/admin/api/user/userInfo")
	if authorizeRequest(nil, selfContext) {
		t.Fatal("self-service request without an authenticated identity must be denied")
	}
	if authorizeRequest(struct{}{}, selfContext) {
		t.Fatal("self-service request with an invalid identity type must be denied")
	}

	principal := &models.User{
		UserLogin: models.UserLogin{
			RoleID: "ordinary-role",
			Role:   &models.Role{Root: false},
		},
	}
	protectedContext := newAuthorizationTestContext(http.MethodGet, "/admin/api/audit-logs/login")
	if authorizeRequest(principal, protectedContext) {
		t.Fatal("protected request without a configured policy engine must be denied")
	}
	if !authorizeRequest(principal, selfContext) {
		t.Fatal("exact self-service request with a valid identity should not depend on Casbin")
	}
	root := &models.User{UserLogin: models.UserLogin{RoleID: "root-role", Role: &models.Role{Root: true}}}
	if !authorizeRequest(root, protectedContext) {
		t.Fatal("root identity should remain authorized when the policy engine is unavailable")
	}
}

func TestAuthorizeRequestRejectsPersonalAccessTokensForInteractiveSensitiveRoutes(t *testing.T) {
	routes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "password reset", method: http.MethodPost, path: "/admin/api/user/reset-password"},
		{name: "profile update", method: http.MethodPut, path: "/admin/api/user/userInfo"},
		{name: "OAuth binding", method: http.MethodPost, path: "/admin/api/user/binding"},
		{name: "OAuth unbinding", method: http.MethodDelete, path: "/admin/api/user/unbinding"},
		{name: "personal token list", method: http.MethodGet, path: "/admin/api/user-auth-tokens"},
		{name: "personal token create", method: http.MethodPost, path: "/admin/api/user-auth-tokens"},
		{name: "personal token revoke", method: http.MethodPut, path: "/admin/api/user-auth-token/:id/revoke"},
		{name: "personal token refresh", method: http.MethodPut, path: "/admin/api/user-auth-token/:id/refresh"},
	}
	tests := []struct {
		name      string
		principal *models.User
		want      bool
	}{
		{
			name: "ordinary session remains compatible",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID: "ordinary-role",
				Role:   &models.Role{},
			}},
			want: true,
		},
		{
			name: "personal access token value is denied",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID:            "ordinary-role",
				Role:              &models.Role{},
				PersonAccessToken: "pat-id",
			}},
			want: false,
		},
		{
			name: "non-refreshable token marker is denied",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID:              "ordinary-role",
				Role:                &models.Role{},
				RefreshTokenDisable: true,
			}},
			want: false,
		},
		{
			name: "root personal access token does not bypass the guard",
			principal: &models.User{UserLogin: models.UserLogin{
				RoleID:            "root-role",
				Role:              &models.Role{Root: true},
				PersonAccessToken: "root-pat-id",
			}},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					ctx := newAuthorizationTestContext(route.method, route.path)
					if got := authorizeRequest(test.principal, ctx); got != test.want {
						t.Fatalf("authorizeRequest() = %t, want %t", got, test.want)
					}
				})
			}
		})
	}
}

func TestAuthorizeRequestUsesExactRegisteredTemplateForPolicy(t *testing.T) {
	policyModel, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, tp, obj, act

[policy_definition]
p = sub, tp, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	if err != nil {
		t.Fatalf("create policy model: %v", err)
	}
	enforcer, err := casbin.NewEnforcer(policyModel)
	if err != nil {
		t.Fatalf("create policy enforcer: %v", err)
	}
	if _, err = enforcer.AddPolicy("operator-role", "API", "/admin/api/user/*/password-reset", http.MethodPut); err != nil {
		t.Fatalf("add policy: %v", err)
	}

	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = enforcer
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })
	principal := &models.User{UserLogin: models.UserLogin{RoleID: "operator-role", Role: &models.Role{}}}

	tests := []struct {
		name  string
		route string
		url   string
		want  bool
	}{
		{name: "exact protected route", route: "/admin/api/user/:userID/password-reset", url: "/admin/api/user/user-1/password-reset", want: true},
		{name: "different suffix", route: "/admin/api/user/:userID/password-delete", url: "/admin/api/user/user-1/password-delete", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got bool
			router := gin.New()
			router.PUT(test.route, func(c *gin.Context) {
				got = authorizeRequest(principal, c)
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, test.url, nil))
			if got != test.want {
				t.Fatalf("authorizeRequest() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRoutePolicyMatchesParametersBySegment(t *testing.T) {
	tests := []struct {
		route  string
		policy string
		want   bool
	}{
		{route: "/admin/api/users/:id", policy: "/admin/api/users/*", want: true},
		{route: "/admin/api/user/:userID/password-reset", policy: "/admin/api/user/*/password-reset", want: true},
		{route: "/admin/api/user/:userID/password-delete", policy: "/admin/api/user/*/password-reset", want: false},
		{route: "/admin/api/user/:userID/password-reset/export", policy: "/admin/api/user/*/password-reset", want: false},
		{route: "/admin/api/online-sessions/user/:userID", policy: "/admin/api/online-sessions/user/:userID", want: true},
	}
	for _, test := range tests {
		if got := routePolicyMatches(test.route, test.policy); got != test.want {
			t.Errorf("routePolicyMatches(%q, %q) = %t, want %t", test.route, test.policy, got, test.want)
		}
	}
}

func TestRequestHasCredentialRecognizesConfiguredSources(t *testing.T) {
	tests := []struct {
		name string
		url  string
		set  func(*http.Request)
		want bool
	}{
		{name: "anonymous", url: "/admin/api/app-configs/profile", want: false},
		{name: "authorization header", url: "/admin/api/app-configs/profile", set: func(r *http.Request) { r.Header.Set("Authorization", "Bearer token") }, want: true},
		{name: "query token", url: "/admin/api/app-configs/profile?token=value", want: true},
		{name: "jwt cookie", url: "/admin/api/app-configs/profile", set: func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "jwt", Value: "value"}) }, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, test.url, nil)
			if test.set != nil {
				test.set(ctx.Request)
			}
			if got := requestHasCredential(ctx); got != test.want {
				t.Fatalf("requestHasCredential() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestOptionalAuthAllowsAnonymousAndFailsClosedWhenCredentialCannotBeValidated(t *testing.T) {
	previousAuth := Auth
	Auth = nil
	t.Cleanup(func() { Auth = previousAuth })

	handled := 0
	router := gin.New()
	router.GET("/profile", OptionalAuth(), func(c *gin.Context) {
		handled++
		c.Status(http.StatusNoContent)
	})

	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if anonymous.Code != http.StatusNoContent || handled != 1 {
		t.Fatalf("anonymous optional request status=%d handled=%d, want 204/1", anonymous.Code, handled)
	}

	credentialedRequest := httptest.NewRequest(http.MethodGet, "/profile", nil)
	credentialedRequest.Header.Set("Authorization", "Bearer unavailable")
	credentialed := httptest.NewRecorder()
	router.ServeHTTP(credentialed, credentialedRequest)
	if credentialed.Code != http.StatusInternalServerError || handled != 1 {
		t.Fatalf("unvalidated credential status=%d handled=%d, want 500/1", credentialed.Code, handled)
	}
}

func TestGetVerifyUsesOnlyMiddlewareEstablishedIdentity(t *testing.T) {
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "test-auth-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	if got := GetVerify(nil); got != nil {
		t.Fatalf("GetVerify(nil) = %#v, want nil", got)
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/user/userInfo", nil)
	ctx.Request.Header.Set("Authorization", "Bearer forged-or-unestablished-token")
	if got := GetVerify(ctx); got != nil {
		t.Fatalf("identity parsed independently from bearer header: %#v", got)
	}

	ctx.Set(config.Cfg.Auth.IdentityKey, "not-a-verifier")
	if got := GetVerify(ctx); got != nil {
		t.Fatalf("invalid middleware identity = %#v, want nil", got)
	}

	principal := &models.User{
		UserLogin: models.UserLogin{
			RoleID: "ordinary-role",
			Role:   &models.Role{Root: false},
		},
	}
	principal.ID = "user-1"
	ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	if got := GetVerify(ctx); got != principal {
		t.Fatalf("GetVerify returned %#v, want established principal %#v", got, principal)
	}
}

func newAuthorizationTestContext(method, path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(method, path, nil)
	return ctx
}
