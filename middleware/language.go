package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	LanguageKey = "accept-language"
	DefaultLang = "zh-CN"
)

var supportedLanguages = map[string]bool{
	"zh-CN": true,
	"en-US": true,
	"zh":    true,
	"en":     true,
}

func Language() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("Accept-Language")
		if lang == "" {
			lang = c.Query("lang")
		}
		if lang == "" {
			lang = c.GetHeader("Content-Language")
		}

		if lang != "" {
			lang = parseAcceptLanguage(lang)
			if !supportedLanguages[lang] {
				lang = normalizeLanguage(lang)
			}
			if !supportedLanguages[lang] {
				lang = DefaultLang
			}
		} else {
			lang = DefaultLang
		}

		c.Set(LanguageKey, lang)
		c.Header("Content-Language", lang)

		c.Next()
	}
}

func parseAcceptLanguage(header string) string {
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return DefaultLang
	}

	lang := strings.TrimSpace(parts[0])
	if idx := strings.Index(lang, ";"); idx != -1 {
		lang = lang[:idx]
	}
	lang = strings.TrimSpace(lang)

	return lang
}

func normalizeLanguage(lang string) string {
	lang = strings.ToLower(lang)
	switch {
	case strings.HasPrefix(lang, "zh"):
		return "zh-CN"
	case strings.HasPrefix(lang, "en"):
		return "en-US"
	default:
		return DefaultLang
	}
}

func GetLanguage(c *gin.Context) string {
	if lang, exists := c.Get(LanguageKey); exists {
		if l, ok := lang.(string); ok {
			return l
		}
	}
	return DefaultLang
}

func IsSupportedLanguage(lang string) bool {
	return supportedLanguages[lang]
}

func GetSupportedLanguages() []string {
	langs := make([]string, 0, len(supportedLanguages))
	for lang := range supportedLanguages {
		langs = append(langs, lang)
	}
	return langs
}