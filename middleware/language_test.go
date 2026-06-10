package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLanguageMiddlewareFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		acceptLanguage  string
		contentLanguage string
		query           string
		want            string
	}{
		{
			name: "empty header uses default language",
			want: DefaultLang,
		},
		{
			name:           "supported accept language is preserved",
			acceptLanguage: "en-US",
			want:           "en-US",
		},
		{
			name:           "accept language quality suffix is ignored",
			acceptLanguage: "zh-CN;q=0.9,en-US;q=0.8",
			want:           "zh-CN",
		},
		{
			name:           "regional language falls back to supported family",
			acceptLanguage: "en-GB",
			want:           "en-US",
		},
		{
			name:           "unsupported language falls back to default",
			acceptLanguage: "fr-FR",
			want:           DefaultLang,
		},
		{
			name:  "query language is used when accept language is empty",
			query: "?lang=en",
			want:  "en",
		},
		{
			name:            "content language is used after empty accept language and query",
			contentLanguage: "zh-TW",
			want:            "zh-CN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Language())
			router.GET("/language", func(c *gin.Context) {
				lang := GetLanguage(c)
				if lang != tt.want {
					t.Fatalf("GetLanguage() = %q, want %q", lang, tt.want)
				}
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/language"+tt.query, nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			if tt.contentLanguage != "" {
				req.Header.Set("Content-Language", tt.contentLanguage)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
			}
			if got := rec.Header().Get("Content-Language"); got != tt.want {
				t.Fatalf("Content-Language = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLanguageHelpers(t *testing.T) {
	if got := parseAcceptLanguage(" en-US;q=0.8, zh-CN;q=0.7 "); got != "en-US" {
		t.Fatalf("parseAcceptLanguage() = %q, want en-US", got)
	}

	if !IsSupportedLanguage("zh-CN") {
		t.Fatalf("zh-CN should be supported")
	}
	if IsSupportedLanguage("fr-FR") {
		t.Fatalf("fr-FR should not be supported")
	}
}
