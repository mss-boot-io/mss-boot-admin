package middleware

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeAuditRequestRedactsNestedSecrets(t *testing.T) {
	body := []byte(`{
		"source":"https://github.com/example/template",
		"status":"enabled",
		"password":"provider-password",
		"accessToken":"provider-token",
		"oauth":{"refresh_token":"refresh-token","code":"oauth-code","state":"oauth-state"},
		"items":[{"clientSecret":"client-secret"},{"credential":"opaque-handle"}]
	}`)

	result := sanitizeAuditRequest(body)
	if result == "" {
		t.Fatal("sanitized JSON should retain safe audit metadata")
	}
	for _, secret := range []string{
		"provider-password",
		"provider-token",
		"refresh-token",
		"oauth-code",
		"oauth-state",
		"client-secret",
		"opaque-handle",
	} {
		if strings.Contains(result, secret) {
			t.Fatalf("sanitized audit request contains %q: %s", secret, result)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("decode sanitized request: %v", err)
	}
	if decoded["source"] != "https://github.com/example/template" || decoded["status"] != "enabled" {
		t.Fatalf("safe metadata changed: %#v", decoded)
	}
	if decoded["password"] != auditRedactedValue || decoded["accessToken"] != auditRedactedValue {
		t.Fatalf("top-level credentials were not redacted: %#v", decoded)
	}
}

func TestSanitizeAuditRequestFailsClosedForOpaqueBodies(t *testing.T) {
	for _, body := range [][]byte{
		[]byte("password=secret&token=value"),
		[]byte("not-json"),
		nil,
	} {
		if result := sanitizeAuditRequest(body); result != "" {
			t.Fatalf("opaque body should not be audited, got %q", result)
		}
	}
}

func TestSensitiveAuditKeyDoesNotRedactOrdinaryStateWords(t *testing.T) {
	for _, key := range []string{"status", "statement", "source", "template"} {
		if isSensitiveAuditKey(key) {
			t.Fatalf("ordinary key %q should not be redacted", key)
		}
	}
	for _, key := range []string{"state", "access_token", "client-secret", "apiKey", "newPassword"} {
		if !isSensitiveAuditKey(key) {
			t.Fatalf("sensitive key %q should be redacted", key)
		}
	}
}
