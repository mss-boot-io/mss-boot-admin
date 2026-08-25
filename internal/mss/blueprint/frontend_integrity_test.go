package blueprint

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveFrontendIntegrityUsesExactLoopbackMetadata(t *testing.T) {
	integrity := testFrontendIntegrity("candidate-tarball")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("metadata method = %s", request.Method)
		}
		if request.URL.Path != "/@mss-boot-io/admin-web/1.3.3" {
			t.Errorf("metadata path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Error("metadata request unexpectedly forwarded credentials")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":%q}}`, integrity)
	}))
	t.Cleanup(server.Close)

	got, err := resolveFrontendIntegrity(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3")
	if err != nil {
		t.Fatalf("resolveFrontendIntegrity() error = %v", err)
	}
	if got != integrity {
		t.Fatalf("resolveFrontendIntegrity() = %q, want %q", got, integrity)
	}
}

func TestResolveFrontendIntegrityFailsClosed(t *testing.T) {
	validIntegrity := testFrontendIntegrity("candidate-tarball")
	for _, test := range []struct {
		name     string
		metadata string
		status   int
		want     string
	}{
		{name: "missing version", metadata: `{"name":"@mss-boot-io/admin-web","dist":{"integrity":"` + validIntegrity + `"}}`, status: http.StatusOK, want: "identity mismatch"},
		{name: "wrong package", metadata: `{"name":"other","version":"1.3.3","dist":{"integrity":"` + validIntegrity + `"}}`, status: http.StatusOK, want: "identity mismatch"},
		{name: "missing integrity", metadata: `{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{}}`, status: http.StatusOK, want: "sha512 SRI"},
		{name: "invalid integrity", metadata: `{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":"sha512-short"}}`, status: http.StatusOK, want: "sha512 SRI"},
		{name: "malformed metadata", metadata: `{`, status: http.StatusOK, want: "decode npm metadata"},
		{name: "not found", metadata: `{}`, status: http.StatusNotFound, want: "HTTP 404"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.metadata))
			}))
			t.Cleanup(server.Close)
			_, err := resolveFrontendIntegrity(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveFrontendIntegrity() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveFrontendIntegrityRejectsOversizedMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxFrontendMetadataBytes+1)))
	}))
	t.Cleanup(server.Close)
	if _, err := resolveFrontendIntegrity(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized metadata error = %v", err)
	}
}

func TestResolveFrontendIntegrityRejectsRedirectAndNonLoopbackOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "https://registry.npmjs.org/", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	if _, err := resolveFrontendIntegrity(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "redirects are not allowed") {
		t.Fatalf("redirect error = %v", err)
	}
	if _, err := resolveFrontendIntegrity(context.Background(), "http://example.com", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback override error = %v", err)
	}
	if _, err := resolveFrontendIntegrity(context.Background(), "http://token@example.com", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("credential-bearing override error = %v", err)
	}
	if _, err := resolveFrontendIntegrity(context.Background(), "http://127.0.0.1/registry", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "a path") {
		t.Fatalf("path-bearing override error = %v", err)
	}
	if _, err := resolveFrontendIntegrity(context.Background(), "http://127.0.0.1?token=secret", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("query-bearing override error = %v", err)
	}
}

func TestResolveFrontendIntegrityForSourceSkipsTemplatesWithoutToken(t *testing.T) {
	got, err := resolveFrontendIntegrityForSource(context.Background(), "http://example.com", nil, []blueprintSourceFile{{Data: []byte("no frontend integrity token")}})
	if err != nil || got != "" {
		t.Fatalf("resolveFrontendIntegrityForSource() = %q, %v", got, err)
	}
}

func testFrontendIntegrity(content string) string {
	digest := sha512.Sum512([]byte(content))
	return "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
}
