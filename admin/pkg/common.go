package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var (
	latestReleaseURL = "https://api.github.com/repos/mss-boot-io/micro-service-gen-tool/releases/latest"
	httpClient       httpDoer = http.DefaultClient
)

// GetInstallPath returns the historical platform-specific installation path.
func GetInstallPath() string {
	if IsWindows() {
		return `C:\Program Files\nps`
	}
	return "/etc/nps"
}

// GetAppPath returns the absolute path to the running directory.
func GetAppPath() string {
	if path, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return path
	}
	return os.Args[0]
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func GetTmpPath() string {
	if IsWindows() {
		return GetAppPath()
	}
	return "/tmp"
}

type release struct {
	TagName string `json:"tag_name"`
}

// GetLatestVersion preserves the historical no-error API without terminating
// the embedding process. New code should call GetLatestVersionContext.
func GetLatestVersion() string {
	version, err := GetLatestVersionContext(context.Background())
	if err != nil {
		log.Printf("get latest generator version: %v", err)
		return ""
	}
	return version
}

func GetLatestVersionContext(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("create latest-release request: %w", err)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return "", fmt.Errorf("latest release returned HTTP %d", response.StatusCode)
	}
	var result release
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	if strings.TrimSpace(result.TagName) == "" {
		return "", errors.New("latest release response has no tag_name")
	}
	return result.TagName, nil
}

func copyStaticFile(srcPath, bin string) string {
	defer os.RemoveAll(srcPath)
	binPath, _ := filepath.Abs(os.Args[0])
	if !IsWindows() {
		if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin); err != nil {
			if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin); err != nil {
				log.Printf("copy %s: %v", bin, err)
				return binPath
			}
			_, _ = copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin+"-update")
			chMod("/usr/local/bin/"+bin+"-update", 0o755)
			binPath = "/usr/local/bin/" + bin
		} else {
			_, _ = copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin+"-update")
			chMod("/usr/bin/"+bin+"-update", 0o755)
			binPath = "/usr/bin/" + bin
		}
	} else {
		_, _ = copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(GetAppPath(), bin+"-update.exe"))
		_, _ = copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(GetAppPath(), bin+".exe"))
	}
	chMod(binPath, 0o755)
	return binPath
}

func CopyDir(srcPath, destPath string) error {
	sourceInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return errors.New("source path is not a directory")
	}
	if err := os.MkdirAll(destPath, sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	destinationInfo, err := os.Stat(destPath)
	if err != nil {
		return err
	}
	if !destinationInfo.IsDir() {
		return errors.New("destination path is not a directory")
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destPath, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := copyFile(path, target); err != nil {
			return fmt.Errorf("copy %s: %w", relative, err)
		}
		return os.Chmod(target, info.Mode().Perm())
	})
}

func copyFile(src, dest string) (int64, error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	destination, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return written, copyErr
	}
	return written, closeErr
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func chMod(name string, mode os.FileMode) {
	if !IsWindows() {
		_ = os.Chmod(name, mode)
	}
}

func InArray(vals []string, array []string, replace string, n int) bool {
	for _, value := range vals {
		for _, candidate := range array {
			candidate = strings.Replace(candidate, replace, "", n)
			if strings.EqualFold(candidate, value) {
				return true
			}
		}
	}
	return false
}

func Pluralize(word string) string {
	if word == "" {
		return ""
	}
	lower := strings.ToLower(word)
	lastLetter := lower[len(lower)-1:]
	beforeLastLetter := ""
	if len(lower) > 1 {
		beforeLastLetter = lower[len(lower)-2 : len(lower)-1]
	}
	switch lastLetter {
	case "y":
		if strings.Contains("aeiou", beforeLastLetter) {
			return word + "s"
		}
		return word[:len(word)-1] + "ies"
	case "x", "s", "z", "o":
		return word + "es"
	case "h":
		if beforeLastLetter == "s" || beforeLastLetter == "c" {
			return word + "es"
		}
		return word + "s"
	case "f":
		if beforeLastLetter == "f" && len(word) > 1 {
			return word[:len(word)-2] + "ves"
		}
		return word[:len(word)-1] + "ves"
	default:
		return word + "s"
	}
}
