package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	bootresponse "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	adminconfig "github.com/mss-boot-io/mss-boot-admin/admin/config"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

const (
	testMultipartEnvelopeAllowance int64 = 64 * 1024
	storageUploadRoute                   = "/admin/api/storage/upload"
	avatarUploadRoute                    = "/admin/api/user/avatar"
)

type uploadAdmissionAppConfig struct {
	values    map[string]string
	getCounts map[string]int
}

func (f *uploadAdmissionAppConfig) SetAppConfig(_ *gin.Context, key string, _ bool, value string) error {
	if f.values == nil {
		f.values = make(map[string]string)
	}
	f.values[key] = value
	return nil
}

func (f *uploadAdmissionAppConfig) GetAppConfig(_ *gin.Context, key string) (string, bool) {
	if f.getCounts == nil {
		f.getCounts = make(map[string]int)
	}
	f.getCounts[key]++
	value, ok := f.values[key]
	return value, ok
}

func (f *uploadAdmissionAppConfig) GetAppConfigSnapshot(_ *gin.Context, keys ...string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := f.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func TestStorageUploadHardLimitBeforeMultipart(t *testing.T) {
	assertUploadHandlerAdmission(t, storageUploadRoute, func(context *gin.Context) {
		(&Storage{}).Upload(context)
	})
}

func TestAvatarUploadHardLimitBeforeMultipart(t *testing.T) {
	assertUploadHandlerAdmission(t, avatarUploadRoute, func(context *gin.Context) {
		(&User{}).UpdateAvatar(context)
	})
}

func assertUploadHandlerAdmission(t *testing.T, route string, handler gin.HandlerFunc) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previous := center.GetAppConfig()
	t.Cleanup(func() { center.SetAppConfig(previous) })

	t.Run("known content length cap plus one is rejected without reading", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("1")
		requestLimit := int64(1) + testMultipartEnvelopeAllowance
		body := &countingReadCloser{reader: bytes.NewReader([]byte("must not be read"))}
		request := httptest.NewRequest(http.MethodPost, route, nil)
		request.Body = body
		request.ContentLength = requestLimit + 1
		request.Header.Set("Content-Type", "multipart/form-data; boundary=admission-test")

		recorder := executeUploadRoute(request, route, handler, 1)
		assertUploadErrorResponse(t, recorder, http.StatusRequestEntityTooLarge, "UPLOAD_REQUEST_TOO_LARGE")
		if body.readBytes != 0 {
			t.Fatalf("preflight read %d body bytes, want zero", body.readBytes)
		}
		if request.MultipartForm != nil {
			t.Fatal("known oversized request parsed a multipart form")
		}
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionNoFiles(t, workingDirectory)
	})

	t.Run("unknown content length reads exactly cap plus one", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("1")
		requestLimit := int64(1) + testMultipartEnvelopeAllowance
		// Keep the admitted file itself within the one-byte object limit and put
		// the excess byte in multipart padding. Without MaxBytesReader this request
		// would parse and store successfully, so the test cannot pass merely because
		// the file-size check happens after parsing.
		multipartBytes, contentType := uploadAdmissionMultipartBodyAtSize(t, int(requestLimit+1), []byte("x"))
		body := &countingReadCloser{reader: bytes.NewReader(multipartBytes)}
		request := httptest.NewRequest(http.MethodPost, route, nil)
		request.Body = body
		request.ContentLength = -1
		request.Header.Set("Content-Type", contentType)

		recorder := executeUploadRoute(request, route, handler, 1)
		assertUploadErrorResponse(t, recorder, http.StatusRequestEntityTooLarge, "UPLOAD_REQUEST_TOO_LARGE")
		if body.readBytes != requestLimit+1 {
			t.Fatalf("bounded reader consumed %d bytes, want %d", body.readBytes, requestLimit+1)
		}
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionNoFiles(t, workingDirectory)
	})

	t.Run("under-limit malformed multipart returns a fixed validation error", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("1")
		body := []byte("--mss-broken-boundary\r\n" +
			"Content-Disposition: form-data; name=\"file\"; filename=\"broken.txt\"\r\n" +
			"Content-Type: text/plain\r\n\r\nunterminated")
		request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(body))
		request.Header.Set("Content-Type", "multipart/form-data; boundary=mss-broken-boundary")

		recorder := executeUploadRoute(request, route, handler, 1)
		assertUploadErrorResponse(t, recorder, http.StatusUnprocessableEntity, "INVALID_UPLOAD")
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionNoFiles(t, workingDirectory)
	})

	t.Run("exact request cap with exact object limit succeeds", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("1")
		installUploadAdmissionLocalStorage(t, filepath.Join(workingDirectory, "public"))
		requestLimit := int64(1) + testMultipartEnvelopeAllowance
		multipartBytes, contentType := uploadAdmissionMultipartBodyAtSize(t, int(requestLimit), []byte("x"))
		body := &countingReadCloser{reader: bytes.NewReader(multipartBytes)}
		request := httptest.NewRequest(http.MethodPost, route, nil)
		request.Body = body
		request.ContentLength = requestLimit
		request.Header.Set("Content-Type", contentType)

		recorder := executeUploadRoute(request, route, handler, 1)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
		}
		if body.readBytes != requestLimit {
			t.Fatalf("exact request read %d bytes, want %d", body.readBytes, requestLimit)
		}
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionFileCount(t, filepath.Join(workingDirectory, "public", "uploads"), 1)
	})

	t.Run("forced multipart spill is removed after policy rejection", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("4")
		installUploadAdmissionLocalStorage(t, filepath.Join(workingDirectory, "public"))
		multipartBytes, contentType := uploadAdmissionMultipartBody(t, []byte("text"), "application/octet-stream", 0)
		request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(multipartBytes))
		request.Header.Set("Content-Type", contentType)

		recorder := executeUploadRoute(request, route, handler, 1)
		assertUploadErrorResponse(t, recorder, http.StatusUnprocessableEntity, "INVALID_UPLOAD")
		if request.MultipartForm == nil {
			t.Fatal("spill test did not parse a multipart form")
		}
		spilledFiles := request.MultipartForm.File["file"]
		if len(spilledFiles) != 1 {
			t.Fatalf("spill file count = %d, want one", len(spilledFiles))
		}
		spilledFile, err := spilledFiles[0].Open()
		if err == nil {
			_ = spilledFile.Close()
			t.Fatal("spilled multipart file remained openable after handler cleanup")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("open cleaned spill error = %v, want not-exist", err)
		}
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionNoFiles(t, workingDirectory)
	})

	t.Run("preparsed multipart input fails closed and is cleaned", func(t *testing.T) {
		workingDirectory := enterUploadAdmissionWorkingDirectory(t)
		temporaryDirectory := t.TempDir()
		t.Setenv("TMPDIR", temporaryDirectory)
		setUploadAdmissionConfig("4")
		multipartBytes, contentType := uploadAdmissionMultipartBody(t, []byte("text"), "text/plain", 0)
		request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(multipartBytes))
		request.Header.Set("Content-Type", contentType)
		if err := request.ParseMultipartForm(1); err != nil {
			t.Fatalf("preparse multipart request: %v", err)
		}
		if request.MultipartForm == nil {
			t.Fatal("preparse did not produce MultipartForm")
		}

		recorder := executeUploadRoute(request, route, handler, 1)
		assertUploadErrorResponse(t, recorder, http.StatusUnprocessableEntity, "INVALID_UPLOAD")
		assertUploadAdmissionNoFiles(t, temporaryDirectory)
		assertUploadAdmissionNoFiles(t, workingDirectory)
	})
}

func setUploadAdmissionConfig(maxObjectBytes string) *uploadAdmissionAppConfig {
	configuration := &uploadAdmissionAppConfig{
		values: map[string]string{
			"storage:maxSize":      maxObjectBytes,
			"storage:allowedTypes": "text/plain",
		},
		getCounts: make(map[string]int),
	}
	center.SetAppConfig(configuration)
	return configuration
}

func installUploadAdmissionLocalStorage(t *testing.T, root string) {
	t.Helper()
	profile, err := (frameworkconfig.Storage{
		Local: &frameworkconfig.LocalStorageConfig{Root: root},
	}).Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("normalize local storage profile: %v", err)
	}
	handle, err := profile.Build(context.Background())
	if err != nil {
		t.Fatalf("build local storage profile: %v", err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("create local storage root: %v", err)
	}
	localRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open local storage root: %v", err)
	}
	if err := adminconfig.Cfg.InstallObjectStorage(handle, localRoot, "/public"); err != nil {
		_ = localRoot.Close()
		t.Fatalf("install local storage owner: %v", err)
	}
	t.Cleanup(func() {
		closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := adminconfig.Cfg.CloseContext(closeContext); err != nil {
			t.Errorf("close local storage owner: %v", err)
		}
	})
}

func executeUploadRoute(request *http.Request, route string, handler gin.HandlerFunc, maxMultipartMemory int64) *httptest.ResponseRecorder {
	engine := gin.New()
	engine.MaxMultipartMemory = maxMultipartMemory
	engine.POST(route, handler)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func assertUploadErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, errorCode string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, status, recorder.Body.String())
	}
	var body bootresponse.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	wantMessage := map[string]string{
		"UPLOAD_REQUEST_TOO_LARGE": "upload request exceeds the configured limit",
		"INVALID_UPLOAD":           "upload request is malformed or violates the file policy",
		"STORAGE_UNAVAILABLE":      "object storage is temporarily unavailable",
	}[errorCode]
	if body.Status != "error" || body.Code != status || body.ErrorCode != errorCode || body.ErrorMessage != wantMessage {
		t.Fatalf("unexpected error response: %#v", body)
	}
	if bytes.Contains([]byte(body.ErrorMessage), []byte("6553")) || bytes.Contains([]byte(body.ErrorMessage), []byte("multipart body")) {
		t.Fatalf("error message leaked internal admission details: %q", body.ErrorMessage)
	}
}

type countingReadCloser struct {
	reader    io.Reader
	readBytes int64
}

func (r *countingReadCloser) Read(buffer []byte) (int, error) {
	read, err := r.reader.Read(buffer)
	r.readBytes += int64(read)
	return read, err
}

func (*countingReadCloser) Close() error { return nil }

func uploadAdmissionMultipartBodyAtSize(t *testing.T, targetSize int, fileData []byte) ([]byte, string) {
	t.Helper()
	if fileData == nil {
		base, _ := uploadAdmissionMultipartBody(t, nil, "text/plain", 0)
		fileBytes := targetSize - len(base)
		if fileBytes < 0 {
			t.Fatalf("target multipart size %d is below envelope %d", targetSize, len(base))
		}
		body, contentType := uploadAdmissionMultipartBody(t, bytes.Repeat([]byte("x"), fileBytes), "text/plain", 0)
		if len(body) != targetSize {
			t.Fatalf("multipart body size = %d, want %d", len(body), targetSize)
		}
		return body, contentType
	}
	base, _ := uploadAdmissionMultipartBody(t, fileData, "text/plain", 0)
	paddingBytes := targetSize - len(base)
	if paddingBytes < 0 {
		t.Fatalf("target multipart size %d is below envelope %d", targetSize, len(base))
	}
	body, contentType := uploadAdmissionMultipartBody(t, fileData, "text/plain", paddingBytes)
	if len(body) != targetSize {
		t.Fatalf("multipart body size = %d, want %d", len(body), targetSize)
	}
	return body, contentType
}

func uploadAdmissionMultipartBody(t *testing.T, data []byte, contentType string, paddingBytes int) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.SetBoundary("mss-upload-admission-boundary"); err != nil {
		t.Fatalf("set multipart boundary: %v", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="upload.txt"`))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart file part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file data: %v", err)
	}
	padding, err := writer.CreateFormField("padding")
	if err != nil {
		t.Fatalf("create multipart padding field: %v", err)
	}
	if _, err := padding.Write(bytes.Repeat([]byte("p"), paddingBytes)); err != nil {
		t.Fatalf("write multipart padding: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func enterUploadAdmissionWorkingDirectory(t *testing.T) string {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	temporary := t.TempDir()
	if err := os.Chdir(temporary); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return temporary
}

func assertUploadAdmissionNoFiles(t *testing.T, root string) {
	t.Helper()
	assertUploadAdmissionFileCount(t, root, 0)
}

func assertUploadAdmissionFileCount(t *testing.T, root string, expected int) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		if expected == 0 {
			return
		}
		t.Fatalf("root %s does not exist", root)
	}
	files := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if files != expected {
		t.Fatalf("file count below %s = %d, want %d", root, files, expected)
	}
}
