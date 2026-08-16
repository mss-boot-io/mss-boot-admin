// Package browsersecurity owns exact browser-origin normalization shared by
// credentialed HTTP and WebSocket entrypoints.
package browsersecurity

import (
	"net"
	"net/url"
	"strings"
)

// NormalizeOrigin accepts only an HTTP(S) origin without credentials, path,
// query, or fragment. It lowercases scheme and hostname and removes the
// protocol's default port so configured and browser-supplied values compare
// deterministically.
func NormalizeOrigin(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "*" || strings.ContainsAny(value, "\r\n\x00") {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return "", false
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return strings.ToLower(parsed.Scheme) + "://" + host, true
}

// IsTrustedOrigin requires an exact match with the configured application
// origin or one of the explicit credentialed CORS origins. Invalid entries and
// wildcards never grant access.
func IsTrustedOrigin(origin, applicationOrigin string, corsOrigins []string) bool {
	normalized, ok := NormalizeOrigin(origin)
	if !ok {
		return false
	}
	candidates := make([]string, 0, len(corsOrigins)+1)
	candidates = append(candidates, applicationOrigin)
	candidates = append(candidates, corsOrigins...)
	for _, candidate := range candidates {
		trusted, valid := NormalizeOrigin(candidate)
		if valid && trusted == normalized {
			return true
		}
	}
	return false
}

// TrustedOrigins returns normalized, de-duplicated origins in declaration
// order. It is suitable for credentialed CORS configuration.
func TrustedOrigins(configured []string) []string {
	trusted := make([]string, 0, len(configured))
	seen := make(map[string]struct{}, len(configured))
	for _, candidate := range configured {
		normalized, ok := NormalizeOrigin(candidate)
		if !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		trusted = append(trusted, normalized)
	}
	return trusted
}
