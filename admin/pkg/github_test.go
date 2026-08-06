package pkg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestGetUserFromGithubValidatesStatusAndIdentity(t *testing.T) {
	const accessToken = "test-access-token"
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantLogin  string
		wantError  string
	}{
		{
			name:       "valid identity",
			statusCode: http.StatusOK,
			body:       `{"id":42,"login":"octocat"}`,
			wantLogin:  "octocat",
		},
		{
			name:       "provider rejects token",
			statusCode: http.StatusUnauthorized,
			body:       `{"id":42,"login":"must-not-decode","message":"secret provider detail"}`,
			wantError:  "status 401",
		},
		{
			name:       "missing numeric identity",
			statusCode: http.StatusOK,
			body:       `{"id":0,"login":"octocat"}`,
			wantError:  "no valid identity",
		},
		{
			name:       "missing meaningful login",
			statusCode: http.StatusOK,
			body:       `{"id":42,"login":"   "}`,
			wantError:  "no valid identity",
		},
		{
			name:       "embedded whitespace login",
			statusCode: http.StatusOK,
			body:       `{"id":42,"login":"not a login"}`,
			wantError:  "no valid identity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/user", r.URL.Path)
				require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			ctx := WithGithubAPIBaseURL(context.Background(), server.URL)
			user, err := GetUserFromGithub(ctx, &oauth2.Config{}, accessToken)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, user)
				require.NotContains(t, err.Error(), accessToken)
				require.NotContains(t, err.Error(), "secret provider detail")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantLogin, user.Login)
			require.Equal(t, int64(42), user.ID)
		})
	}
}

func TestGetOrganizationsFromGithubRejectsProviderErrorsBeforeDecode(t *testing.T) {
	const accessToken = "test-access-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/user/orgs", r.URL.Path)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`[{"login":"allowed-org"}]`))
	}))
	defer server.Close()

	ctx := WithGithubAPIBaseURL(context.Background(), server.URL)
	organizations, err := GetOrganizationsFromGithub(ctx, &oauth2.Config{}, accessToken)
	require.ErrorContains(t, err, "status 403")
	require.Nil(t, organizations)
}

func TestGetOrganizationsFromGithubUsesBoundedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `[{"login":"org"}]`+strings.Repeat(" ", githubResponseMaxSize))
	}))
	defer server.Close()

	ctx := WithGithubAPIBaseURL(context.Background(), server.URL)
	organizations, err := GetOrganizationsFromGithub(ctx, &oauth2.Config{}, "token")
	require.ErrorContains(t, err, "exceeds size limit")
	require.Nil(t, organizations)
}

func TestGetOrganizationsFromGithubReturnsTrimmedNonEmptyLogins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"login":" org-one "},{"login":""},{"login":"Org-Two"}]`))
	}))
	defer server.Close()

	ctx := WithGithubAPIBaseURL(context.Background(), server.URL)
	organizations, err := GetOrganizationsFromGithub(ctx, &oauth2.Config{}, "token")
	require.NoError(t, err)
	require.Equal(t, []string{"org-one", "Org-Two"}, organizations)
}
