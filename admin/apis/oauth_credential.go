package apis

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/oauthcredential"
	"github.com/redis/go-redis/v9"
)

const oauthCredentialHeader = "X-MSS-OAuth-Credential"

type oauthCredentialStore interface {
	Issue(context.Context, redis.UniversalClient, oauthcredential.Record, time.Duration) (string, oauthcredential.Record, error)
	Lookup(context.Context, redis.UniversalClient, string) (oauthcredential.Record, error)
	Consume(context.Context, redis.UniversalClient, string) (oauthcredential.Record, error)
	Delete(context.Context, redis.UniversalClient, string) error
}

var (
	defaultOAuthCredentialOnce     sync.Once
	defaultOAuthCredentialStore    oauthCredentialStore
	defaultOAuthCredentialStoreErr error
)

// defaultOAuthCredentials lazily derives one process-wide store from the
// configured application signing secret. Initialization happens on first use
// because config.Cfg is populated after package initialization.
func defaultOAuthCredentials() (oauthCredentialStore, error) {
	defaultOAuthCredentialOnce.Do(func() {
		defaultOAuthCredentialStore, defaultOAuthCredentialStoreErr = oauthcredential.New([]byte(config.Cfg.Auth.Key))
	})
	return defaultOAuthCredentialStore, defaultOAuthCredentialStoreErr
}

func (e Template) credentialStore() (oauthCredentialStore, error) {
	if e.oauthCredentials != nil {
		return e.oauthCredentials, nil
	}
	return defaultOAuthCredentials()
}

func (e Template) lookupOAuthIntegrationCredential(c *gin.Context) (string, string, int) {
	if c == nil || strings.TrimSpace(c.GetHeader(oauthCredentialHeader)) == "" {
		return lookupOAuthIntegrationCredential(c, nil)
	}
	store, err := e.credentialStore()
	if err != nil {
		return "", "", http.StatusServiceUnavailable
	}
	return lookupOAuthIntegrationCredential(c, store)
}

func (e Template) consumeOAuthIntegrationCredential(c *gin.Context) (string, string, int) {
	if c == nil || strings.TrimSpace(c.GetHeader(oauthCredentialHeader)) == "" {
		return consumeOAuthIntegrationCredential(c, nil)
	}
	store, err := e.credentialStore()
	if err != nil {
		return "", "", http.StatusServiceUnavailable
	}
	return consumeOAuthIntegrationCredential(c, store)
}

func lookupOAuthIntegrationCredential(
	c *gin.Context,
	store oauthCredentialStore,
) (accessToken string, handle string, status int) {
	if c == nil {
		return "", "", http.StatusUnauthorized
	}
	handle = strings.TrimSpace(c.GetHeader(oauthCredentialHeader))
	if handle == "" {
		// Public repositories remain usable without a provider credential.
		return "", "", 0
	}

	verifier := currentVerifier(c)
	if verifier == nil {
		return "", "", http.StatusUnauthorized
	}
	if middleware.IsPersonalAccessTokenVerifier(verifier) {
		return "", "", http.StatusForbidden
	}
	credentialFingerprint := requestCredentialFingerprint(c)
	userID := strings.TrimSpace(verifier.GetUserID())
	if credentialFingerprint == "" || userID == "" {
		return "", "", http.StatusUnauthorized
	}
	if store == nil {
		return "", "", http.StatusServiceUnavailable
	}

	record, err := store.Lookup(c, center.GetCache(), handle)
	return validateOAuthIntegrationCredentialRecord(c, handle, record, err)
}

func consumeOAuthIntegrationCredential(
	c *gin.Context,
	store oauthCredentialStore,
) (accessToken string, handle string, status int) {
	if c == nil {
		return "", "", http.StatusUnauthorized
	}
	handle = strings.TrimSpace(c.GetHeader(oauthCredentialHeader))
	if handle == "" {
		// Generate always pushes a branch to GitHub. Requiring the short-lived
		// handle here avoids doing clone/generation work that cannot succeed and
		// keeps the state-changing operation fail-closed.
		return "", "", http.StatusUnauthorized
	}
	verifier := currentVerifier(c)
	if verifier == nil {
		return "", "", http.StatusUnauthorized
	}
	if middleware.IsPersonalAccessTokenVerifier(verifier) {
		return "", "", http.StatusForbidden
	}
	if store == nil {
		return "", "", http.StatusServiceUnavailable
	}
	record, err := store.Consume(c, center.GetCache(), handle)
	return validateOAuthIntegrationCredentialRecord(c, handle, record, err)
}

func validateOAuthIntegrationCredentialRecord(
	c *gin.Context,
	handle string,
	record oauthcredential.Record,
	err error,
) (accessToken string, returnedHandle string, status int) {
	if err != nil {
		if errors.Is(err, oauthcredential.ErrNotFound) ||
			errors.Is(err, oauthcredential.ErrExpired) ||
			errors.Is(err, oauthcredential.ErrInvalid) {
			return "", "", http.StatusUnauthorized
		}
		return "", "", http.StatusServiceUnavailable
	}
	verifier := currentVerifier(c)
	if verifier == nil {
		return "", "", http.StatusUnauthorized
	}
	if middleware.IsPersonalAccessTokenVerifier(verifier) {
		return "", "", http.StatusForbidden
	}
	credentialFingerprint := requestCredentialFingerprint(c)
	userID := strings.TrimSpace(verifier.GetUserID())
	if credentialFingerprint == "" || userID == "" {
		return "", "", http.StatusUnauthorized
	}
	if !constantTimeEqual(record.Provider, "github") ||
		!constantTimeEqual(string(record.Intent), string(oauthcredential.IntentIntegration)) ||
		!constantTimeEqual(record.UserID, userID) ||
		!constantTimeEqual(record.CredentialFingerprint, credentialFingerprint) {
		return "", "", http.StatusUnauthorized
	}
	return record.AccessToken, handle, 0
}

func legacyTemplateAccessToken(c *gin.Context, boundValue string) bool {
	if strings.TrimSpace(boundValue) != "" {
		return true
	}
	if c == nil || c.Request == nil {
		return false
	}
	return strings.TrimSpace(c.Query("accessToken")) != "" ||
		strings.TrimSpace(c.PostForm("accessToken")) != ""
}
