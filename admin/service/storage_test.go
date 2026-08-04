package service

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
)

type fakeAppConfig struct {
	values map[string]string
}

func (f *fakeAppConfig) SetAppConfig(_ *gin.Context, key string, _ bool, value string) error {
	if f.values == nil {
		f.values = make(map[string]string)
	}
	f.values[key] = value
	return nil
}

func (f *fakeAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	value, ok := f.values[key]
	return value, ok
}

func TestStorageUploadLocalAcceptsShortTextAndSanitizesFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	center.SetAppConfig(nil)
	t.Cleanup(func() { center.SetAppConfig(previous) })

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	temporary := t.TempDir()
	if err := os.Chdir(temporary); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	context, header := multipartFile(t, "hello world.txt", "text/plain; charset=utf-8", []byte("hello"))
	result, err := (&Storage{}).Upload(context, header, "user-1")
	if err != nil {
		t.Fatalf("upload text file: %v", err)
	}
	if result.URL != "/public/user-1/hello_world.txt" {
		t.Fatalf("URL = %q", result.URL)
	}
	if result.Filename != "hello world.txt" || result.Size != 5 || result.MimeType != "text/plain" {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join("public", "user-1", "hello_world.txt"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("uploaded contents = %q", data)
	}
}

func TestStorageValidateRejectsInvalidConfigurationAndContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	t.Cleanup(func() { center.SetAppConfig(previous) })

	tests := []struct {
		name        string
		values      map[string]string
		contentType string
		data        []byte
		want        string
	}{
		{
			name:        "invalid maximum size",
			values:      map[string]string{maxSizeConfigKey: "not-a-number"},
			contentType: "text/plain",
			data:        []byte("hello"),
			want:        "invalid maximum file size",
		},
		{
			name:        "file too large",
			values:      map[string]string{maxSizeConfigKey: "4"},
			contentType: "text/plain",
			data:        []byte("hello"),
			want:        "exceeds maximum allowed size",
		},
		{
			name:        "declared type forbidden",
			values:      map[string]string{allowedTypesConfigKey: "text/plain"},
			contentType: "application/octet-stream",
			data:        []byte{0, 1, 2, 3},
			want:        "file type application/octet-stream is not allowed",
		},
		{
			name:        "detected type forbidden",
			values:      map[string]string{allowedTypesConfigKey: "text/plain"},
			contentType: "text/plain",
			data:        []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
			want:        "detected file type image/png is not allowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			center.SetAppConfig(&fakeAppConfig{values: test.values})
			context, header := multipartFile(t, "fixture.bin", test.contentType, test.data)
			err := (&Storage{}).validate(context, header)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestStorageValidateSupportsWildcardMediaTypes(t *testing.T) {
	previous := center.GetAppConfig()
	center.SetAppConfig(&fakeAppConfig{values: map[string]string{
		allowedTypesConfigKey: " image/* , application/pdf ",
	}})
	t.Cleanup(func() { center.SetAppConfig(previous) })

	context, header := multipartFile(
		t,
		"image.png",
		"image/png",
		[]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
	)
	if err := (&Storage{}).validate(context, header); err != nil {
		t.Fatalf("validate wildcard image: %v", err)
	}
	if !(&Storage{}).isAllowedType("IMAGE/PNG; charset=binary", []string{"image/*"}) {
		t.Fatal("wildcard media type did not match normalized content type")
	}
}

func TestStorageValidationHelpers(t *testing.T) {
	storage := &Storage{}
	if err := storage.validate(&gin.Context{}, nil); err == nil || err.Error() != "file is required" {
		t.Fatalf("nil file error = %v", err)
	}
	if got := storage.sanitizeFilename("../ unsafe name.txt"); got != "_unsafe_name.txt" {
		t.Fatalf("sanitized filename = %q", got)
	}
	if got := normalizeMediaType("Text/Plain; Charset=UTF-8"); got != "text/plain" {
		t.Fatalf("normalized media type = %q", got)
	}
	if got := normalizeMediaType(" IMAGE/* "); got != "image/*" {
		t.Fatalf("normalized wildcard = %q", got)
	}
}

func multipartFile(t *testing.T, filename, contentType string, data []byte) (*gin.Context, *multipart.FileHeader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	files := request.MultipartForm.File["file"]
	if len(files) != 1 {
		t.Fatalf("multipart file count = %d", len(files))
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	return context, files[0]
}
