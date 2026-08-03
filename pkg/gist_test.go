package pkg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v88/github"
)

func Test_GistClone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gists/1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":{"hello.txt":{"filename":"hello.txt","content":"hello"}}}`))
	}))
	defer server.Close()

	baseURL := server.URL + "/"
	client, err := github.NewClient(
		github.WithHTTPClient(server.Client()),
		github.WithURLs(&baseURL, nil),
	)
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}

	dir := t.TempDir()
	if err := gistClone(t.Context(), client, "1", dir); err != nil {
		t.Fatalf("gistClone() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read cloned gist file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("cloned gist content = %q, want %q", string(got), "hello")
	}
}
