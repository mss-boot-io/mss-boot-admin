// Package requestlog provides Gin access and recovery logging that redacts
// credential-shaped query data even though query-string authentication is not
// supported.
package requestlog

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const redactedQueryValue = "[REDACTED]"

// Logger returns access logging that redacts every case-insensitive token
// query parameter. It deliberately operates on a log-only copy of the path;
// the request URL remains available to authentication and handlers unchanged.
func Logger() gin.HandlerFunc {
	return LoggerWithWriter(gin.DefaultWriter)
}

// LoggerWithWriter is Logger with an explicit destination, primarily for
// focused tests and applications that do not use gin.DefaultWriter.
func LoggerWithWriter(out io.Writer) gin.HandlerFunc {
	if out == nil {
		out = io.Discard
	}
	return func(c *gin.Context) {
		start := time.Now()
		method, path, rawQuery := requestTarget(c.Request)
		safePath := path
		if rawQuery != "" {
			safePath += "?" + redactRawQuery(rawQuery)
		}
		tokenValues := tokenQueryRepresentations(rawQuery)

		c.Next()

		timestamp := time.Now()
		latency := normalizeLatency(timestamp.Sub(start))
		errorMessage := redactKnownValues(
			c.Errors.ByType(gin.ErrorTypePrivate).String(),
			tokenValues,
		)
		fmt.Fprintf(out, "[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			timestamp.Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			method,
			safePath,
			errorMessage,
		)
	}
}

// Recovery returns panic recovery that logs only a redacted request target,
// the panic type, and the stack. Panic values and headers are intentionally
// omitted because either can contain bearer credentials.
func Recovery() gin.HandlerFunc {
	return RecoveryWithWriter(gin.DefaultErrorWriter)
}

// RecoveryWithWriter is Recovery with an explicit destination.
func RecoveryWithWriter(out io.Writer) gin.HandlerFunc {
	var logger *log.Logger
	if out != nil {
		logger = log.New(out, "\n\n", log.LstdFlags)
	}
	return func(c *gin.Context) {
		method, path, rawQuery := requestTarget(c.Request)
		safePath := path
		if rawQuery != "" {
			safePath += "?" + redactRawQuery(rawQuery)
		}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			brokenPipe := isBrokenConnection(recovered)
			if logger != nil {
				if brokenPipe {
					logger.Printf("[Recovery] broken connection: method=%s path=%q panic_type=%T", method, safePath, recovered)
				} else {
					logger.Printf("[Recovery] panic recovered: method=%s path=%q panic_type=%T\n%s", method, safePath, recovered, debug.Stack())
				}
			}
			if brokenPipe {
				if err, ok := recovered.(error); ok {
					_ = c.Error(err)
				}
				c.Abort()
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
		}()

		c.Next()
	}
}

func requestTarget(request *http.Request) (method, path, rawQuery string) {
	if request == nil {
		return "", "", ""
	}
	method = request.Method
	if request.URL == nil {
		return method, "", ""
	}
	return method, request.URL.Path, request.URL.RawQuery
}

func redactRawQuery(rawQuery string) string {
	parts := strings.Split(rawQuery, "&")
	for index, part := range parts {
		rawKey := part
		if equals := strings.IndexByte(part, '='); equals >= 0 {
			rawKey = part[:equals]
		}
		key, err := url.QueryUnescape(rawKey)
		if err == nil && strings.EqualFold(key, "token") {
			parts[index] = rawKey + "=" + redactedQueryValue
		}
	}
	return strings.Join(parts, "&")
}

func tokenQueryRepresentations(rawQuery string) []string {
	seen := make(map[string]struct{})
	for _, part := range strings.Split(rawQuery, "&") {
		equals := strings.IndexByte(part, '=')
		if equals < 0 {
			continue
		}
		rawKey, rawValue := part[:equals], part[equals+1:]
		key, err := url.QueryUnescape(rawKey)
		if err != nil || !strings.EqualFold(key, "token") || rawValue == "" {
			continue
		}
		seen[rawValue] = struct{}{}
		if value, decodeErr := url.QueryUnescape(rawValue); decodeErr == nil && value != "" {
			seen[value] = struct{}{}
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})
	return values
}

func redactKnownValues(message string, values []string) string {
	for _, value := range values {
		message = strings.ReplaceAll(message, value, redactedQueryValue)
	}
	return message
}

func normalizeLatency(latency time.Duration) time.Duration {
	switch {
	case latency > time.Minute:
		return latency.Truncate(10 * time.Second)
	case latency > time.Second:
		return latency.Truncate(10 * time.Millisecond)
	case latency > time.Millisecond:
		return latency.Truncate(10 * time.Microsecond)
	default:
		return latency
	}
}

func isBrokenConnection(recovered any) bool {
	err, ok := recovered.(error)
	return ok && (errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, http.ErrAbortHandler))
}
