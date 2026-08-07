package models

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const larkTestProviderToken = "lark-provider-token-must-stay-secret"

func TestGetUserLarkOAuth2HonorsRequestCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	ginContext := larkOAuthTestContext(requestContext)
	login := &UserLogin{Password: larkTestProviderToken}
	_, err := login.getUserLarkOAuth2(ginContext, &http.Client{Timeout: time.Second}, server.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("getUserLarkOAuth2() error = %v, want context cancellation", err)
	}
	assertLarkErrorDoesNotLeakToken(t, err)
}

func TestGetUserLarkOAuth2HasBoundedHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	login := &UserLogin{Password: larkTestProviderToken}
	started := time.Now()
	_, err := login.getUserLarkOAuth2(
		larkOAuthTestContext(context.Background()),
		&http.Client{Timeout: 25 * time.Millisecond},
		server.URL,
	)
	if err == nil {
		t.Fatal("getUserLarkOAuth2() accepted a timed-out provider request")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("provider timeout took %v, want bounded failure", elapsed)
	}
	assertLarkErrorDoesNotLeakToken(t, err)
}

func TestGetUserLarkOAuth2RejectsMalformedProviderPayloadWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "missing data", status: http.StatusOK, body: `{"code":0,"data":null}`},
		{name: "missing identity", status: http.StatusOK, body: `{"code":0,"data":{}}`},
		{name: "provider business error", status: http.StatusOK, body: `{"code":123,"msg":"provider detail"}`},
		{name: "http error", status: http.StatusBadGateway, body: `provider detail`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if authorization := r.Header.Get("Authorization"); authorization != "Bearer "+larkTestProviderToken {
					t.Errorf("Authorization = %q, want bearer provider token", authorization)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			login := &UserLogin{Password: larkTestProviderToken}
			_, err := login.getUserLarkOAuth2(
				larkOAuthTestContext(context.Background()),
				&http.Client{Timeout: time.Second},
				server.URL,
			)
			if err == nil {
				t.Fatal("getUserLarkOAuth2() accepted malformed provider response")
			}
			assertLarkErrorDoesNotLeakToken(t, err)
		})
	}
}

func TestGetUserLarkOAuth2RejectsClientWithoutTimeout(t *testing.T) {
	login := &UserLogin{Password: larkTestProviderToken}
	_, err := login.getUserLarkOAuth2(
		larkOAuthTestContext(context.Background()),
		&http.Client{},
		"https://open.larksuite.invalid/user_info",
	)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("getUserLarkOAuth2() error = %v, want timeout configuration rejection", err)
	}
}

func TestGetUserLarkOAuth2DoesNotLogTransportErrorDetail(t *testing.T) {
	const transportDetail = "transport-secret-detail"
	client := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New(transportDetail + ": " + larkTestProviderToken)
		}),
	}
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	login := &UserLogin{Password: larkTestProviderToken}
	_, err := login.getUserLarkOAuth2(
		larkOAuthTestContext(context.Background()),
		client,
		"https://open.larksuite.invalid/user_info",
	)
	if err == nil || !strings.Contains(err.Error(), transportDetail) {
		t.Fatalf("getUserLarkOAuth2() error = %v, want transport error", err)
	}
	if strings.Contains(logs.String(), larkTestProviderToken) || strings.Contains(logs.String(), transportDetail) {
		t.Fatalf("Lark identity log leaked untrusted detail: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "lark identity request failed") {
		t.Fatalf("Lark identity log did not contain generic diagnostic: %s", logs.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func larkOAuthTestContext(ctx context.Context) *gin.Context {
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/callback/lark", nil).WithContext(ctx)
	return ginContext
}

func assertLarkErrorDoesNotLeakToken(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), larkTestProviderToken) {
		t.Fatalf("provider error leaked token: %v", err)
	}
}
