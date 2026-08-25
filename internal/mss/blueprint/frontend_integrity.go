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
	frontendTarballToken       = "__MSS_DISTRIBUTION_FRONTEND_TARBALL__"
	maxFrontendMetadataBytes   = 1 << 20
)

type frontendPackageResolution struct {
	Integrity string
	Tarball   string
}

// resolveFrontendPackageForSource resolves the immutable npm tarball URL and
// SRI only when a selected application template actually consumes them. Exact
// npm versions are immutable, so both values become part of the generated
// snapshot and subsequent three-way upgrade baseline.
func resolveFrontendPackageForSource(
	ctx context.Context,
	registryURL string,
	blueprint *Document,
	sourceFiles []blueprintSourceFile,
) (frontendPackageResolution, error) {
	needsIntegrity := false
	needsTarball := false
	for _, sourceFile := range sourceFiles {
		text := string(sourceFile.Data)
		needsIntegrity = needsIntegrity || strings.Contains(text, frontendIntegrityToken)
		needsTarball = needsTarball || strings.Contains(text, frontendTarballToken)
	}
	if !needsIntegrity && !needsTarball {
		return frontendPackageResolution{}, nil
	}
	if !needsIntegrity || !needsTarball {
		return frontendPackageResolution{}, errors.New("application template must consume frontend tarball and integrity together")
	}
	if blueprint == nil {
		return frontendPackageResolution{}, errors.New("application blueprint is required to resolve frontend package")
	}
	frontend := blueprint.Spec.Distribution.Frontend
	resolved, err := resolveFrontendPackage(ctx, registryURL, frontend.Package, frontend.Version)
	if err != nil {
		return frontendPackageResolution{}, fmt.Errorf("resolve frozen frontend %s@%s: %w", frontend.Package, frontend.Version, err)
	}
	return resolved, nil
}

func resolveFrontendPackage(ctx context.Context, registryURL, packageName, version string) (frontendPackageResolution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint, err := frontendMetadataEndpoint(registryURL, packageName, version)
	if err != nil {
		return frontendPackageResolution{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return frontendPackageResolution{}, fmt.Errorf("create npm metadata request: %w", err)
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
		return frontendPackageResolution{}, fmt.Errorf("fetch exact npm metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return frontendPackageResolution{}, fmt.Errorf("npm registry returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxFrontendMetadataBytes+1))
	if err != nil {
		return frontendPackageResolution{}, fmt.Errorf("read npm metadata: %w", err)
	}
	if len(data) > maxFrontendMetadataBytes {
		return frontendPackageResolution{}, fmt.Errorf("npm metadata exceeds %d bytes", maxFrontendMetadataBytes)
	}
	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dist    struct {
			Integrity string `json:"integrity"`
			Tarball   string `json:"tarball"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return frontendPackageResolution{}, fmt.Errorf("decode npm metadata: %w", err)
	}
	if metadata.Name != packageName || metadata.Version != version {
		return frontendPackageResolution{}, fmt.Errorf(
			"npm metadata identity mismatch: requested %s@%s, received %s@%s",
			packageName,
			version,
			metadata.Name,
			metadata.Version,
		)
	}
	if !validFrontendIntegrity(metadata.Dist.Integrity) {
		return frontendPackageResolution{}, errors.New("npm metadata dist.integrity must be one exact sha512 SRI")
	}
	tarball, err := validFrontendTarballURL(metadata.Dist.Tarball)
	if err != nil {
		return frontendPackageResolution{}, fmt.Errorf("npm metadata dist.tarball: %w", err)
	}
	return frontendPackageResolution{Integrity: metadata.Dist.Integrity, Tarball: tarball}, nil
}

func validFrontendTarballURL(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\r\n\t") {
		return "", errors.New("must be one absolute stable URL")
	}
	tarball, err := url.Parse(value)
	if err != nil || !tarball.IsAbs() || tarball.Opaque != "" || tarball.Hostname() == "" || tarball.Path == "" {
		return "", errors.New("must be one absolute stable URL")
	}
	if tarball.User != nil {
		return "", errors.New("must not contain credentials")
	}
	if tarball.RawQuery != "" || tarball.Fragment != "" {
		return "", errors.New("must not contain a query or fragment")
	}
	switch tarball.Scheme {
	case "https":
		return tarball.String(), nil
	case "http":
		if loopbackRegistryHost(tarball.Hostname()) {
			return tarball.String(), nil
		}
		return "", errors.New("HTTP is allowed only for an explicit loopback fixture")
	default:
		return "", errors.New("must use HTTPS outside an explicit loopback fixture")
	}
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
