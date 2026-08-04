package pkg

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGetLatestVersionContext(t *testing.T) {
	previousURL := latestReleaseURL
	previousClient := httpClient
	t.Cleanup(func() {
		latestReleaseURL = previousURL
		httpClient = previousClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"tag_name":"v1.2.3"}`)
	}))
	defer server.Close()
	latestReleaseURL = server.URL
	httpClient = server.Client()

	version, err := GetLatestVersionContext(context.Background())
	if err != nil || version != "v1.2.3" {
		t.Fatalf("version=%q error=%v", version, err)
	}
	if got := GetLatestVersion(); got != "v1.2.3" {
		t.Fatalf("compatibility version = %q", got)
	}
}

func TestGetLatestVersionContextErrors(t *testing.T) {
	previousURL := latestReleaseURL
	previousClient := httpClient
	t.Cleanup(func() {
		latestReleaseURL = previousURL
		httpClient = previousClient
	})

	tests := []struct {
		name string
		body string
		code int
		want string
	}{
		{name: "HTTP failure", code: http.StatusBadGateway, body: "bad gateway", want: "HTTP 502"},
		{name: "invalid JSON", code: http.StatusOK, body: "{", want: "decode latest release"},
		{name: "missing tag", code: http.StatusOK, body: `{}`, want: "no tag_name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.code)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			latestReleaseURL = server.URL
			httpClient = server.Client()
			if _, err := GetLatestVersionContext(nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}

	httpClient = roundTripClient(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})
	latestReleaseURL = "https://example.invalid/latest"
	if _, err := GetLatestVersionContext(context.Background()); err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("network error = %v", err)
	}
	if got := GetLatestVersion(); got != "" {
		t.Fatalf("compatibility failure result = %q", got)
	}
}

func TestCopyDirAndPathHelpers(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "nested", "destination")
	if err := os.MkdirAll(filepath.Join(source, "sub"), 0o750); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "root.txt"), []byte("root"), 0o640); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "sub", "child.txt"), []byte("child"), 0o600); err != nil {
		t.Fatalf("write child file: %v", err)
	}
	if err := CopyDir(source, destination); err != nil {
		t.Fatalf("copy directory: %v", err)
	}
	for relative, expected := range map[string]string{
		"root.txt":      "root",
		"sub/child.txt": "child",
	} {
		data, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read copied %s: %v", relative, err)
		}
		if string(data) != expected {
			t.Fatalf("copied %s = %q", relative, data)
		}
	}
	if exists, err := pathExists(destination); err != nil || !exists {
		t.Fatalf("existing path result exists=%t error=%v", exists, err)
	}
	if exists, err := pathExists(filepath.Join(destination, "missing")); err != nil || exists {
		t.Fatalf("missing path result exists=%t error=%v", exists, err)
	}

	fileSource := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(fileSource, []byte("copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileDestination := filepath.Join(t.TempDir(), "a", "b", "destination.txt")
	written, err := copyFile(fileSource, fileDestination)
	if err != nil || written != 4 {
		t.Fatalf("copyFile written=%d error=%v", written, err)
	}
	if err := CopyDir(fileSource, filepath.Join(t.TempDir(), "destination")); err == nil {
		t.Fatal("CopyDir accepted a file source")
	}
}

func TestCommonStringAndPlatformHelpers(t *testing.T) {
	if IsWindows() {
		if GetInstallPath() != `C:\Program Files\nps` || GetTmpPath() == "" {
			t.Fatalf("Windows paths install=%q temp=%q", GetInstallPath(), GetTmpPath())
		}
	} else if GetInstallPath() != "/etc/nps" || GetTmpPath() != "/tmp" {
		t.Fatalf("Unix paths install=%q temp=%q", GetInstallPath(), GetTmpPath())
	}
	if GetAppPath() == "" {
		t.Fatal("application path is empty")
	}
	if !InArray([]string{"ADMIN"}, []string{"prefix-admin"}, "prefix-", 1) {
		t.Fatal("InArray did not apply case-insensitive replacement")
	}
	if InArray([]string{"missing"}, []string{"admin"}, "", -1) {
		t.Fatal("InArray reported an absent value")
	}

	cases := map[string]string{
		"":      "",
		"a":     "as",
		"boy":   "boys",
		"city":  "cities",
		"box":   "boxes",
		"class": "classes",
		"hero":  "heroes",
		"church": "churches",
		"roof":  "rooves",
		"staff": "staves",
		"post":  "posts",
	}
	for singular, plural := range cases {
		if got := Pluralize(singular); got != plural {
			t.Fatalf("Pluralize(%q) = %q, want %q", singular, got, plural)
		}
	}

	if got := []bool{IsWindows(), !IsWindows()}; reflect.DeepEqual(got, []bool{true, true}) {
		t.Fatal("platform predicates are inconsistent")
	}
}

type roundTripClient func(*http.Request) (*http.Response, error)

func (f roundTripClient) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}
