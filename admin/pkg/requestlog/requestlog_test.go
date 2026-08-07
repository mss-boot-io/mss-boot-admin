package requestlog

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ginjwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

func TestRedactRawQueryPreservesNonSensitiveParameters(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "ordinary token",
			raw:  "room=alpha&token=header.payload.signature&filter=a%2Bb",
			want: "room=alpha&token=[REDACTED]&filter=a%2Bb",
		},
		{
			name: "case insensitive repeated values",
			raw:  "TOKEN=first&ToKeN=second%2Evalue&token=third",
			want: "TOKEN=[REDACTED]&ToKeN=[REDACTED]&token=[REDACTED]",
		},
		{
			name: "encoded key",
			raw:  "to%6ben=encoded%2Dsecret&not_token=visible",
			want: "to%6ben=[REDACTED]&not_token=visible",
		},
		{
			name: "similar key is retained",
			raw:  "access_token=visible&tokenized=visible-too",
			want: "access_token=visible&tokenized=visible-too",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := redactRawQuery(test.raw); got != test.want {
				t.Fatalf("redactRawQuery(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestLoggerRedactsQueryTokenWithoutChangingJWTAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &ginjwt.GinJWTMiddleware{
		Realm:       "request-log-test",
		Key:         []byte("request-log-test-signing-key-32-bytes"),
		Timeout:     time.Hour,
		IdentityKey: "identity",
		PayloadFunc: func(data any) ginjwt.MapClaims {
			return data.(ginjwt.MapClaims)
		},
		TokenLookup: "query: token",
	}
	if err := auth.MiddlewareInit(); err != nil {
		t.Fatalf("initialize JWT middleware: %v", err)
	}
	token, _, err := auth.TokenGenerator(ginjwt.MapClaims{"identity": "admin"})
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}

	var logs bytes.Buffer
	var handledRawQuery string
	engine := gin.New()
	engine.Use(LoggerWithWriter(&logs), RecoveryWithWriter(&logs))
	engine.GET("/ws/connect", auth.MiddlewareFunc(), func(c *gin.Context) {
		handledRawQuery = c.Request.URL.RawQuery
		if got := c.Query("token"); got != token {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		_ = c.Error(errors.New("request rejected: " + c.Request.URL.RequestURI()))
		c.Status(http.StatusNoContent)
	})

	rawQuery := "room=alpha&token=" + token + "&ToKeN=provider%2Dcredential&token=another%2Esecret&filter=a%2Bb"
	request := httptest.NewRequest(http.MethodGet, "/ws/connect?"+rawQuery, nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("authenticated response status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if handledRawQuery != rawQuery || request.URL.RawQuery != rawQuery {
		t.Fatalf("request RawQuery changed: handler=%q request=%q want=%q", handledRawQuery, request.URL.RawQuery, rawQuery)
	}
	assertNoSecret(t, logs.String(), token, "provider%2Dcredential", "provider-credential", "another%2Esecret", "another.secret")
	for _, retained := range []string{"room=alpha", "filter=a%2Bb", "token=[REDACTED]", "ToKeN=[REDACTED]"} {
		if !strings.Contains(logs.String(), retained) {
			t.Fatalf("access log did not retain %q: %s", retained, logs.String())
		}
	}
}

func TestRecoveryRedactsQueryTokenAndOmitsPanicValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const rawQuery = "keep=visible&TOKEN=panic%2Dcredential&token=second.secret"
	var logs bytes.Buffer
	var handledRawQuery string
	engine := gin.New()
	engine.Use(LoggerWithWriter(&logs), RecoveryWithWriter(&logs))
	engine.GET("/panic", func(c *gin.Context) {
		handledRawQuery = c.Request.URL.RawQuery
		panic("panic included request URI " + c.Request.URL.RequestURI())
	})

	request := httptest.NewRequest(http.MethodGet, "/panic?"+rawQuery, nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("panic response status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if handledRawQuery != rawQuery || request.URL.RawQuery != rawQuery {
		t.Fatalf("panic request RawQuery changed: handler=%q request=%q want=%q", handledRawQuery, request.URL.RawQuery, rawQuery)
	}
	assertNoSecret(t, logs.String(), "panic%2Dcredential", "panic-credential", "second.secret")
	for _, retained := range []string{"keep=visible", "TOKEN=[REDACTED]", "token=[REDACTED]", "panic_type=string"} {
		if !strings.Contains(logs.String(), retained) {
			t.Fatalf("recovery log did not retain %q: %s", retained, logs.String())
		}
	}
}

func assertNoSecret(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Fatalf("log leaked %q: %s", secret, output)
		}
	}
}
