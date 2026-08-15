package websocket

import (
	"net/http/httptest"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
)

func TestIsTrustedOriginUsesExactConfiguredOrigins(t *testing.T) {
	previousApplication := config.Cfg.Application
	previousCORS := config.Cfg.CORS
	t.Cleanup(func() {
		config.Cfg.Application = previousApplication
		config.Cfg.CORS = previousCORS
	})
	config.Cfg.Application.Origin = "https://api.example"
	config.Cfg.CORS.AllowOrigins = []string{"https://admin.example", "*"}

	request := httptest.NewRequest("GET", "/admin/api/ws/connect-v6", nil)
	request.Header.Set("Origin", "https://admin.example")
	if !IsTrustedOrigin(request) {
		t.Fatal("configured origin was rejected")
	}
	for _, origin := range []string{"", "null", "https://admin.example.attacker.test", "https://attacker.test"} {
		request.Header.Set("Origin", origin)
		if IsTrustedOrigin(request) {
			t.Fatalf("origin %q was accepted", origin)
		}
	}
}

func TestV6UpgraderSelectsOnlyApplicationProtocol(t *testing.T) {
	if len(upgrader.Subprotocols) != 1 || upgrader.Subprotocols[0] != ApplicationSubprotocol {
		t.Fatalf("upgrader subprotocols = %#v", upgrader.Subprotocols)
	}
}
