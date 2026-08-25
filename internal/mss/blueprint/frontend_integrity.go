package blueprint

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultFrontendRegistryURL = "https://registry.npmjs.org"
	frontendIntegrityToken     = "__MSS_DISTRIBUTION_FRONTEND_INTEGRITY__"
	maxFrontendMetadataBytes   = 1 << 20
)

// resolveFrontendIntegrityForSource resolves the immutable npm tarball SRI
// only when a selected application template actually consumes it. Exact npm
// versions are immutable, so the resolved value becomes part of the generated
// snapshot and subsequent three-way upgrade baseline.
func resolveFrontendIntegrityForSource(
	ctx context.Context,
	registryURL string,
	blueprint *Document,
	sourceFiles []blueprintSourceFile,
) (string, error) {
	needsIntegrity := false
	for _, sourceFile := range sourceFiles {
		if strings.Contains(string(sourceFile.Data), frontendIntegrityToken) {
			needsIntegrity = true
			break
		}
	}
	if !needsIntegrity {
		return "", nil
	}
	if blueprint == nil {
		return "", errors.New("application blueprint is required to resolve frontend integrity")
	}
	frontend := blueprint.Spec.Distribution.Frontend
	integrity, err := resolveFrontendIntegrity(ctx, registryURL, frontend.Package, frontend.Version)
	if err != nil {
		return "", fmt.Errorf("resolve frozen frontend %s@%s: %w", frontend.Package, frontend.Version, err)
	}
	return integrity, nil
}

func resolveFrontendIntegrity(ctx context.Context, registryURL, packageName, version string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint, err := frontendMetadataEndpoint(registryURL, packageName, version)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create npm metadata request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.npm.install-v1+json, application/json")
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("npm metadata redirects are not allowed")
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch exact npm metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("npm registry returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxFrontendMetadataBytes+1))
	if err != nil {
		return "", fmt.Errorf("read npm metadata: %w", err)
	}
	if len(data) > maxFrontendMetadataBytes {
		return "", fmt.Errorf("npm metadata exceeds %d bytes", maxFrontendMetadataBytes)
	}
	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dist    struct {
			Integrity string `json:"integrity"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("decode npm metadata: %w", err)
	}
	if metadata.Name != packageName || metadata.Version != version {
		return "", fmt.Errorf(
			"npm metadata identity mismatch: requested %s@%s, received %s@%s",
			packageName,
			version,
			metadata.Name,
			metadata.Version,
		)
	}
	if !validFrontendIntegrity(metadata.Dist.Integrity) {
		return "", errors.New("npm metadata dist.integrity must be one exact sha512 SRI")
	}
	return metadata.Dist.Integrity, nil
}

func frontendMetadataEndpoint(registryURL, packageName, version string) (string, error) {
	packageName = strings.TrimSpace(packageName)
	version = strings.TrimSpace(version)
	if packageName == "" || version == "" || strings.ContainsAny(version, "/?#") {
		return "", errors.New("exact npm package and version are required")
	}
	registryURL = strings.TrimSpace(registryURL)
	if registryURL == "" {
		registryURL = defaultFrontendRegistryURL
	}
	registry, err := url.Parse(registryURL)
	if err != nil {
		return "", fmt.Errorf("parse npm registry URL: %w", err)
	}
	if registry.User != nil || registry.RawQuery != "" || registry.Fragment != "" ||
		(registry.Path != "" && registry.Path != "/") {
		return "", errors.New("npm registry URL must not contain credentials, query, fragment, or a path")
	}
	if registryURL == defaultFrontendRegistryURL {
		if registry.Scheme != "https" || registry.Host != "registry.npmjs.org" {
			return "", errors.New("default npm registry must be https://registry.npmjs.org")
		}
	} else if registry.Scheme != "http" || !loopbackRegistryHost(registry.Hostname()) {
		return "", errors.New("contributor npm registry override must be an explicit loopback HTTP endpoint")
	}
	plainPath := "/" + packageName + "/" + version
	registry.Path = plainPath
	registry.RawPath = "/" + url.PathEscape(packageName) + "/" + url.PathEscape(version)
	return registry.String(), nil
}

func loopbackRegistryHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func validFrontendIntegrity(value string) bool {
	if !strings.HasPrefix(value, "sha512-") || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "sha512-"))
	return err == nil && len(digest) == 64
}
