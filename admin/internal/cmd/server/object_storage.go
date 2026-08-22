package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	frameworkconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config"
)

func initializeApplicationDelivery(ctx context.Context, applicationConfig *config.Config, router gin.IRouter) error {
	if applicationConfig == nil || router == nil {
		return errors.New("application delivery requires configuration and router")
	}
	if err := installObjectStorage(ctx, applicationConfig); err != nil {
		return err
	}
	applicationConfig.Application.Init(router)
	return nil
}

func installObjectStorage(ctx context.Context, applicationConfig *config.Config) error {
	if applicationConfig == nil || applicationConfig.Storage == nil {
		return nil
	}
	profile, err := applicationConfig.Storage.Normalize(ctx, frameworkconfig.EnvSecretResolver{})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		// Storage is optional for the application, but upload endpoints must fail
		// closed. Do not include provider errors that may expose environment data.
		slog.Warn("object storage profile is invalid; upload endpoints are disabled")
		return nil
	}

	localURLPrefix := ""
	var localRoot *os.Root
	if profile.Provider() == frameworkconfig.Local {
		root, ok := profile.LocalRoot()
		if !ok {
			slog.Warn("local object storage root is unavailable; upload endpoints are disabled")
			return nil
		}
		localURLPrefix, ok = explicitLocalDelivery(applicationConfig.Application, root)
		if !ok {
			slog.Warn("local object storage has no explicit development delivery; upload endpoints are disabled")
			return nil
		}
		localRoot, err = openPinnedLocalRoot(root)
		if err != nil {
			slog.Warn("local object storage root is unsafe; upload endpoints are disabled")
			return nil
		}
	}

	handle, err := profile.Build(ctx)
	if err != nil {
		if localRoot != nil {
			_ = localRoot.Close()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		slog.Warn("object storage client is unavailable; upload endpoints are disabled")
		return nil
	}
	if err := applicationConfig.InstallObjectStorage(handle, localRoot, localURLPrefix); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		closeErr := handle.Close(closeCtx)
		var rootErr error
		if localRoot != nil {
			rootErr = localRoot.Close()
		}
		if errors.Is(err, config.ErrObjectStorageAlreadyInstalled) {
			return errors.Join(err, closeErr, rootErr)
		}
		return errors.Join(fmt.Errorf("install object storage owner: %w", err), closeErr, rootErr)
	}
	if localRoot != nil {
		applicationConfig.Application.PinStatic(localURLPrefix, http.FS(localRoot.FS()))
	}
	return nil
}

// openPinnedLocalRoot creates the configured directory, opens it once, and
// verifies that the path still names the same non-symlink directory. Opening
// first makes later path replacement harmless to object writes: os.Root keeps
// the original directory handle pinned for the application lifetime.
func openPinnedLocalRoot(rootName string) (*os.Root, error) {
	rootName = filepath.Clean(rootName)
	if !filepath.IsAbs(rootName) {
		return nil, errors.New("local object root must be absolute")
	}
	if err := os.MkdirAll(rootName, 0o750); err != nil {
		return nil, fmt.Errorf("create local object root: %w", err)
	}
	root, err := os.OpenRoot(rootName)
	if err != nil {
		return nil, fmt.Errorf("open local object root: %w", err)
	}
	openedInfo, openedErr := root.Stat(".")
	namedInfo, namedErr := os.Lstat(rootName)
	if openedErr != nil || namedErr != nil || !namedInfo.IsDir() ||
		namedInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, namedInfo) {
		_ = root.Close()
		return nil, errors.New("local object root must remain a real directory")
	}
	return root, nil
}

func explicitLocalDelivery(application config.Application, root string) (string, bool) {
	if application.Mode != config.ModeDev || !filepath.IsAbs(root) {
		return "", false
	}
	cleanRoot := filepath.Clean(root)
	matched := ""
	for rawURLPrefix, rawFilesystemRoot := range application.StaticPath {
		rawURLPrefix = strings.TrimSpace(rawURLPrefix)
		urlPrefix := path.Clean(rawURLPrefix)
		if rawURLPrefix == "" || !strings.HasPrefix(rawURLPrefix, "/") ||
			urlPrefix != rawURLPrefix || urlPrefix == "/" || filepath.Ext(urlPrefix) != "" {
			continue
		}
		if !filepath.IsAbs(rawFilesystemRoot) {
			continue
		}
		filesystemRoot := filepath.Clean(rawFilesystemRoot)
		if filesystemRoot != cleanRoot {
			continue
		}
		if matched != "" && matched != urlPrefix {
			return "", false
		}
		matched = urlPrefix
	}
	return matched, matched != ""
}
