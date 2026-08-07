package apis

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const templateWorkspaceDirectory = "temp/mss-generator"

var (
	githubOwnerPattern      = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

type githubRepository struct {
	Owner    string
	Name     string
	CloneURL string
	WebURL   string
}

// parseGitHubRepositoryURL limits provider credentials to GitHub itself. A
// caller-controlled host, port, userinfo, query, fragment, or extra path could
// otherwise receive the server-held provider token during clone or push.
func parseGitHubRepositoryURL(raw string) (githubRepository, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed == nil {
		return githubRepository{}, errors.New("invalid GitHub repository URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Hostname(), "github.com") ||
		parsed.Port() != "" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return githubRepository{}, errors.New("GitHub repository URL must use canonical HTTPS")
	}

	trimmedPath := strings.TrimPrefix(parsed.Path, "/")
	segments := strings.Split(trimmedPath, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return githubRepository{}, errors.New("GitHub repository URL must contain owner and repository")
	}
	owner := segments[0]
	repository := strings.TrimSuffix(segments[1], ".git")
	if !githubOwnerPattern.MatchString(owner) ||
		repository == "" || repository == "." || repository == ".." ||
		!githubRepositoryPattern.MatchString(repository) {
		return githubRepository{}, errors.New("GitHub repository owner or name is invalid")
	}

	webURL := (&url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/" + owner + "/" + repository,
	}).String()
	return githubRepository{
		Owner:    owner,
		Name:     repository,
		WebURL:   webURL,
		CloneURL: webURL + ".git",
	}, nil
}

// safeTemplateRelativePath normalizes a caller-provided repository-relative
// path and rejects values that can escape a checked-out workspace on either
// Unix or Windows.
func safeTemplateRelativePath(raw, emptyValue string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return emptyValue, nil
	}
	if strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") ||
		strings.Contains(value, ":") || strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return "", errors.New("repository path must be relative")
	}
	cleaned := path.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("repository path escapes its workspace")
	}
	for _, component := range strings.Split(cleaned, "/") {
		if strings.EqualFold(component, ".git") {
			return "", errors.New("repository metadata path is not allowed")
		}
	}
	return filepath.FromSlash(cleaned), nil
}

func newTemplateWorkspace() (string, error) {
	root, err := resolvedTemplateWorkspaceRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("create template workspace root: %w", err)
	}
	workspace, err := os.MkdirTemp(root, "request-")
	if err != nil {
		return "", fmt.Errorf("create template workspace: %w", err)
	}
	return workspace, nil
}

func resolvedTemplateWorkspaceRoot() (string, error) {
	root, err := filepath.Abs(templateWorkspaceDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve template workspace root: %w", err)
	}
	return filepath.Clean(root), nil
}

func removeTemplateWorkspace(workspace string) error {
	root, err := resolvedTemplateWorkspaceRoot()
	if err != nil {
		return err
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("resolve template workspace: %w", err)
	}
	relative, err := filepath.Rel(root, filepath.Clean(workspace))
	if err != nil || relative == "." || relative == ".." ||
		filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("refusing to remove a path outside the template workspace root")
	}
	if err := os.RemoveAll(workspace); err != nil {
		return fmt.Errorf("remove template workspace: %w", err)
	}
	return nil
}
