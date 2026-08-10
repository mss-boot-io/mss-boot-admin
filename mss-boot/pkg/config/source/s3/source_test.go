package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
)

type s3SourceRoundTripper func(*http.Request) (*http.Response, error)

func (f s3SourceRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type s3SourceBody struct {
	reader io.Reader
	err    error
	closed atomic.Int32
}

func (b *s3SourceBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.reader.Read(p)
}

func (b *s3SourceBody) Close() error {
	b.closed.Add(1)
	return nil
}

func TestSourceClosesBodyOnReadSuccessAndFailure(t *testing.T) {
	t.Parallel()
	readFailure := errors.New("injected body read failure")
	tests := []struct {
		name    string
		body    *s3SourceBody
		wantErr error
	}{
		{
			name: "success",
			body: &s3SourceBody{reader: strings.NewReader("name: test")},
		},
		{
			name:    "read failure",
			body:    &s3SourceBody{reader: strings.NewReader(""), err: readFailure},
			wantErr: readFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newS3SourceTestClient(test.body)
			provider, err := New(
				source.WithContext(context.Background()),
				source.WithClient(client),
				source.WithBucket("config-bucket"),
				source.WithDir("config"),
			)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = provider.ReadFile("application")
			if test.wantErr == nil && err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("ReadFile() error = %v, want %v", err, test.wantErr)
			}
			if got := test.body.closed.Load(); got != 1 {
				t.Fatalf("response body close calls = %d, want 1", got)
			}
		})
	}
}

func TestSourceRequiresCallerOwnedContextAndClient(t *testing.T) {
	t.Parallel()
	client := newS3SourceTestClient(&s3SourceBody{reader: strings.NewReader("")})
	if _, err := New(source.WithClient(client)); err == nil {
		t.Fatal("New() without caller context succeeded")
	}
	if _, err := New(source.WithContext(context.Background())); err == nil {
		t.Fatal("New() without borrowed client succeeded")
	}
}

func TestSourceWatchReturnsUnsupported(t *testing.T) {
	t.Parallel()
	provider, err := New(
		source.WithContext(context.Background()),
		source.WithClient(newS3SourceTestClient(&s3SourceBody{reader: strings.NewReader("")})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := provider.Watch(nil, nil); !errors.Is(err, ErrWatchUnsupported) {
		t.Fatalf("Watch() error = %v, want ErrWatchUnsupported", err)
	}
}

func TestSourceContinuesAcrossMissingExtensions(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	httpClient := &http.Client{Transport: s3SourceRoundTripper(func(request *http.Request) (*http.Response, error) {
		attempt := requests.Add(1)
		if attempt == 1 {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`,
				)),
				Request: request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("name: fallback")),
			Request:    request,
		}, nil
	})}
	client := awss3.New(awss3.Options{
		Region:       "test-region-1",
		BaseEndpoint: aws.String("https://objects.example.test"),
		UsePathStyle: true,
		HTTPClient:   httpClient,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "access", SecretAccessKey: "secret"}, nil
		}),
	})
	provider, err := New(
		source.WithContext(context.Background()),
		source.WithClient(client),
		source.WithBucket("config-bucket"),
		source.WithDir("config"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	content, err := provider.ReadFile("application")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "name: fallback" || provider.GetExtend() != source.SchemeYml || requests.Load() != 2 {
		t.Fatalf("fallback content=%q extension=%q requests=%d", content, provider.GetExtend(), requests.Load())
	}
}

func newS3SourceTestClient(body io.ReadCloser) *awss3.Client {
	httpClient := &http.Client{Transport: s3SourceRoundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	})}
	return awss3.New(awss3.Options{
		Region:       "test-region-1",
		BaseEndpoint: aws.String("https://objects.example.test"),
		UsePathStyle: true,
		HTTPClient:   httpClient,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "access", SecretAccessKey: "secret"}, nil
		}),
	})
}
