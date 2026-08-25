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

func TestResolveFrontendPackageUsesExactLoopbackMetadata(t *testing.T) {
	integrity := testFrontendIntegrity("candidate-tarball")
	tarball := "https://packages.example.test/artifacts/admin-web-1.3.3-frozen.tgz"
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
		_, _ = fmt.Fprintf(
			writer,
			`{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":%q,"tarball":%q}}`,
			integrity,
			tarball,
		)
	}))
	t.Cleanup(server.Close)

	got, err := resolveFrontendPackage(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3")
	if err != nil {
		t.Fatalf("resolveFrontendPackage() error = %v", err)
	}
	if got.Integrity != integrity || got.Tarball != tarball {
		t.Fatalf("resolveFrontendPackage() = %#v, want integrity %q and tarball %q", got, integrity, tarball)
	}
}

func TestResolveFrontendPackageFailsClosed(t *testing.T) {
	validIntegrity := testFrontendIntegrity("candidate-tarball")
	validTarball := "https://packages.example.test/artifacts/admin-web-1.3.3-frozen.tgz"
	for _, test := range []struct {
		name     string
		metadata string
		status   int
		want     string
	}{
		{name: "missing version", metadata: fmt.Sprintf(`{"name":"@mss-boot-io/admin-web","dist":{"integrity":%q,"tarball":%q}}`, validIntegrity, validTarball), status: http.StatusOK, want: "identity mismatch"},
		{name: "wrong package", metadata: fmt.Sprintf(`{"name":"other","version":"1.3.3","dist":{"integrity":%q,"tarball":%q}}`, validIntegrity, validTarball), status: http.StatusOK, want: "identity mismatch"},
		{name: "missing integrity", metadata: fmt.Sprintf(`{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"tarball":%q}}`, validTarball), status: http.StatusOK, want: "sha512 SRI"},
		{name: "invalid integrity", metadata: fmt.Sprintf(`{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":"sha512-short","tarball":%q}}`, validTarball), status: http.StatusOK, want: "sha512 SRI"},
		{name: "missing tarball", metadata: fmt.Sprintf(`{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":%q}}`, validIntegrity), status: http.StatusOK, want: "absolute stable URL"},
		{name: "malformed metadata", metadata: `{`, status: http.StatusOK, want: "decode npm metadata"},
		{name: "not found", metadata: `{}`, status: http.StatusNotFound, want: "HTTP 404"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.metadata))
			}))
			t.Cleanup(server.Close)
			_, err := resolveFrontendPackage(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveFrontendPackage() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveFrontendPackageRejectsUnsafeTarballURLs(t *testing.T) {
	integrity := testFrontendIntegrity("candidate-tarball")
	for _, test := range []struct {
		name    string
		tarball string
		want    string
	}{
		{name: "relative", tarball: "/artifacts/admin-web.tgz", want: "absolute stable URL"},
		{name: "invalid scheme", tarball: "ftp://packages.example.test/admin-web.tgz", want: "must use HTTPS"},
		{name: "non loopback HTTP", tarball: "http://packages.example.test/admin-web.tgz", want: "explicit loopback"},
		{name: "userinfo", tarball: "https://token@packages.example.test/admin-web.tgz", want: "credentials"},
		{name: "query", tarball: "https://packages.example.test/admin-web.tgz?token=secret", want: "query or fragment"},
		{name: "fragment", tarball: "https://packages.example.test/admin-web.tgz#fragment", want: "query or fragment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(
					writer,
					`{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":%q,"tarball":%q}}`,
					integrity,
					test.tarball,
				)
			}))
			t.Cleanup(server.Close)
			_, err := resolveFrontendPackage(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveFrontendPackage() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidFrontendTarballURLAllowsHTTPSAndExplicitLoopbackHTTP(t *testing.T) {
	for _, value := range []string{
		"https://packages.example.test/artifacts/admin-web.tgz",
		"http://localhost:4873/artifacts/admin-web.tgz",
		"http://127.0.0.1:4873/artifacts/admin-web.tgz",
		"http://[::1]:4873/artifacts/admin-web.tgz",
	} {
		if got, err := validFrontendTarballURL(value); err != nil || got != value {
			t.Fatalf("validFrontendTarballURL(%q) = %q, %v", value, got, err)
		}
	}
}

func TestResolveFrontendPackageRejectsOversizedMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxFrontendMetadataBytes+1)))
	}))
	t.Cleanup(server.Close)
	if _, err := resolveFrontendPackage(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized metadata error = %v", err)
	}
}

func TestResolveFrontendPackageRejectsRedirectAndNonLoopbackOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "https://registry.npmjs.org/", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	if _, err := resolveFrontendPackage(context.Background(), server.URL, "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "redirects are not allowed") {
		t.Fatalf("redirect error = %v", err)
	}
	if _, err := resolveFrontendPackage(context.Background(), "http://example.com", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback override error = %v", err)
	}
	if _, err := resolveFrontendPackage(context.Background(), "http://token@example.com", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("credential-bearing override error = %v", err)
	}
	if _, err := resolveFrontendPackage(context.Background(), "http://127.0.0.1/registry", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "a path") {
		t.Fatalf("path-bearing override error = %v", err)
	}
	if _, err := resolveFrontendPackage(context.Background(), "http://127.0.0.1?token=secret", "@mss-boot-io/admin-web", "1.3.3"); err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("query-bearing override error = %v", err)
	}
}

func TestResolveFrontendPackageForSourceSkipsTemplatesWithoutTokens(t *testing.T) {
	got, err := resolveFrontendPackageForSource(context.Background(), "http://example.com", nil, []blueprintSourceFile{{Data: []byte("no frontend package tokens")}})
	if err != nil || got != (frontendPackageResolution{}) {
		t.Fatalf("resolveFrontendPackageForSource() = %#v, %v", got, err)
	}
}

func TestResolveFrontendPackageForSourceRequiresTarballAndIntegrityTogether(t *testing.T) {
	for _, token := range []string{frontendIntegrityToken, frontendTarballToken} {
		_, err := resolveFrontendPackageForSource(context.Background(), "http://example.com", nil, []blueprintSourceFile{{Data: []byte(token)}})
		if err == nil || !strings.Contains(err.Error(), "tarball and integrity together") {
			t.Fatalf("single frontend package token %q error = %v", token, err)
		}
	}
}

func testFrontendIntegrity(content string) string {
	digest := sha512.Sum512([]byte(content))
	return "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
}
