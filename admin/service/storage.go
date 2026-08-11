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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	adminconfig "github.com/mss-boot-io/mss-boot-admin/admin/config"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

const (
	defaultMaxSize             int64 = 10 * 1024 * 1024 // 10 MiB
	maxConfiguredObjectBytes         = 100 * 1024 * 1024
	multipartEnvelopeAllowance       = 64 * 1024
	maxSizeConfigKey                 = "storage:maxSize"
	allowedTypesConfigKey            = "storage:allowedTypes"
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
	// ErrStorageUnavailable reports that no provider/delivery snapshot can
	// safely serve this request. It must map to a fixed 503 response.
	ErrStorageUnavailable = errors.New("object storage is unavailable")
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
	useObjectStorage func(context.Context, func(adminconfig.ObjectStorageLease) error) error
}

type uploadPolicy struct {
	maxBytes     int64
	allowedTypes []string
}

type uploadPolicySnapshotReader interface {
	GetAppConfigSnapshot(*gin.Context, ...string) (map[string]string, error)
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
	appConfig := center.GetAppConfig()
	snapshotReader, ok := appConfig.(uploadPolicySnapshotReader)
	if !ok {
		slog.Warn("upload policy snapshot reader unavailable; upload rejected")
		return uploadPolicy{}, ErrStorageUnavailable
	}
	values, err := snapshotReader.GetAppConfigSnapshot(c, maxSizeConfigKey, allowedTypesConfigKey)
	if err != nil {
		slog.Warn("upload policy snapshot unavailable; upload rejected")
		return uploadPolicy{}, ErrStorageUnavailable
	}
	if maxSizeText, configured := values[maxSizeConfigKey]; configured {
		parsed, err := strconv.ParseInt(strings.TrimSpace(maxSizeText), 10, 64)
		if err != nil || parsed <= 0 || parsed > maxConfiguredObjectBytes {
			return uploadPolicy{}, errors.Join(ErrStorageUnavailable, errors.New("invalid maximum file size policy"))
		}
		policy.maxBytes = parsed
	}
	if allowedTypesText, configured := values[allowedTypesConfigKey]; configured {
		if strings.TrimSpace(allowedTypesText) == "" {
			return uploadPolicy{}, errors.Join(ErrStorageUnavailable, errors.New("invalid allowed media type policy"))
		}
		policy.allowedTypes = strings.Split(allowedTypesText, ",")
		for i := range policy.allowedTypes {
			policy.allowedTypes[i] = normalizeMediaType(policy.allowedTypes[i])
			if !validAllowedMediaType(policy.allowedTypes[i]) {
				return uploadPolicy{}, errors.Join(ErrStorageUnavailable, errors.New("invalid allowed media type policy"))
			}
		}
	}
	return policy, nil
}

func (s *Storage) store(c *gin.Context, upload *admittedUpload) (*UploadResult, error) {
	if c == nil || c.Request == nil {
		return nil, ErrStorageUnavailable
	}
	ctx := c.Request.Context()
	var result *UploadResult
	err := s.withObjectStorage(ctx, func(lease adminconfig.ObjectStorageLease) error {
		if lease.Profile == nil {
			return ErrStorageUnavailable
		}
		if lease.Profile.Provider() != frameworkconfig.Local {
			// D1 builds and owns the S3 client once, but private delivery and the
			// ObjectStore contract intentionally remain unavailable until D4.
			return ErrStorageUnavailable
		}
		if lease.LocalRoot == nil || lease.LocalURLPrefix == "" {
			return ErrStorageUnavailable
		}

		metadata, inspectErr := s.inspect(upload)
		if inspectErr != nil {
			return inspectErr
		}
		closeSource := func(cause error) error {
			return errors.Join(cause, metadata.source.Close())
		}
		metadata.objectKey, inspectErr = s.newObjectKey()
		if inspectErr != nil {
			return closeSource(inspectErr)
		}
		url, storeErr := s.uploadLocal(ctx, lease.LocalRoot, lease.LocalURLPrefix, upload, metadata)
		if storeErr != nil {
			return closeSource(storeErr)
		}
		if closeErr := metadata.source.Close(); closeErr != nil {
			// Provider success is already externally visible. Preserve the success
			// result and report cleanup failure without inviting a duplicate retry.
			slog.Warn("close admitted upload source failed")
		}
		result = &UploadResult{
			URL:      url,
			Filename: upload.header.Filename,
			Size:     metadata.size,
			MimeType: metadata.mimeType,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Storage) withObjectStorage(
	ctx context.Context,
	operation func(adminconfig.ObjectStorageLease) error,
) error {
	use := s.useObjectStorage
	if use == nil {
		use = adminconfig.Cfg.WithObjectStorage
	}
	if err := use(ctx, operation); err != nil {
		if errors.Is(err, adminconfig.ErrObjectStorageUnavailable) {
			return errors.Join(ErrStorageUnavailable, err)
		}
		return err
	}
	return nil
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

func (s *Storage) uploadLocal(
	ctx context.Context,
	root *os.Root,
	urlPrefix string,
	upload *admittedUpload,
	metadata inspectedUpload,
) (string, error) {
	if _, err := metadata.source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return s.writeLocalObject(ctx, root, urlPrefix, metadata.source, upload.policy.maxBytes, metadata)
}

func (s *Storage) writeLocalObject(
	ctx context.Context,
	objectRoot *os.Root,
	urlPrefix string,
	source io.Reader,
	maxBytes int64,
	metadata inspectedUpload,
) (string, error) {
	if objectRoot == nil || urlPrefix == "" {
		return "", fmt.Errorf("%w: local object root and delivery prefix must be explicit", ErrUnsafeObjectPath)
	}

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
	return strings.TrimRight(urlPrefix, "/") + "/" + metadata.objectKey, nil
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
