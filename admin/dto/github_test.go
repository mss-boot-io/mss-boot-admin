package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOauthTokenCannotSerializeProviderCredentials(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	payload, err := json.Marshal(OauthToken{
		Provider:      "github",
		Intent:        "login",
		AccessToken:   "provider-access-token",
		TokenType:     "bearer",
		RefreshToken:  "provider-refresh-token",
		Expiry:        &expiry,
		RefreshExpiry: &expiry,
	})
	if err != nil {
		t.Fatalf("marshal internal OAuth token: %v", err)
	}
	serialized := string(payload)
	for _, forbidden := range []string{
		"provider-access-token",
		"provider-refresh-token",
		"accessToken",
		"refreshToken",
		"tokenType",
		"expiry",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("internal OAuth token serialized %q: %s", forbidden, serialized)
		}
	}
}
