package service

import (
	"regexp"
	"unicode/utf8"
)

const operationalRedactionMarker = "[REDACTED]"

var operationalSecretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern: regexp.MustCompile(
			`(?i)("(?:password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|secret|client[_-]?secret|api[_-]?key|authorization|cookie|set-cookie)"\s*:\s*)("(?:\\.|[^"\\])*"|[^,\s}\]]+)`,
		),
		replacement: `${1}"` + operationalRedactionMarker + `"`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(\bBearer\s+)[A-Za-z0-9._~+/=-]+`),
		replacement: `${1}` + operationalRedactionMarker,
	},
	{
		pattern: regexp.MustCompile(
			`(?i)\b(password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|secret|client[_-]?secret|api[_-]?key|authorization|cookie|set-cookie)(\s*[=:]\s*)(?:"[^"]*"|'[^']*'|[^\s,;&#]+)`,
		),
		replacement: `${1}${2}` + operationalRedactionMarker,
	},
	{
		pattern: regexp.MustCompile(
			`(?i)([?&](?:password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|secret|client[_-]?secret|api[_-]?key)=)[^&#\s]+`,
		),
		replacement: `${1}` + operationalRedactionMarker,
	},
}

// RedactOperationalText removes common credential forms before operational
// data is serialized to a browser or export. The byte limit also prevents a
// single diagnostic value from dominating a response.
func RedactOperationalText(value string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = 4_096
	}
	scanLimit := maxBytes * 4
	if scanLimit < maxBytes || scanLimit > 1<<20 {
		scanLimit = 1 << 20
	}
	value = truncateOperationalUTF8(value, scanLimit)
	for _, item := range operationalSecretPatterns {
		value = item.pattern.ReplaceAllString(value, item.replacement)
	}
	return truncateOperationalUTF8(value, maxBytes)
}

func truncateOperationalUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	const marker = "…"
	if maxBytes < len(marker) {
		return ""
	}
	if maxBytes == len(marker) {
		return marker
	}
	value = value[:maxBytes-len(marker)]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + marker
}
