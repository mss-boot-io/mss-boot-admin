package service

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
)

type Storage struct{}

const (
	defaultMaxSize        = 10 * 1024 * 1024 // 10MB
	maxSizeConfigKey      = "storage:maxSize"
	allowedTypesConfigKey = "storage:allowedTypes"
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

type UploadResult struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

func (s *Storage) Upload(c *gin.Context, f *multipart.FileHeader, userID string) (*UploadResult, error) {
	if err := s.validate(c, f); err != nil {
		return nil, err
	}

	var storageType, endpoint string
	if appConfig := center.GetAppConfig(); appConfig != nil {
		storageType, _ = appConfig.GetAppConfig(c, "storage:type")
		endpoint, _ = appConfig.GetAppConfig(c, "storage:endpoint")
	}

	var url string
	var err error

	switch storageType {
	case "s3":
		url, err = s.uploadS3(c, f, userID)
	default:
		url, err = s.uploadLocal(c, f, userID, endpoint)
	}

	if err != nil {
		return nil, err
	}

	return &UploadResult{
		URL:      url,
		Filename: f.Filename,
		Size:     f.Size,
		MimeType: normalizeMediaType(f.Header.Get("Content-Type")),
	}, nil
}

func (s *Storage) validate(c *gin.Context, f *multipart.FileHeader) error {
	if f == nil {
		return fmt.Errorf("file is required")
	}

	maxSize := int64(defaultMaxSize)
	allowedTypes := append([]string(nil), defaultAllowedTypes...)
	if appConfig := center.GetAppConfig(); appConfig != nil {
		if maxSizeText, _ := appConfig.GetAppConfig(c, maxSizeConfigKey); maxSizeText != "" {
			parsed, err := strconv.ParseInt(strings.TrimSpace(maxSizeText), 10, 64)
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid maximum file size %q", maxSizeText)
			}
			maxSize = parsed
		}
		if allowedTypesText, _ := appConfig.GetAppConfig(c, allowedTypesConfigKey); allowedTypesText != "" {
			allowedTypes = strings.Split(allowedTypesText, ",")
			for i := range allowedTypes {
				allowedTypes[i] = normalizeMediaType(allowedTypes[i])
			}
		}
	}

	if f.Size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", f.Size, maxSize)
	}

	contentType := normalizeMediaType(f.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if !s.isAllowedType(contentType, allowedTypes) {
		return fmt.Errorf("file type %s is not allowed", contentType)
	}

	file, err := f.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	read, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}
	actualType := normalizeMediaType(http.DetectContentType(buffer[:read]))
	if !s.isAllowedType(actualType, allowedTypes) {
		return fmt.Errorf("detected file type %s is not allowed", actualType)
	}

	_, _ = file.Seek(0, 0)
	return nil
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

func (s *Storage) uploadS3(c *gin.Context, f *multipart.FileHeader, userID string) (string, error) {
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

	file, err := f.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	key := fmt.Sprintf("%s/%s", userID, s.sanitizeFilename(f.Filename))
	_, err = storage.GetClient().PutObject(c, &s3.PutObjectInput{
		Bucket: &storage.Bucket,
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", err
	}

	endpoint, _ := appConfig.GetAppConfig(c, "storage:endpoint")
	return fmt.Sprintf("%s/%s", endpoint, key), nil
}

func (s *Storage) uploadLocal(c *gin.Context, f *multipart.FileHeader, userID, _ string) (string, error) {
	filename := s.sanitizeFilename(f.Filename)
	relativePath := filepath.Join("public", userID, filename)

	if err := c.SaveUploadedFile(f, relativePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("/public/%s/%s", userID, filename), nil
}

func (s *Storage) sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, " ", "_")
	return filename
}
