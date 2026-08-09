package service

import (
	"bytes"
	stdcontext "context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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

func TestStorageLocalNoClobberAndConfinement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	center.SetAppConfig(&fakeAppConfig{values: map[string]string{
		"storage:type":        "local",
		maxSizeConfigKey:      "1024",
		allowedTypesConfigKey: "text/plain",
	}})
	t.Cleanup(func() { center.SetAppConfig(previous) })

	t.Run("opaque keys keep same-name uploads distinct", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		storage := &Storage{}

		first := uploadText(t, storage, "../shared name.txt", []byte("first"))
		second := uploadText(t, storage, "../shared name.txt", []byte("second"))
		if first.URL == second.URL {
			t.Fatalf("same-name uploads reused URL %q", first.URL)
		}
		for _, result := range []*UploadResult{first, second} {
			if !strings.HasPrefix(result.URL, "/public/uploads/") {
				t.Fatalf("URL %q does not use the fixed opaque prefix", result.URL)
			}
			identifier := strings.TrimPrefix(result.URL, "/public/uploads/")
			if _, err := uuid.Parse(identifier); err != nil {
				t.Fatalf("URL identifier %q is not a UUID: %v", identifier, err)
			}
			if strings.Contains(result.URL, "shared") || strings.Contains(result.URL, "name") {
				t.Fatalf("URL leaked original filename: %q", result.URL)
			}
		}
		assertFileContents(t, filepath.Join(workingDirectory, strings.TrimPrefix(first.URL, "/")), "first")
		assertFileContents(t, filepath.Join(workingDirectory, strings.TrimPrefix(second.URL, "/")), "second")
	})

	t.Run("create-only collision preserves the first object", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		fixedID := "11111111-1111-4111-8111-111111111111"
		storage := &Storage{generateObjectID: func() (string, error) { return fixedID, nil }}
		first := uploadText(t, storage, "first.txt", []byte("sentinel"))

		context, _ := multipartRequestContext(t, "second.txt", "text/plain", []byte("replacement"))
		_, err := storage.Upload(context, "file")
		if !errors.Is(err, ErrObjectConflict) {
			t.Fatalf("collision error = %v, want ErrObjectConflict", err)
		}
		assertFileContents(t, filepath.Join(workingDirectory, strings.TrimPrefix(first.URL, "/")), "sentinel")
	})

	t.Run("concurrent create-only collision publishes one complete object", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		fixedID := "22222222-2222-4222-8222-222222222222"
		storage := &Storage{generateObjectID: func() (string, error) { return fixedID, nil }}
		firstContext, _ := multipartRequestContext(t, "same.txt", "text/plain", []byte("first"))
		secondContext, _ := multipartRequestContext(t, "same.txt", "text/plain", []byte("second"))
		type outcome struct {
			result *UploadResult
			err    error
		}
		start := make(chan struct{})
		outcomes := make(chan outcome, 2)
		for _, requestContext := range []*gin.Context{firstContext, secondContext} {
			go func(requestContext *gin.Context) {
				<-start
				result, err := storage.Upload(requestContext, "file")
				outcomes <- outcome{result: result, err: err}
			}(requestContext)
		}
		close(start)
		var succeeded, conflicted int
		for range 2 {
			result := <-outcomes
			switch {
			case result.err == nil:
				succeeded++
			case errors.Is(result.err, ErrObjectConflict):
				conflicted++
			default:
				t.Fatalf("unexpected concurrent outcome: result=%#v err=%v", result.result, result.err)
			}
		}
		if succeeded != 1 || conflicted != 1 {
			t.Fatalf("concurrent outcomes: succeeded=%d conflicted=%d", succeeded, conflicted)
		}
		filename := filepath.Join(workingDirectory, "public", "uploads", fixedID)
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read concurrent object: %v", err)
		}
		if string(data) != "first" && string(data) != "second" {
			t.Fatalf("concurrent object is partial/corrupt: %q", data)
		}
	})

	t.Run("invalid generated identifier cannot become a path", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		storage := &Storage{generateObjectID: func() (string, error) { return "../../escape", nil }}
		context, _ := multipartRequestContext(t, "safe.txt", "text/plain", []byte("safe"))
		_, err := storage.Upload(context, "file")
		if !errors.Is(err, ErrUnsafeObjectPath) {
			t.Fatalf("invalid key error = %v, want ErrUnsafeObjectPath", err)
		}
		assertNoFiles(t, workingDirectory)
	})

	t.Run("external root symlink is rejected", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(workingDirectory, "public")); err != nil {
			t.Fatalf("create external root symlink: %v", err)
		}
		context, _ := multipartRequestContext(t, "safe.txt", "text/plain", []byte("safe"))
		_, err := (&Storage{}).Upload(context, "file")
		if !errors.Is(err, ErrUnsafeObjectPath) {
			t.Fatalf("symlink error = %v, want ErrUnsafeObjectPath", err)
		}
		assertNoFiles(t, outside)
	})

	t.Run("internal directory symlink escape is rejected", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		outside := t.TempDir()
		if err := os.Mkdir(filepath.Join(workingDirectory, "public"), 0o750); err != nil {
			t.Fatalf("create public root: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(workingDirectory, "public", "uploads")); err != nil {
			t.Fatalf("create internal directory symlink: %v", err)
		}
		context, _ := multipartRequestContext(t, "safe.txt", "text/plain", []byte("safe"))
		_, err := (&Storage{}).Upload(context, "file")
		if !errors.Is(err, ErrUnsafeObjectPath) {
			t.Fatalf("internal symlink error = %v, want ErrUnsafeObjectPath", err)
		}
		assertNoFiles(t, outside)
	})

	t.Run("stream max plus one defeats a forged header size", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		context, header := parsedMultipartFile(t, "forged.txt", "text/plain", []byte("12345"))
		header.Size = 4
		upload := &admittedUpload{
			header: header,
			policy: uploadPolicy{maxBytes: 4, allowedTypes: []string{"text/plain"}},
			form:   context.Request.MultipartForm,
		}
		defer upload.close()
		_, err := (&Storage{}).store(context, upload)
		if !errors.Is(err, ErrUploadTooLarge) {
			t.Fatalf("forged size error = %v, want ErrUploadTooLarge", err)
		}
		assertNoFiles(t, workingDirectory)
	})

	t.Run("cancellation removes a partial destination", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		context, header := parsedMultipartFile(t, "cancel.txt", "text/plain", []byte("cancelled"))
		requestContext, cancel := stdcontext.WithCancel(context.Request.Context())
		cancel()
		context.Request = context.Request.WithContext(requestContext)
		upload := &admittedUpload{
			header: header,
			policy: uploadPolicy{maxBytes: 1024, allowedTypes: []string{"text/plain"}},
			form:   context.Request.MultipartForm,
		}
		defer upload.close()
		_, err := (&Storage{}).store(context, upload)
		if !errors.Is(err, stdcontext.Canceled) {
			t.Fatalf("canceled write error = %v, want context.Canceled", err)
		}
		assertNoFiles(t, filepath.Join(workingDirectory, "public", "uploads"))
	})

	t.Run("cancellation after bytes are written removes the partial destination", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		requestContext, cancel := stdcontext.WithCancel(stdcontext.Background())
		reader := &cancelUploadAfterRead{
			data:   []byte("partial"),
			cancel: cancel,
		}
		metadata := inspectedUpload{
			size:      int64(len(reader.data)),
			mimeType:  "text/plain",
			objectKey: "uploads/33333333-3333-4333-8333-333333333333",
		}
		_, err := (&Storage{}).writeLocalObject(requestContext, reader, 1024, metadata)
		if !errors.Is(err, stdcontext.Canceled) {
			t.Fatalf("partial cancellation error = %v, want context.Canceled", err)
		}
		if reader.reads != 1 {
			t.Fatalf("partial reader calls = %d, want one", reader.reads)
		}
		assertNoFiles(t, filepath.Join(workingDirectory, "public", "uploads"))
	})

	t.Run("exact object limit succeeds with restricted permissions", func(t *testing.T) {
		workingDirectory := enterTemporaryWorkingDirectory(t)
		center.SetAppConfig(&fakeAppConfig{values: map[string]string{
			"storage:type":        "local",
			maxSizeConfigKey:      "5",
			allowedTypesConfigKey: "text/plain",
		}})
		result := uploadText(t, &Storage{}, "exact.txt", []byte("12345"))
		filename := filepath.Join(workingDirectory, strings.TrimPrefix(result.URL, "/"))
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatalf("stat exact-limit object: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("object permissions = %o, want 600", info.Mode().Perm())
		}
	})
}

func TestStoragePolicyRejectsInvalidConfigurationAndContent(t *testing.T) {
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
			name:        "maximum size exceeds hard ceiling",
			values:      map[string]string{maxSizeConfigKey: "104857601"},
			contentType: "text/plain",
			data:        []byte("hello"),
			want:        "invalid maximum file size",
		},
		{
			name:        "file too large",
			values:      map[string]string{maxSizeConfigKey: "4"},
			contentType: "text/plain",
			data:        []byte("hello"),
			want:        ErrUploadTooLarge.Error(),
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
			enterTemporaryWorkingDirectory(t)
			center.SetAppConfig(&fakeAppConfig{values: test.values})
			context, _ := multipartRequestContext(t, "fixture.bin", test.contentType, test.data)
			_, err := (&Storage{}).Upload(context, "file")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("upload error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestStorageUploadPolicyBytesAndMIMEContract(t *testing.T) {
	previous := center.GetAppConfig()
	t.Cleanup(func() { center.SetAppConfig(previous) })
	storage := &Storage{}
	center.SetAppConfig(&fakeAppConfig{values: map[string]string{}})
	defaultPolicy, err := storage.loadPolicy(&gin.Context{})
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	if defaultPolicy.maxBytes != 10*1024*1024 {
		t.Fatalf("default max object bytes = %d", defaultPolicy.maxBytes)
	}
	if len(defaultPolicy.allowedTypes) == 0 {
		t.Fatal("default media-type policy is empty")
	}

	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "object bytes exceed hard ceiling",
			values: map[string]string{maxSizeConfigKey: "104857601"},
			want:   "invalid maximum file size",
		},
		{
			name:   "filename extensions are not media types",
			values: map[string]string{allowedTypesConfigKey: ".jpg,.png"},
			want:   "invalid allowed media type",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			center.SetAppConfig(&fakeAppConfig{values: test.values})
			_, err := storage.loadPolicy(&gin.Context{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load policy error = %v, want %q", err, test.want)
			}
		})
	}

	center.SetAppConfig(&fakeAppConfig{values: map[string]string{
		maxSizeConfigKey:      "104857600",
		allowedTypesConfigKey: "image/*,text/plain",
	}})
	policy, err := storage.loadPolicy(&gin.Context{})
	if err != nil {
		t.Fatalf("load maximum valid policy: %v", err)
	}
	if policy.maxBytes != 100*1024*1024 {
		t.Fatalf("max object bytes = %d", policy.maxBytes)
	}
	if got := multipartRequestLimit(1); got != 1+64*1024 {
		t.Fatalf("one-byte object request cap = %d", got)
	}
}

func TestStorageValidationSupportsWildcardMediaTypes(t *testing.T) {
	storage := &Storage{}
	if !storage.isAllowedType("IMAGE/PNG; charset=binary", []string{"image/*"}) {
		t.Fatal("wildcard media type did not match normalized content type")
	}
	if got := normalizeMediaType("Text/Plain; Charset=UTF-8"); got != "text/plain" {
		t.Fatalf("normalized media type = %q", got)
	}
	if got := normalizeMediaType(" IMAGE/* "); got != "image/*" {
		t.Fatalf("normalized wildcard = %q", got)
	}
}

func uploadText(t *testing.T, storage *Storage, filename string, data []byte) *UploadResult {
	t.Helper()
	context, _ := multipartRequestContext(t, filename, "text/plain", data)
	result, err := storage.Upload(context, "file")
	if err != nil {
		t.Fatalf("upload %q: %v", filename, err)
	}
	if result.Filename != filepath.Base(filename) || result.Size != int64(len(data)) || result.MimeType != "text/plain" {
		t.Fatalf("unexpected result: %#v", result)
	}
	return result
}

func multipartRequestContext(t *testing.T, filename, contentType string, data []byte) (*gin.Context, []byte) {
	t.Helper()
	body, formDataContentType := multipartBody(t, filename, contentType, data)
	request := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(body))
	request.Header.Set("Content-Type", formDataContentType)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	return context, body
}

func parsedMultipartFile(t *testing.T, filename, contentType string, data []byte) (*gin.Context, *multipart.FileHeader) {
	t.Helper()
	context, _ := multipartRequestContext(t, filename, contentType, data)
	if err := context.Request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	files := context.Request.MultipartForm.File["file"]
	if len(files) != 1 {
		t.Fatalf("multipart file count = %d", len(files))
	}
	return context, files[0]
}

func multipartBody(t *testing.T, filename, contentType string, data []byte) ([]byte, string) {
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
	return body.Bytes(), writer.FormDataContentType()
}

func enterTemporaryWorkingDirectory(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	temporary := t.TempDir()
	if err := os.Chdir(temporary); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return temporary
}

func assertFileContents(t *testing.T, filename, expected string) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	if string(data) != expected {
		t.Fatalf("contents of %s = %q, want %q", filename, data, expected)
	}
}

func assertNoFiles(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return
	}
	var files []string
	err := filepath.WalkDir(root, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, filename)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) != 0 {
		t.Fatalf("unexpected files below %s: %v", root, files)
	}
}

type cancelUploadAfterRead struct {
	data   []byte
	cancel stdcontext.CancelFunc
	reads  int
}

func (r *cancelUploadAfterRead) Read(buffer []byte) (int, error) {
	if r.reads > 0 {
		return 0, io.EOF
	}
	r.reads++
	read := copy(buffer, r.data)
	r.cancel()
	return read, nil
}
