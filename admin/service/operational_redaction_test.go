package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestRedactOperationalTextCoversCommonCredentialForms(t *testing.T) {
	raw := `{"password":"json-secret","ok":"visible"} ` +
		`token=kv-secret Authorization: Bearer abc.def.ghi ` +
		`https://example.test/path?api_key=query-secret&safe=yes`
	redacted := RedactOperationalText(raw, 4_096)

	for _, secret := range []string{"json-secret", "kv-secret", "abc.def.ghi", "query-secret"} {
		require.NotContains(t, redacted, secret)
	}
	require.Contains(t, redacted, operationalRedactionMarker)
	require.Contains(t, redacted, `"ok":"visible"`)
	require.Contains(t, redacted, "safe=yes")
}

func TestRedactOperationalTextBoundsUTF8Output(t *testing.T) {
	redacted := RedactOperationalText(strings.Repeat("界", 100), 31)
	require.LessOrEqual(t, len(redacted), 31)
	require.True(t, strings.HasSuffix(redacted, "…"))
	require.True(t, utf8.ValidString(redacted))

	short := RedactOperationalText("secret", 2)
	require.Empty(t, short)
	require.True(t, utf8.ValidString(short))
}
