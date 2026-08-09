package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
)

const (
	defaultMaxSize             int64 = 10 * 1024 * 1024 // 10 MiB
	maxConfiguredObjectBytes         = 100 * 1024 * 1024
	multipartEnvelopeAllowance       = 64 * 1024
	maxSizeConfigKey                 = "storage:maxSize"
	allowedTypesConfigKey            = "storage:allowedTypes"
	defaultLocalObjectRoot           = "public"
	objectKeyPrefix                  = "uploads"
)

var (
	// ErrUploadTooLarge is returned before an object write when either the raw
	// multipart request or the opened file stream exceeds the configured limit.
	ErrUploadTooLarge = errors.New("upload exceeds configured size limit")
	// ErrInvalidUpload identifies malformed multipart input or a file rejected
	// by the declared and detected media-type policy.
	ErrInvalidUpload = errors.New("invalid upload")
	// ErrObjectConflict reports a create-only local write collision.
	ErrObjectConflict = errors.New("object already exists")
	// ErrUnsafeObjectPath reports an object root or generated key that cannot be
	// proven to stay below the fixed local object root.
	ErrUnsafeObjectPath = errors.New("unsafe object path")
)

var defaultAllowedTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
	"application/pdf",
	"text/plain",
	"application/vnd.ms-excel",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/msword",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

type Storage struct {
	generateObjectID func() (string, error)
	localObjectRoot  string
}

type uploadPolicy struct {
	maxBytes     int64
	allowedTypes []string
}

type admittedUpload struct {
	header *multipart.FileHeader
	policy uploadPolicy
	form   *multipart.Form
}

type inspectedUpload struct {
	size      int64
	mimeType  string
	objectKey string
	source    multipart.File
}

type UploadResult struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

// Upload admits one multipart file and stores it using one immutable policy
// snapshot. The raw body is bounded before Gin invokes multipart parsing, and
// any multipart temporary files are removed before the method returns.
func (s *Storage) Upload(c *gin.Context, fieldName string) (*UploadResult, error) {
	upload, err := s.admit(c, fieldName)
	if err != nil {
		return nil, err
	}
	result, storeErr := s.store(c, upload)
	cleanupErr := upload.close()
	if storeErr != nil {
		return nil, errors.Join(storeErr, cleanupErr)
	}
	if cleanupErr != nil {
		// The published object is complete. Returning an error would encourage a
		// retry and create a second object, so surface the bounded temp cleanup
		// failure operationally without turning a successful write into failure.
		slog.Warn("remove multipart temporary files failed")
	}
	return result, nil
}

func (s *Storage) admit(c *gin.Context, fieldName string) (*admittedUpload, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, fmt.Errorf("%w: multipart request is required", ErrInvalidUpload)
	}
	if c.Request.MultipartForm != nil {
		_ = c.Request.MultipartForm.RemoveAll()
		return nil, fmt.Errorf("%w: multipart parsing occurred before upload admission", ErrInvalidUpload)
	}

	policy, err := s.loadPolicy(c)
	if err != nil {
		return nil, err
	}
	requestLimit := multipartRequestLimit(policy.maxBytes)
	if c.Request.ContentLength > requestLimit {
		return nil, fmt.Errorf("%w: request body exceeds %d bytes", ErrUploadTooLarge, requestLimit)
	}

	// This assignment must happen before FormFile. It bounds both chunked and
	// content-length requests even when multipart parsing spills to disk.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requestLimit)
	header, err := c.FormFile(fieldName)
	form := c.Request.MultipartForm
	if err != nil {
		if form != nil {
			_ = form.RemoveAll()
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) || errors.Is(err, multipart.ErrMessageTooLarge) {
			return nil, fmt.Errorf("%w: request body exceeds %d bytes", ErrUploadTooLarge, requestLimit)
		}
		return nil, fmt.Errorf("%w: parse multipart file: %v", ErrInvalidUpload, err)
	}

	upload := &admittedUpload{header: header, policy: policy, form: form}
	if header == nil {
		_ = upload.close()
		return nil, fmt.Errorf("%w: multipart file is required", ErrInvalidUpload)
	}
	if header.Size < 0 || header.Size > policy.maxBytes {
		_ = upload.close()
		return nil, fmt.Errorf("%w: file stream exceeds %d bytes", ErrUploadTooLarge, policy.maxBytes)
	}
	return upload, nil
}

func (u *admittedUpload) close() error {
	if u == nil || u.form == nil {
		return nil
	}
	form := u.form
	u.form = nil
	return form.RemoveAll()
}

func multipartRequestLimit(maxFileBytes int64) int64 {
	if maxFileBytes > math.MaxInt64-multipartEnvelopeAllowance {
		return math.MaxInt64
	}
	return maxFileBytes + multipartEnvelopeAllowance
}

func (s *Storage) loadPolicy(c *gin.Context) (uploadPolicy, error) {
	policy := uploadPolicy{
		maxBytes:     defaultMaxSize,
		allowedTypes: append([]string(nil), defaultAllowedTypes...),
	}
	if appConfig := center.GetAppConfig(); appConfig != nil {
		if maxSizeText, _ := appConfig.GetAppConfig(c, maxSizeConfigKey); maxSizeText != "" {
			parsed, err := strconv.ParseInt(strings.TrimSpace(maxSizeText), 10, 64)
			if err != nil || parsed <= 0 || parsed > maxConfiguredObjectBytes {
				return uploadPolicy{}, fmt.Errorf("invalid maximum file size %q", maxSizeText)
			}
			policy.maxBytes = parsed
		}
		if allowedTypesText, _ := appConfig.GetAppConfig(c, allowedTypesConfigKey); allowedTypesText != "" {
			policy.allowedTypes = strings.Split(allowedTypesText, ",")
			for i := range policy.allowedTypes {
				policy.allowedTypes[i] = normalizeMediaType(policy.allowedTypes[i])
				if !validAllowedMediaType(policy.allowedTypes[i]) {
					return uploadPolicy{}, fmt.Errorf("invalid allowed media type %q", policy.allowedTypes[i])
				}
			}
		}
	}
	return policy, nil
}

func (s *Storage) store(c *gin.Context, upload *admittedUpload) (*UploadResult, error) {
	metadata, err := s.inspect(upload)
	if err != nil {
		return nil, err
	}
	closeSource := func(cause error) error {
		return errors.Join(cause, metadata.source.Close())
	}
	metadata.objectKey, err = s.newObjectKey()
	if err != nil {
		return nil, closeSource(err)
	}

	var storageType string
	if appConfig := center.GetAppConfig(); appConfig != nil {
		storageType, _ = appConfig.GetAppConfig(c, "storage:type")
	}

	var url string
	switch storageType {
	case "s3":
		url, err = s.uploadS3(c, upload, metadata)
	default:
		url, err = s.uploadLocal(c.Request.Context(), upload, metadata)
	}
	if err != nil {
		return nil, closeSource(err)
	}
	if err := metadata.source.Close(); err != nil {
		// Provider success is already externally visible. Preserve the success
		// result and report cleanup failure without inviting a duplicate retry.
		slog.Warn("close admitted upload source failed")
	}

	return &UploadResult{
		URL:      url,
		Filename: upload.header.Filename,
		Size:     metadata.size,
		MimeType: metadata.mimeType,
	}, nil
}

func (s *Storage) inspect(upload *admittedUpload) (metadata inspectedUpload, err error) {
	if upload == nil || upload.header == nil {
		return inspectedUpload{}, fmt.Errorf("%w: file is required", ErrInvalidUpload)
	}
	file, err := upload.header.Open()
	if err != nil {
		return inspectedUpload{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, file.Close())
		}
	}()

	limited := io.LimitReader(file, upload.policy.maxBytes+1)
	buffer := make([]byte, 512)
	read, readErr := io.ReadFull(limited, buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return inspectedUpload{}, readErr
	}
	remainder, err := io.Copy(io.Discard, limited)
	if err != nil {
		return inspectedUpload{}, err
	}
	actualSize := int64(read) + remainder
	if actualSize > upload.policy.maxBytes {
		return inspectedUpload{}, fmt.Errorf("%w: file stream exceeds %d bytes", ErrUploadTooLarge, upload.policy.maxBytes)
	}

	declaredType := normalizeMediaType(upload.header.Header.Get("Content-Type"))
	if declaredType == "" {
		declaredType = "application/octet-stream"
	}
	if !s.isAllowedType(declaredType, upload.policy.allowedTypes) {
		return inspectedUpload{}, fmt.Errorf("%w: file type %s is not allowed", ErrInvalidUpload, declaredType)
	}
	actualType := normalizeMediaType(http.DetectContentType(buffer[:read]))
	if !s.isAllowedType(actualType, upload.policy.allowedTypes) {
		return inspectedUpload{}, fmt.Errorf("%w: detected file type %s is not allowed", ErrInvalidUpload, actualType)
	}
	return inspectedUpload{size: actualSize, mimeType: actualType, source: file}, nil
}

func (s *Storage) isAllowedType(contentType string, allowedTypes []string) bool {
	contentType = normalizeMediaType(contentType)
	for _, allowed := range allowedTypes {
		allowed = normalizeMediaType(allowed)
		if strings.EqualFold(contentType, allowed) {
			return true
		}
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(strings.ToLower(contentType), strings.ToLower(prefix)) {
				return true
			}
		}
	}
	return false
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasSuffix(value, "/*") {
		return strings.ToLower(value)
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(value)
	}
	return strings.ToLower(mediaType)
}

func validAllowedMediaType(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	if parts[1] == "*" {
		return !strings.ContainsAny(parts[0], "* ,;")
	}
	_, _, err := mime.ParseMediaType(value)
	return err == nil
}

func (s *Storage) newObjectKey() (string, error) {
	generator := s.generateObjectID
	if generator == nil {
		generator = func() (string, error) {
			identifier, err := uuid.NewRandom()
			return identifier.String(), err
		}
	}
	identifier, err := generator()
	if err != nil {
		return "", fmt.Errorf("generate object identifier: %w", err)
	}
	parsed, err := uuid.Parse(identifier)
	if err != nil || parsed.String() != strings.ToLower(identifier) {
		return "", fmt.Errorf("%w: generated identifier is not a canonical UUID", ErrUnsafeObjectPath)
	}
	return path.Join(objectKeyPrefix, parsed.String()), nil
}

func (s *Storage) uploadS3(c *gin.Context, upload *admittedUpload, metadata inspectedUpload) (string, error) {
	appConfig := center.GetAppConfig()
	if appConfig == nil {
		return "", fmt.Errorf("application storage configuration is not initialized")
	}

	storage := config.Storage{}
	s3Type, _ := appConfig.GetAppConfig(c, "storage:type")
	if s3Type == "" {
		s3Type = string(config.S3)
	}
	storage.Type = config.ProviderType(s3Type)
	storage.Region, _ = appConfig.GetAppConfig(c, "storage:s3Region")
	storage.Endpoint, _ = appConfig.GetAppConfig(c, "storage:s3Endpoint")
	storage.Bucket, _ = appConfig.GetAppConfig(c, "storage:s3Bucket")
	storage.AccessKeyID, _ = appConfig.GetAppConfig(c, "storage:s3AccessKeyID")
	storage.SecretAccessKey, _ = appConfig.GetAppConfig(c, "storage:s3SecretAccessKey")
	storage.SigningMethod, _ = appConfig.GetAppConfig(c, "storage:s3SigningMethod")
	storage.Init()

	if _, err := metadata.source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	_, err := storage.GetClient().PutObject(c, &s3.PutObjectInput{
		Bucket:        &storage.Bucket,
		Key:           aws.String(metadata.objectKey),
		Body:          io.LimitReader(metadata.source, upload.policy.maxBytes+1),
		ContentLength: aws.Int64(metadata.size),
	})
	if err != nil {
		return "", err
	}

	endpoint, _ := appConfig.GetAppConfig(c, "storage:endpoint")
	return fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), metadata.objectKey), nil
}

func (s *Storage) uploadLocal(ctx context.Context, upload *admittedUpload, metadata inspectedUpload) (string, error) {
	if _, err := metadata.source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return s.writeLocalObject(ctx, metadata.source, upload.policy.maxBytes, metadata)
}

func (s *Storage) writeLocalObject(ctx context.Context, source io.Reader, maxBytes int64, metadata inspectedUpload) (string, error) {
	rootName := s.localObjectRoot
	if rootName == "" {
		rootName = defaultLocalObjectRoot
	}
	rootName = filepath.Clean(rootName)
	if filepath.IsAbs(rootName) || rootName == "." || rootName == ".." || strings.HasPrefix(rootName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: local object root must be a confined relative directory", ErrUnsafeObjectPath)
	}

	workspaceRoot, err := os.OpenRoot(".")
	if err != nil {
		return "", err
	}
	defer workspaceRoot.Close()
	if err := workspaceRoot.MkdirAll(rootName, 0o750); err != nil {
		return "", fmt.Errorf("%w: create local object root: %v", ErrUnsafeObjectPath, err)
	}
	objectRoot, err := workspaceRoot.OpenRoot(rootName)
	if err != nil {
		return "", fmt.Errorf("%w: open local object root: %v", ErrUnsafeObjectPath, err)
	}
	defer objectRoot.Close()

	directory := path.Dir(metadata.objectKey)
	if err := objectRoot.MkdirAll(directory, 0o750); err != nil {
		return "", fmt.Errorf("%w: create object directory: %v", ErrUnsafeObjectPath, err)
	}
	destination, err := objectRoot.OpenFile(metadata.objectKey, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("%w: %s", ErrObjectConflict, metadata.objectKey)
		}
		return "", fmt.Errorf("create local object: %w", err)
	}
	cleanup := func(cause error) error {
		return errors.Join(cause, destination.Close(), objectRoot.Remove(metadata.objectKey))
	}
	written, err := io.Copy(destination, io.LimitReader(&contextReader{ctx: ctx, reader: source}, maxBytes+1))
	if err != nil {
		return "", cleanup(err)
	}
	if written > maxBytes {
		return "", cleanup(fmt.Errorf("%w: file stream exceeds %d bytes", ErrUploadTooLarge, maxBytes))
	}
	if written != metadata.size {
		return "", cleanup(fmt.Errorf("file stream changed during storage"))
	}
	if err := ctx.Err(); err != nil {
		return "", cleanup(err)
	}
	if err := destination.Sync(); err != nil {
		return "", cleanup(err)
	}
	if err := ctx.Err(); err != nil {
		return "", cleanup(err)
	}
	if err := destination.Close(); err != nil {
		return "", errors.Join(err, objectRoot.Remove(metadata.objectKey))
	}
	return "/public/" + metadata.objectKey, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		read, err := r.reader.Read(buffer)
		if contextErr := r.ctx.Err(); contextErr != nil {
			return read, contextErr
		}
		return read, err
	}
}
