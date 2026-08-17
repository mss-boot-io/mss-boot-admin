package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"github.com/spf13/cast"
)

const (
	BrowserSessionCookieName = "mss_admin_session"
	BrowserCSRFCookieName    = "mss_csrf"
	BrowserCSRFHeaderName    = "X-CSRF-Token"

	browserSessionCookiePath = "/admin/api"
	browserCSRFCookiePath    = "/"
	browserCSRFVersion       = "v1"
	browserCSRFNonceBytes    = 32
)

var errBrowserSessionUnavailable = errors.New("browser session is unavailable")

// BrowserSessionLoginHandler authenticates with the canonical verifier but
// returns no bearer credential. The signed JWT is stored only in an HttpOnly,
// host-only cookie and is paired with a JWT-bound CSRF cookie.
func BrowserSessionLoginHandler(c *gin.Context) {
	setBrowserNoStoreHeaders(c)
	if !browserSessionReady() {
		writeAuthErrorResponse(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, errBrowserSessionUnavailable.Error())
		return
	}
	if !IsTrustedBrowserOrigin(c) {
		writeAuthErrorResponse(c, http.StatusForbidden, http.StatusForbidden, "browser origin is not trusted")
		return
	}
	if Auth == nil || Auth.Authenticator == nil {
		writeAuthErrorResponse(c, http.StatusInternalServerError, http.StatusInternalServerError, "authentication middleware is not initialized")
		return
	}
	defer loginContext.Clear()
	c.Set(publicLoginDisallowOAuthKey, true)
	principal, err := Auth.Authenticator(c)
	if err != nil {
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "authentication failed")
		return
	}
	token, expiresAt, err := Auth.TokenGenerator(principal)
	if err != nil {
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "authentication token creation failed")
		return
	}
	if err = SetBrowserSessionCookies(c, token, expiresAt); err != nil {
		writeAuthErrorResponse(c, http.StatusInternalServerError, http.StatusInternalServerError, "browser session creation failed")
		return
	}
	c.JSON(http.StatusOK, dto.BrowserSessionResponse{Code: http.StatusOK, Expire: expiresAt})
}

// BrowserSessionRefreshHandler rotates both the JWT cookie and its bound CSRF
// token. It reuses gin-jwt's signed refresh window while reloading current
// authority and checking the server-side session before issuing the cookie.
func BrowserSessionRefreshHandler(c *gin.Context) {
	setBrowserNoStoreHeaders(c)
	if !browserSessionReady() {
		writeAuthErrorResponse(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, errBrowserSessionUnavailable.Error())
		return
	}
	if !RequestUsesBrowserSession(c) {
		writeAuthErrorResponse(c, http.StatusUnauthorized, http.StatusUnauthorized, "browser session credential is required")
		return
	}
	if err := validateBrowserCSRFRequest(c, browserSessionCookie(c)); err != nil {
		writeAuthErrorResponse(c, http.StatusForbidden, http.StatusForbidden, "csrf validation failed")
		return
	}
	token, expiresAt, err := Auth.RefreshToken(c)
	if err != nil {
		ClearBrowserSessionCookies(c)
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "browser session refresh failed")
		return
	}
	parsed, err := Auth.ParseTokenString(token)
	if err != nil {
		ClearBrowserSessionCookies(c)
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "browser session refresh failed")
		return
	}
	claims := jwt.ExtractClaimsFromToken(parsed)
	if cast.ToBool(claims["refreshTokenDisabled"]) {
		ClearBrowserSessionCookies(c)
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "browser session refresh is disabled")
		return
	}
	principal, err := currentPrincipalFromClaims(c, claims)
	if err != nil || !validateSessionFromClaims(c, claims, principal) {
		ClearBrowserSessionCookies(c)
		writeUnauthorizedAuthResponse(c, http.StatusUnauthorized, "browser session is no longer active")
		return
	}
	if err = SetBrowserSessionCookies(c, token, expiresAt); err != nil {
		ClearBrowserSessionCookies(c)
		writeAuthErrorResponse(c, http.StatusInternalServerError, http.StatusInternalServerError, "browser session refresh failed")
		return
	}
	c.JSON(http.StatusOK, dto.BrowserSessionResponse{Code: http.StatusOK, Expire: expiresAt})
}

// EnforceBrowserCSRF protects every unsafe API operation that would otherwise
// authenticate from a browser cookie. A syntactically present bearer header is
// left to the JWT middleware so API clients retain their explicit bearer
// contract; an invalid bearer cannot fall back to a browser cookie.
func EnforceBrowserCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isUnsafeMethod(c.Request.Method) ||
			hasBearerHeader(c) || browserCSRFExempt(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}
		token := authenticationCookieToken(c)
		if token == "" {
			c.Next()
			return
		}
		if err := validateBrowserCSRFRequest(c, token); err != nil {
			writeAuthErrorResponse(c, http.StatusForbidden, http.StatusForbidden, "csrf validation failed")
			return
		}
		c.Next()
	}
}

func RequireTrustedBrowserOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsTrustedBrowserOrigin(c) {
			writeAuthErrorResponse(c, http.StatusForbidden, http.StatusForbidden, "browser origin is not trusted")
			return
		}
		c.Next()
	}
}

func IsTrustedBrowserOrigin(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return browsersecurity.IsTrustedOrigin(
		c.GetHeader("Origin"),
		config.Cfg.Application.Origin,
		config.Cfg.CORS.AllowOrigins,
	)
}

// RequestUsesBrowserSession reports whether a route is using the dedicated V6
// cookie rather than an explicit API bearer header. Query parameters are never
// credentials and cannot disable browser CSRF enforcement.
func RequestUsesBrowserSession(c *gin.Context) bool {
	if c == nil || c.Request == nil || hasBearerHeader(c) {
		return false
	}
	return browserSessionCookie(c) != ""
}

// SetBrowserSessionCookies writes a host-only HttpOnly JWT cookie and a
// readable signed double-submit token. The CSRF token is cryptographically
// bound to this exact JWT so a sibling origin cannot forge a matching pair.
func SetBrowserSessionCookies(c *gin.Context, token string, expiresAt time.Time) error {
	if c == nil || strings.TrimSpace(token) == "" || !browserSessionReady() {
		return errBrowserSessionUnavailable
	}
	csrfToken, err := issueBrowserCSRFToken(token, []byte(config.Cfg.Auth.Key))
	if err != nil {
		return err
	}
	now := time.Now()
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		return errors.New("browser session expiry must be in the future")
	}
	secure := config.Cfg.Auth.BrowserSession.Secure || requestIsHTTPS(c)
	sameSite := config.Cfg.Auth.BrowserSession.CookieSameSite()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     BrowserSessionCookieName,
		Value:    token,
		Path:     browserSessionCookiePath,
		Expires:  now.Add(time.Duration(maxAge) * time.Second),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     BrowserCSRFCookieName,
		Value:    csrfToken,
		Path:     browserCSRFCookiePath,
		Expires:  now.Add(time.Duration(maxAge) * time.Second),
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
	setBrowserNoStoreHeaders(c)
	return nil
}

func ClearBrowserSessionCookies(c *gin.Context) {
	if c == nil {
		return
	}
	secure := config.Cfg.Auth.BrowserSession.Secure || requestIsHTTPS(c)
	sameSite := config.Cfg.Auth.BrowserSession.CookieSameSite()
	for _, cookie := range []http.Cookie{
		{Name: BrowserSessionCookieName, Path: browserSessionCookiePath, HttpOnly: true},
		{Name: BrowserCSRFCookieName, Path: browserCSRFCookiePath},
	} {
		cookie.Value = ""
		cookie.Expires = time.Unix(1, 0)
		cookie.MaxAge = -1
		cookie.Secure = secure
		cookie.SameSite = sameSite
		http.SetCookie(c.Writer, &cookie)
	}
	setBrowserNoStoreHeaders(c)
}

func browserSessionReady() bool {
	return Auth != nil
}

// BrowserSessionAvailable exposes the configured runtime capability to API
// wrappers without exposing cookie construction details.
func BrowserSessionAvailable() bool {
	return browserSessionReady()
}

func browserSessionCookie(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, err := c.Cookie(BrowserSessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// authenticationCookieToken mirrors the cookie portion of TokenLookup.
func authenticationCookieToken(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value := browserSessionCookie(c); value != "" {
		return value
	}
	return ""
}

func isUnsafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func hasBearerHeader(c *gin.Context) bool {
	if c == nil {
		return false
	}
	parts := strings.Fields(c.GetHeader("Authorization"))
	return len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != ""
}

func browserCSRFExempt(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/admin/api/user/session/login", "/admin/api/user/auth-cookie/clear":
		return true
	default:
		return false
	}
}

func validateBrowserCSRFRequest(c *gin.Context, sessionToken string) error {
	if !IsTrustedBrowserOrigin(c) {
		return errors.New("browser origin is not trusted")
	}
	cookieToken, err := c.Cookie(BrowserCSRFCookieName)
	if err != nil || strings.TrimSpace(cookieToken) == "" {
		return errors.New("csrf cookie is missing")
	}
	headerToken := strings.TrimSpace(c.GetHeader(BrowserCSRFHeaderName))
	if headerToken == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
		return errors.New("csrf header does not match cookie")
	}
	return validateBrowserCSRFToken(headerToken, sessionToken, []byte(config.Cfg.Auth.Key))
}

func issueBrowserCSRFToken(sessionToken string, key []byte) (string, error) {
	if sessionToken == "" || len(key) == 0 {
		return "", errors.New("csrf binding material is missing")
	}
	nonce := make([]byte, browserCSRFNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	encodedNonce := base64.RawURLEncoding.EncodeToString(nonce)
	signature := browserCSRFSignature(encodedNonce, sessionToken, key)
	return browserCSRFVersion + "." + encodedNonce + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func validateBrowserCSRFToken(value, sessionToken string, key []byte) error {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != browserCSRFVersion || sessionToken == "" || len(key) == 0 {
		return errors.New("csrf token is malformed")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != browserCSRFNonceBytes {
		return errors.New("csrf nonce is invalid")
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size {
		return errors.New("csrf signature is invalid")
	}
	expected := browserCSRFSignature(parts[1], sessionToken, key)
	if subtle.ConstantTimeCompare(provided, expected) != 1 {
		return errors.New("csrf token does not belong to this session")
	}
	return nil
}

func browserCSRFSignature(encodedNonce, sessionToken string, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(browserCSRFVersion))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(encodedNonce))
	_, _ = mac.Write([]byte{0})
	tokenDigest := sha256.Sum256([]byte(sessionToken))
	_, _ = mac.Write(tokenDigest[:])
	return mac.Sum(nil)
}

func setBrowserNoStoreHeaders(c *gin.Context) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
}

func requestIsHTTPS(c *gin.Context) bool {
	return c != nil && c.Request != nil &&
		(c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"))
}
