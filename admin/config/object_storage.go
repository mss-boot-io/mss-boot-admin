package config

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

var (
	// ErrObjectStorageUnavailable is returned when no immutable runtime profile
	// has been installed or its owner is already closing.
	ErrObjectStorageUnavailable = errors.New("object storage is unavailable")
	// ErrObjectStorageAlreadyInstalled prevents hidden client replacement during
	// configuration reload. Storage profiles are startup snapshots in v1.1.
	ErrObjectStorageAlreadyInstalled = errors.New("object storage is already installed")
)

// ObjectStorageLease exposes one borrowed immutable profile and provider
// client for the duration of WithObjectStorage. Callers must not retain or
// close any field after the callback returns.
type ObjectStorageLease struct {
	Profile        *frameworkconfig.StorageProfile
	S3Client       *s3.Client
	LocalRoot      *os.Root
	LocalURLPrefix string
}

// InstallObjectStorage publishes the single application-owned storage handle.
// The URL prefix is meaningful only for an explicitly mounted Local profile.
func (e *Config) InstallObjectStorage(
	handle *frameworkconfig.StorageHandle,
	localRoot *os.Root,
	localURLPrefix string,
) error {
	if e == nil || handle == nil {
		return ErrObjectStorageUnavailable
	}
	if (localRoot == nil) != (strings.TrimSpace(localURLPrefix) == "") {
		return ErrObjectStorageUnavailable
	}
	e.objectStorageMu.Lock()
	defer e.objectStorageMu.Unlock()
	if e.objectStorageHandle != nil {
		return ErrObjectStorageAlreadyInstalled
	}
	e.objectStorageHandle = handle
	e.objectStorageLocalRoot = localRoot
	e.objectStorageLocalURLPrefix = strings.TrimRight(localURLPrefix, "/")
	return nil
}

// WithObjectStorage leases the installed runtime handle for one bounded
// operation. The framework Handle rejects leases after shutdown starts.
func (e *Config) WithObjectStorage(
	ctx context.Context,
	operation func(ObjectStorageLease) error,
) error {
	if e == nil || ctx == nil || operation == nil {
		return ErrObjectStorageUnavailable
	}
	e.objectStorageMu.Lock()
	handle := e.objectStorageHandle
	localRoot := e.objectStorageLocalRoot
	localURLPrefix := e.objectStorageLocalURLPrefix
	e.objectStorageMu.Unlock()
	if handle == nil {
		return ErrObjectStorageUnavailable
	}
	err := handle.Use(ctx, func(profile *frameworkconfig.StorageProfile, client *s3.Client) error {
		return operation(ObjectStorageLease{
			Profile:        profile,
			S3Client:       client,
			LocalRoot:      localRoot,
			LocalURLPrefix: localURLPrefix,
		})
	})
	if err != nil && errors.Is(err, frameworkconfig.ErrStorageHandleClosing) {
		return errors.Join(ErrObjectStorageUnavailable, err)
	}
	return err
}

func (e *Config) closeObjectStorage(ctx context.Context) error {
	if e == nil {
		return nil
	}
	e.objectStorageCloseMu.Lock()
	defer e.objectStorageCloseMu.Unlock()
	e.objectStorageMu.Lock()
	handle := e.objectStorageHandle
	localRoot := e.objectStorageLocalRoot
	e.objectStorageMu.Unlock()
	if handle == nil {
		return nil
	}
	if err := handle.Close(ctx); err != nil {
		return err
	}
	var rootErr error
	if localRoot != nil {
		rootErr = localRoot.Close()
	}
	e.objectStorageMu.Lock()
	if e.objectStorageHandle == handle {
		e.objectStorageHandle = nil
		e.objectStorageLocalRoot = nil
		e.objectStorageLocalURLPrefix = ""
	}
	e.objectStorageMu.Unlock()
	return rootErr
}
