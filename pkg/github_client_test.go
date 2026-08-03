package pkg

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v88/github"
)

func TestNewGitHubClientAppliesTokenAndCustomEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		if r.URL.Path != "/repos/acme/example/branches" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	baseURL := server.URL + "/"
	client, err := newGitHubClient(
		"test-token",
		github.WithHTTPClient(server.Client()),
		github.WithURLs(&baseURL, nil),
	)
	if err != nil {
		t.Fatalf("newGitHubClient() error = %v", err)
	}

	if _, _, err := client.Repositories.ListBranches(t.Context(), "acme", "example", nil); err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
}
