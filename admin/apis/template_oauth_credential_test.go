package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	git "github.com/go-git/go-git/v5"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthcredential"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthstate"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"github.com/redis/go-redis/v9"
)

const (
	templateTestHandle      = "opaque-oauth-credential-handle"
	templateTestCredential  = "interactive-admin-session"
	templateTestAccessToken = "github-provider-access-token"
)

type templateTestCredentialStore struct {
	record       oauthcredential.Record
	lookupErr    error
	deleteErr    error
	lookupCount  int
	deleteCount  int
	lookupHandle string
	deleteHandle string
	onDelete     func()
}

func (s *templateTestCredentialStore) Issue(
	context.Context,
	redis.UniversalClient,
	oauthcredential.Record,
	time.Duration,
) (string, oauthcredential.Record, error) {
	return "", oauthcredential.Record{}, errors.New("unexpected Issue call")
}

func (s *templateTestCredentialStore) Lookup(
	_ context.Context,
	_ redis.UniversalClient,
	handle string,
) (oauthcredential.Record, error) {
	s.lookupCount++
	s.lookupHandle = handle
	return s.record, s.lookupErr
}

func (s *templateTestCredentialStore) Delete(
	_ context.Context,
	_ redis.UniversalClient,
	handle string,
) error {
	s.deleteCount++
	s.deleteHandle = handle
	if s.onDelete != nil {
		s.onDelete()
	}
	return s.deleteErr
}

func TestTemplateOAuthCredentialLookupIsSessionBound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installTemplateTestVerifier(t)

	validRecord := oauthcredential.Record{
		Provider:              "github",
		Intent:                oauthcredential.IntentIntegration,
		UserID:                "user-1",
		CredentialFingerprint: oauthstate.Digest(templateTestCredential),
		AccessToken:           templateTestAccessToken,
		ExpiresAt:             time.Now().Add(time.Minute),
	}
	tests := []struct {
		name           string
		header         string
		credential     string
		verifier       security.Verifier
		mutate         func(*templateTestCredentialStore)
		wantStatus     int
		wantToken      string
		wantHandle     string
		wantLookupCall int
	}{
		{
			name:           "correct binding",
			header:         templateTestHandle,
			credential:     templateTestCredential,
			verifier:       templateTestUser("user-1", false),
			wantToken:      templateTestAccessToken,
			wantHandle:     templateTestHandle,
			wantLookupCall: 1,
		},
		{
			name:           "missing handle uses public token",
			credential:     templateTestCredential,
			verifier:       templateTestUser("user-1", false),
			wantLookupCall: 0,
		},
		{
			name:       "expired handle",
			header:     templateTestHandle,
			credential: templateTestCredential,
			verifier:   templateTestUser("user-1", false),
			mutate: func(store *templateTestCredentialStore) {
				store.lookupErr = oauthcredential.ErrExpired
			},
			wantStatus:     http.StatusUnauthorized,
			wantLookupCall: 1,
		},
		{
			name:           "wrong user",
			header:         templateTestHandle,
			credential:     templateTestCredential,
			verifier:       templateTestUser("user-2", false),
			wantStatus:     http.StatusUnauthorized,
			wantLookupCall: 1,
		},
		{
			name:           "wrong credential fingerprint",
			header:         templateTestHandle,
			credential:     "different-admin-session",
			verifier:       templateTestUser("user-1", false),
			wantStatus:     http.StatusUnauthorized,
			wantLookupCall: 1,
		},
		{
			name:           "personal access token",
			header:         templateTestHandle,
			credential:     "personal-access-token",
			verifier:       templateTestUser("user-1", true),
			wantStatus:     http.StatusForbidden,
			wantLookupCall: 0,
		},
		{
			name:       "wrong provider",
			header:     templateTestHandle,
			credential: templateTestCredential,
			verifier:   templateTestUser("user-1", false),
			mutate: func(store *templateTestCredentialStore) {
				store.record.Provider = "lark"
			},
			wantStatus:     http.StatusUnauthorized,
			wantLookupCall: 1,
		},
		{
			name:       "wrong intent",
			header:     templateTestHandle,
			credential: templateTestCredential,
			verifier:   templateTestUser("user-1", false),
			mutate: func(store *templateTestCredentialStore) {
				store.record.Intent = "login"
			},
			wantStatus:     http.StatusUnauthorized,
			wantLookupCall: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &templateTestCredentialStore{record: validRecord}
			if test.mutate != nil {
				test.mutate(store)
			}
			ctx := templateTestContext(test.credential, test.header, test.verifier)
			token, handle, status := (Template{oauthCredentials: store}).lookupOAuthIntegrationCredential(ctx)
			if status != test.wantStatus || token != test.wantToken || handle != test.wantHandle {
				t.Fatalf("lookup = (%q, %q, %d), want (%q, %q, %d)",
					token, handle, status, test.wantToken, test.wantHandle, test.wantStatus)
			}
			if store.lookupCount != test.wantLookupCall {
				t.Fatalf("Lookup calls = %d, want %d", store.lookupCount, test.wantLookupCall)
			}
			if store.lookupCount > 0 && store.lookupHandle != templateTestHandle {
				t.Fatalf("Lookup handle = %q, want supplied opaque handle", store.lookupHandle)
			}
		})
	}
}

func TestTemplateRejectsLegacyAccessTokenInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &templateTestCredentialStore{}
	template := Template{
		oauthCredentials: store,
		gitClone: func(string, string, string, bool, string) (*git.Repository, error) {
			t.Fatal("legacy accessToken reached GitClone")
			return nil, nil
		},
	}

	t.Run("GET query", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodGet,
			"/template/get-branches?source=https://github.com/example/template&accessToken=legacy-provider-token",
			nil,
		)
		recorder := executeTemplateHandler(request, nil, template.GetBranches)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("POST JSON body", func(t *testing.T) {
		body := validTemplateGenerateBody(t, "legacy-provider-token")
		request := httptest.NewRequest(http.MethodPost, "/template/generate", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := executeTemplateHandler(request, nil, template.Generate)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	if store.lookupCount != 0 {
		t.Fatalf("legacy inputs caused %d credential lookups, want zero", store.lookupCount)
	}
}

func TestTemplateGenerateDeletesCredentialAfterSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installTemplateTestVerifier(t)
	events := make([]string, 0, 5)
	store := &templateTestCredentialStore{
		record: oauthcredential.Record{
			Provider:              "github",
			Intent:                oauthcredential.IntentIntegration,
			UserID:                "user-1",
			CredentialFingerprint: oauthstate.Digest(templateTestCredential),
			AccessToken:           templateTestAccessToken,
			ExpiresAt:             time.Now().Add(time.Minute),
		},
		onDelete: func() { events = append(events, "delete") },
	}
	cloneCalls := 0
	template := Template{
		oauthCredentials: store,
		gitClone: func(_ string, _ string, _ string, _ bool, accessToken string) (*git.Repository, error) {
			cloneCalls++
			if accessToken != templateTestAccessToken {
				t.Fatalf("GitClone access token = %q, want resolved provider token", accessToken)
			}
			events = append(events, "clone")
			return nil, nil
		},
		generate: func(*pkg.TemplateConfig) error {
			events = append(events, "generate")
			return nil
		},
		commitAndPush: func(_ string, _ string, _ string, accessToken string, auth *gitHttp.BasicAuth) error {
			if accessToken != templateTestAccessToken || auth == nil || auth.Password != templateTestAccessToken {
				t.Fatalf("push credentials were not resolved server-side: token=%q auth=%#v", accessToken, auth)
			}
			events = append(events, "push")
			return nil
		},
		cleanup: func(...string) {},
	}

	body := validTemplateGenerateBody(t, "")
	request := httptest.NewRequest(http.MethodPost, "/template/generate", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+templateTestCredential)
	request.Header.Set(oauthCredentialHeader, templateTestHandle)
	recorder := executeTemplateHandler(request, templateTestUser("user-1", false), template.Generate)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	if cloneCalls != 2 || store.lookupCount != 1 || store.deleteCount != 1 {
		t.Fatalf("calls: clone=%d lookup=%d delete=%d, want 2/1/1", cloneCalls, store.lookupCount, store.deleteCount)
	}
	if store.deleteHandle != templateTestHandle {
		t.Fatalf("Delete handle = %q, want supplied handle", store.deleteHandle)
	}
	if len(events) != 5 || events[4] != "delete" {
		t.Fatalf("event order = %#v, want credential deletion after successful push", events)
	}
	if strings.Contains(recorder.Body.String(), templateTestAccessToken) ||
		strings.Contains(recorder.Body.String(), templateTestHandle) {
		t.Fatalf("response leaked integration credential: %s", recorder.Body.String())
	}
}

func validTemplateGenerateBody(t *testing.T, legacyAccessToken string) []byte {
	t.Helper()
	payload := map[string]any{
		"template": map[string]any{
			"source": "https://github.com/example/template",
			"branch": "main",
			"path":   ".",
		},
		"generate": map[string]any{
			"repo":   "https://github.com/example/generated",
			"params": map[string]string{},
		},
	}
	if legacyAccessToken != "" {
		payload["accessToken"] = legacyAccessToken
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return body
}

func installTemplateTestVerifier(t *testing.T) {
	t.Helper()
	previous := response.VerifyHandler
	response.VerifyHandler = func(c *gin.Context) security.Verifier {
		value, exists := c.Get("template-test-verifier")
		if !exists {
			return nil
		}
		verifier, _ := value.(security.Verifier)
		return verifier
	}
	t.Cleanup(func() { response.VerifyHandler = previous })
}

func templateTestUser(userID string, personalAccessToken bool) *models.User {
	user := &models.User{}
	user.ID = userID
	if personalAccessToken {
		user.RefreshTokenDisable = true
		user.PersonAccessToken = "pat"
	}
	return user
}

func templateTestContext(credential, handle string, verifier security.Verifier) *gin.Context {
	request := httptest.NewRequest(http.MethodGet, "/template", nil)
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	if handle != "" {
		request.Header.Set(oauthCredentialHeader, handle)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	if verifier != nil {
		ctx.Set("template-test-verifier", verifier)
	}
	return ctx
}

func executeTemplateHandler(
	request *http.Request,
	verifier security.Verifier,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if verifier != nil {
			c.Set("template-test-verifier", verifier)
		}
		c.Next()
	})
	router.Handle(request.Method, request.URL.Path, handler)
	router.ServeHTTP(recorder, request)
	return recorder
}
