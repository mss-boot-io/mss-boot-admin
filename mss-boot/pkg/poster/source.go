package poster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxImageResourceBytes int64 = 20 << 20

var imageHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many image source redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("unsupported image redirect scheme %q", req.URL.Scheme)
		}
		return nil
	},
}

// GetImage 从源读取图片，支持网络和本地。
func GetImage(src string) (image.Image, error) {
	r, err := getResourceReader(src)
	if err != nil {
		return nil, err
	}
	m, _, err := image.Decode(r)
	return m, err
}

// getResourceReader 读取图片，支持本地文件和经过证书校验的 HTTP(S) 资源。
func getResourceReader(src string) (*bytes.Reader, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, errors.New("图片源错误")
	}

	lowerSource := strings.ToLower(src)
	if strings.HasPrefix(lowerSource, "http://") || strings.HasPrefix(lowerSource, "https://") {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, src, nil)
		if err != nil {
			return nil, err
		}
		response, err := imageHTTPClient.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("image source returned HTTP status %s", response.Status)
		}
		fileBytes, err := readImageResource(response.Body)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(fileBytes), nil
	}

	file, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileBytes, err := readImageResource(file)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(fileBytes), nil
}

func readImageResource(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, maxImageResourceBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxImageResourceBytes {
		return nil, fmt.Errorf("image source exceeds %d bytes", maxImageResourceBytes)
	}
	return data, nil
}
