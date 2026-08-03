package poster

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetResourceReaderHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("image-data"))
	}))
	defer server.Close()

	reader, err := getResourceReader(server.URL)
	if err != nil {
		t.Fatalf("getResourceReader() error = %v", err)
	}
	if got := reader.String(); got != "image-data" {
		t.Fatalf("resource content = %q, want %q", got, "image-data")
	}
}

func TestGetResourceReaderRejectsUntrustedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("image-data"))
	}))
	defer server.Close()

	if _, err := getResourceReader(server.URL); err == nil {
		t.Fatal("getResourceReader() accepted an untrusted TLS certificate")
	}
}

func TestGetResourceReaderRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	if _, err := getResourceReader(server.URL); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("getResourceReader() error = %v, want HTTP 404 error", err)
	}
}

func TestGetResourceReaderLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.bin")
	if err := os.WriteFile(path, []byte("local-image"), 0o600); err != nil {
		t.Fatalf("write local image: %v", err)
	}

	reader, err := getResourceReader(path)
	if err != nil {
		t.Fatalf("getResourceReader() error = %v", err)
	}
	if !bytes.Equal(reader.Bytes(), []byte("local-image")) {
		t.Fatalf("local resource content = %q", reader.Bytes())
	}
}

func TestReadImageResourceRejectsOversizedContent(t *testing.T) {
	content := bytes.Repeat([]byte{'x'}, int(maxImageResourceBytes)+1)
	if _, err := readImageResource(bytes.NewReader(content)); err == nil {
		t.Fatal("readImageResource() accepted oversized content")
	}
}
