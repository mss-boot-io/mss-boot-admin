package config

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"gopkg.in/yaml.v3"
)

func TestLocalOverlayDefaultsToV6DevelopmentBrowserSession(t *testing.T) {
	base, err := FS.ReadFile("application.yml")
	if err != nil {
		t.Fatalf("read base configuration: %v", err)
	}
	overlay, err := FS.ReadFile("application-local.yml")
	if err != nil {
		t.Fatalf("read local overlay: %v", err)
	}
	var cfg Config
	if err = yaml.Unmarshal(base, &cfg); err != nil {
		t.Fatalf("decode base configuration: %v", err)
	}
	if err = yaml.Unmarshal(overlay, &cfg); err != nil {
		t.Fatalf("decode V6 local overlay: %v", err)
	}
	if cfg.Application.Mode != ModeDev || cfg.Application.Origin != "http://localhost:8001" {
		t.Fatalf("unexpected primary local application profile: %#v", cfg.Application)
	}
	if cfg.Auth.BrowserSession.Secure {
		t.Fatalf("unexpected primary local browser-session profile: %#v", cfg.Auth)
	}
	if err = validateBrowserSession(cfg.Application.Mode, cfg.Auth); err != nil {
		t.Fatalf("validate primary local browser session: %v", err)
	}
	if err = validateBrowserSessionOrigins(
		cfg.Application.Mode,
		cfg.Auth,
		cfg.Application.Origin,
		cfg.CORS.AllowOrigins,
		cfg.CORS.AllowHeaders,
	); err != nil {
		t.Fatalf("validate primary local browser origins: %v", err)
	}
}

func TestE2EOverlayProducesAValidBrowserSessionProfile(t *testing.T) {
	base, err := FS.ReadFile("application.yml")
	if err != nil {
		t.Fatalf("read base configuration: %v", err)
	}
	overlay, err := FS.ReadFile("application-e2e.yml")
	if err != nil {
		t.Fatalf("read E2E configuration: %v", err)
	}
	var cfg Config
	if err = yaml.Unmarshal(base, &cfg); err != nil {
		t.Fatalf("decode base configuration: %v", err)
	}
	if err = yaml.Unmarshal(overlay, &cfg); err != nil {
		t.Fatalf("decode E2E configuration: %v", err)
	}
	if cfg.Application.Mode != ModeDev || cfg.Database.Driver != "sqlite" {
		t.Fatalf("unexpected E2E profile: application=%#v database=%#v", cfg.Application, cfg.Database)
	}
	if err = validateBrowserSession(cfg.Application.Mode, cfg.Auth); err != nil {
		t.Fatalf("validate E2E browser session: %v", err)
	}
	if err = validateBrowserSessionOrigins(
		cfg.Application.Mode,
		cfg.Auth,
		cfg.Application.Origin,
		cfg.CORS.AllowOrigins,
		cfg.CORS.AllowHeaders,
	); err != nil {
		t.Fatalf("validate E2E browser origins: %v", err)
	}
	for _, origin := range []string{"http://127.0.0.1:18001", "http://localhost:18001"} {
		if !browsersecurity.IsTrustedOrigin(origin, cfg.Application.Origin, cfg.CORS.AllowOrigins) {
			t.Errorf("E2E browser origin %q is not trusted", origin)
		}
	}
}

func TestValidateProductionAuthKeyFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		key     string
		wantErr bool
	}{
		{name: "development default remains usable", mode: ModeDev, key: insecureDefaultAuthKey},
		{name: "test empty remains usable", mode: ModeTest, key: ""},
		{name: "production empty", mode: ModeProd, key: "", wantErr: true},
		{name: "production whitespace", mode: ModeProd, key: "   ", wantErr: true},
		{name: "production default", mode: ModeProd, key: insecureDefaultAuthKey, wantErr: true},
		{name: "production short", mode: ModeProd, key: "short-but-custom", wantErr: true},
		{name: "production strong override", mode: ModeProd, key: "0123456789abcdef0123456789abcdef"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateProductionAuthKey(test.mode, test.key)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateProductionAuthKey(%q) error = %v, wantErr %v", test.key, err, test.wantErr)
			}
			if err != nil && (contains(err.Error(), test.key) || contains(err.Error(), insecureDefaultAuthKey)) {
				t.Fatalf("validation error leaked auth key material: %v", err)
			}
		})
	}
}

func TestValidateBrowserSessionFailsClosed(t *testing.T) {
	valid := Auth{
		Key:         "browser-session-test-key",
		IdentityKey: "browser-session-test-identity",
		Timeout:     12 * time.Hour,
		MaxRefresh:  30 * 24 * time.Hour,
		BrowserSession: BrowserSession{
			Secure:             true,
			SameSite:           "lax",
			WebSocketTicketTTL: 30 * time.Second,
		},
	}
	tests := []struct {
		name    string
		mode    Mode
		mutate  func(*Auth)
		wantErr bool
	}{
		{name: "production valid", mode: ModeProd},
		{name: "development may use insecure cookie", mode: ModeDev, mutate: func(auth *Auth) { auth.BrowserSession.Secure = false }},
		{name: "signing key required", mode: ModeDev, mutate: func(auth *Auth) { auth.Key = "" }, wantErr: true},
		{name: "identity key required", mode: ModeDev, mutate: func(auth *Auth) { auth.IdentityKey = "" }, wantErr: true},
		{name: "positive timeout required", mode: ModeDev, mutate: func(auth *Auth) { auth.Timeout = 0 }, wantErr: true},
		{name: "positive refresh window required", mode: ModeDev, mutate: func(auth *Auth) { auth.MaxRefresh = 0 }, wantErr: true},
		{name: "production secure required", mode: ModeProd, mutate: func(auth *Auth) { auth.BrowserSession.Secure = false }, wantErr: true},
		{name: "same site none rejected", mode: ModeDev, mutate: func(auth *Auth) { auth.BrowserSession.SameSite = "none" }, wantErr: true},
		{name: "ticket too short", mode: ModeDev, mutate: func(auth *Auth) { auth.BrowserSession.WebSocketTicketTTL = time.Second }, wantErr: true},
		{name: "ticket too long", mode: ModeDev, mutate: func(auth *Auth) { auth.BrowserSession.WebSocketTicketTTL = 3 * time.Minute }, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth := valid
			if test.mutate != nil {
				test.mutate(&auth)
			}
			err := validateBrowserSession(test.mode, auth)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateBrowserSession() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestBrowserSessionDefaults(t *testing.T) {
	settings := BrowserSession{}
	if got := settings.CookieSameSite(); got != http.SameSiteLaxMode {
		t.Fatalf("CookieSameSite() = %v, want Lax", got)
	}
	if got := settings.TicketTTL(); got != 30*time.Second {
		t.Fatalf("TicketTTL() = %v, want 30s", got)
	}
	settings.SameSite = "STRICT"
	if got := settings.CookieSameSite(); got != http.SameSiteStrictMode {
		t.Fatalf("CookieSameSite() = %v, want Strict", got)
	}
}

func TestValidateBrowserSessionOrigins(t *testing.T) {
	auth := Auth{}
	tests := []struct {
		name              string
		mode              Mode
		applicationOrigin string
		corsOrigins       []string
		corsHeaders       []string
		wantErr           bool
	}{
		{name: "development exact HTTP", mode: ModeDev, corsOrigins: []string{"http://localhost:8001"}, corsHeaders: []string{"x-csrf-token"}},
		{name: "production exact HTTPS", mode: ModeProd, applicationOrigin: "https://admin.example"},
		{name: "missing", mode: ModeDev, wantErr: true},
		{name: "wildcard", mode: ModeDev, corsOrigins: []string{"*"}, corsHeaders: []string{"X-CSRF-Token"}, wantErr: true},
		{name: "path", mode: ModeDev, corsOrigins: []string{"https://admin.example/path"}, corsHeaders: []string{"X-CSRF-Token"}, wantErr: true},
		{name: "production HTTP", mode: ModeProd, corsOrigins: []string{"http://admin.example"}, corsHeaders: []string{"X-CSRF-Token"}, wantErr: true},
		{name: "cross origin CSRF header missing", mode: ModeProd, applicationOrigin: "https://api.example", corsOrigins: []string{"https://admin.example"}, wantErr: true},
		{name: "cross origin CSRF header present", mode: ModeProd, applicationOrigin: "https://api.example", corsOrigins: []string{"https://admin.example"}, corsHeaders: []string{"Content-Type", "X-CSRF-Token"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateBrowserSessionOrigins(test.mode, auth, test.applicationOrigin, test.corsOrigins, test.corsHeaders)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateBrowserSessionOrigins() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func contains(value, secret string) bool {
	return secret != "" && len(secret) > 4 && strings.Contains(value, secret)
}
