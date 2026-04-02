package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config"

	"github.com/mss-boot-io/mss-boot-admin/center"
)

type Storage struct{}

const (
	defaultMaxSize       = 10 * 1024 * 1024 // 10MB
	maxSizeConfigKey     = "storage:maxSize"
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

	storageType, _ := center.GetAppConfig().GetAppConfig(c, "storage:type")
	endpoint, _ := center.GetAppConfig().GetAppConfig(c, "storage:endpoint")

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
		MimeType: f.Header.Get("Content-Type"),
	}, nil
}

func (s *Storage) validate(c *gin.Context, f *multipart.FileHeader) error {
	maxSizeStr, _ := center.GetAppConfig().GetAppConfig(c, maxSizeConfigKey)
	maxSize := int64(defaultMaxSize)
	if maxSizeStr != "" {
		var size int
		fmt.Sscanf(maxSizeStr, "%d", &size)
		maxSize = int64(size)
	}

	if f.Size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", f.Size, maxSize)
	}

	allowedTypesStr, _ := center.GetAppConfig().GetAppConfig(c, allowedTypesConfigKey)
	allowedTypes := defaultAllowedTypes
	if allowedTypesStr != "" {
		allowedTypes = strings.Split(allowedTypesStr, ",")
		for i := range allowedTypes {
			allowedTypes[i] = strings.TrimSpace(allowedTypes[i])
		}
	}

	contentType := f.Header.Get("Content-Type")
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
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}

	actualType := http.DetectContentType(buffer)
	if !s.isAllowedType(actualType, allowedTypes) {
		return fmt.Errorf("detected file type %s is not allowed", actualType)
	}

	_, _ = file.Seek(0, 0)

	return nil
}

func (s *Storage) isAllowedType(contentType string, allowedTypes []string) bool {
	for _, t := range allowedTypes {
		if strings.EqualFold(contentType, t) {
			return true
		}
		if strings.HasSuffix(t, "/*") {
			prefix := strings.TrimSuffix(t, "*")
			if strings.HasPrefix(strings.ToLower(contentType), strings.ToLower(prefix)) {
				return true
			}
		}
	}
	return false
}

func (s *Storage) uploadS3(c *gin.Context, f *multipart.FileHeader, userID string) (string, error) {
	storage := config.Storage{}
	s3Type, _ := center.GetAppConfig().GetAppConfig(c, "storage:type")
	if s3Type == "" {
		s3Type = string(config.S3)
	}
	storage.Type = config.ProviderType(s3Type)
	storage.Region, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3Region")
	storage.Endpoint, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3Endpoint")
	storage.Bucket, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3Bucket")
	storage.AccessKeyID, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3AccessKeyID")
	storage.SecretAccessKey, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3SecretAccessKey")
	storage.SigningMethod, _ = center.GetAppConfig().GetAppConfig(c, "storage:s3SigningMethod")
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

	endpoint, _ := center.GetAppConfig().GetAppConfig(c, "storage:endpoint")
	return fmt.Sprintf("%s/%s", endpoint, key), nil
}

func (s *Storage) uploadLocal(c *gin.Context, f *multipart.FileHeader, userID, endpoint string) (string, error) {
	filename := s.sanitizeFilename(f.Filename)
	relativePath := filepath.Join("public", userID, filename)
	
	err := c.SaveUploadedFile(f, relativePath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/public/%s/%s", userID, filename), nil
}

func (s *Storage) sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, " ", "_")
	return filename
}