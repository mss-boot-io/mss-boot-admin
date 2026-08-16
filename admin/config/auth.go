package config

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 23:22:37
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 23:22:37
 */

type Auth struct {
	Realm          string         `yaml:"realm" json:"realm"`
	Key            string         `yaml:"key" json:"key"`
	IdentityKey    string         `yaml:"identityKey" json:"identityKey"`
	Timeout        time.Duration  `yaml:"timeout" json:"timeout"`
	MaxRefresh     time.Duration  `yaml:"maxRefresh" json:"maxRefresh"`
	SessionEnabled bool           `yaml:"sessionEnabled" json:"sessionEnabled"`
	BrowserSession BrowserSession `yaml:"browserSession" json:"browserSession"`
}

const insecureDefaultAuthKey = "mss-boot-admin-secret"

const (
	defaultWebSocketTicketTTL = 30 * time.Second
	minimumWebSocketTicketTTL = 5 * time.Second
	maximumWebSocketTicketTTL = 2 * time.Minute
)

// BrowserSession is an opt-in compatibility surface for the independently
// deployed V6 frontend. Fixed host-only cookie names and paths are owned by the
// auth middleware; deployment configuration can enable the capability, require
// Secure cookies, select Lax or Strict SameSite behavior, and bound one-time
// WebSocket tickets. The legacy query-token switch is intentionally separate
// so V5 can remain available during the overlap window without making query
// credentials valid for general REST authentication.
type BrowserSession struct {
	Enabled                          bool          `yaml:"enabled" json:"enabled"`
	Secure                           bool          `yaml:"secure" json:"secure"`
	SameSite                         string        `yaml:"sameSite" json:"sameSite"`
	WebSocketTicketTTL               time.Duration `yaml:"webSocketTicketTTL" json:"webSocketTicketTTL"`
	LegacyWebSocketQueryTokenEnabled *bool         `yaml:"legacyWebSocketQueryTokenEnabled" json:"legacyWebSocketQueryTokenEnabled"`
}

func (e BrowserSession) CookieSameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(e.SameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

func (e BrowserSession) TicketTTL() time.Duration {
	if e.WebSocketTicketTTL <= 0 {
		return defaultWebSocketTicketTTL
	}
	return e.WebSocketTicketTTL
}

// LegacyWebSocketQueryTokenAllowed defaults to true when an older external
// configuration has no knowledge of the new switch. Only an explicit false
// retires the V5 route, preventing an additive backend rollout from silently
// disconnecting the independently deployed legacy frontend.
func (e BrowserSession) LegacyWebSocketQueryTokenAllowed() bool {
	return e.LegacyWebSocketQueryTokenEnabled == nil || *e.LegacyWebSocketQueryTokenEnabled
}

func validateProductionAuthKey(mode Mode, key string) error {
	if mode != ModeProd {
		return nil
	}
	normalized := strings.TrimSpace(key)
	if normalized == "" || normalized == insecureDefaultAuthKey {
		return errors.New("production auth.key must override the insecure development default")
	}
	if len(normalized) < 32 {
		return errors.New("production auth.key must contain at least 32 bytes of entropy-bearing material")
	}
	return nil
}

func validateBrowserSession(mode Mode, auth Auth) error {
	if !auth.BrowserSession.Enabled {
		return nil
	}
	if !auth.SessionEnabled {
		return errors.New("auth.browserSession requires auth.sessionEnabled")
	}
	if strings.TrimSpace(auth.Key) == "" {
		return errors.New("auth.browserSession requires auth.key")
	}
	if strings.TrimSpace(auth.IdentityKey) == "" {
		return errors.New("auth.browserSession requires auth.identityKey")
	}
	if auth.Timeout <= 0 {
		return errors.New("auth.browserSession requires a positive auth.timeout")
	}
	if auth.MaxRefresh <= 0 {
		return errors.New("auth.browserSession requires a positive auth.maxRefresh")
	}
	if mode == ModeProd && !auth.BrowserSession.Secure {
		return errors.New("production auth.browserSession.secure must be true")
	}
	sameSite := strings.ToLower(strings.TrimSpace(auth.BrowserSession.SameSite))
	if sameSite != "" && sameSite != "lax" && sameSite != "strict" {
		return errors.New("auth.browserSession.sameSite must be lax or strict")
	}
	ticketTTL := auth.BrowserSession.TicketTTL()
	if ticketTTL < minimumWebSocketTicketTTL || ticketTTL > maximumWebSocketTicketTTL {
		return errors.New("auth.browserSession.webSocketTicketTTL must be between 5s and 2m")
	}
	return nil
}

func validateBrowserSessionOrigins(
	mode Mode,
	auth Auth,
	applicationOrigin string,
	corsOrigins []string,
	corsHeaders []string,
) error {
	if !auth.BrowserSession.Enabled {
		return nil
	}
	applicationNormalized, _ := browsersecurity.NormalizeOrigin(applicationOrigin)
	candidates := append([]string{applicationOrigin}, corsOrigins...)
	validOrigins := 0
	requiresCSRFPreflightHeader := false
	for index, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		normalized, ok := browsersecurity.NormalizeOrigin(candidate)
		if !ok {
			return errors.New("auth.browserSession requires exact HTTP(S) application and CORS origins")
		}
		if mode == ModeProd && !strings.HasPrefix(normalized, "https://") {
			return errors.New("production auth.browserSession requires HTTPS application and CORS origins")
		}
		if index > 0 && normalized != applicationNormalized {
			requiresCSRFPreflightHeader = true
		}
		validOrigins++
	}
	if validOrigins == 0 {
		return errors.New("auth.browserSession requires at least one trusted browser origin")
	}
	if requiresCSRFPreflightHeader {
		csrfHeaderAllowed := false
		for _, header := range corsHeaders {
			if strings.EqualFold(strings.TrimSpace(header), "X-CSRF-Token") {
				csrfHeaderAllowed = true
				break
			}
		}
		if !csrfHeaderAllowed {
			return errors.New("auth.browserSession cross-origin requests require CORS allowHeaders to include X-CSRF-Token")
		}
	}
	return nil
}
