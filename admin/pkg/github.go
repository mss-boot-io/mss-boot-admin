package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/oauth2"
)

const (
	githubAPIBaseURL      = "https://api.github.com"
	githubResponseMaxSize = 1 << 20
)

type githubAPIBaseURLKey struct{}

// WithGithubAPIBaseURL overrides the GitHub REST API origin for deterministic
// tests and trusted integration environments. Production callers use GitHub's
// public API origin by default.
func WithGithubAPIBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, githubAPIBaseURLKey{}, strings.TrimRight(baseURL, "/"))
}

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/2 23:12:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/2 23:12:03
 */

type GithubUser struct {
	Login             string `json:"login"`
	ID                int64  `json:"id"`
	NodeID            string `json:"node_id"`
	AvatarURL         string `json:"avatar_url"`
	GravatarID        string `json:"gravatar_id"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
	Name              string `json:"name"`
	Company           string `json:"company"`
	Blog              string `json:"blog"`
	Location          string `json:"location"`
	Email             string `json:"email"`
	Hireable          bool   `json:"hireable"`
	Bio               string `json:"bio"`
	TwitterUsername   string `json:"twitter_username"`
	PublicRepos       int64  `json:"public_repos"`
	PublicGists       int64  `json:"public_gists"`
	Followers         int64  `json:"followers"`
	Following         int64  `json:"following"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	PrivateGists      int64  `json:"private_gists"`
	TotalPrivateRepos int64  `json:"total_private_repos"`
	OwnedPrivateRepos int64  `json:"owned_private_repos"`
	DiskUsage         int64  `json:"disk_usage"`
	Collaborators     int64  `json:"collaborators"`
	TwoFactorAuth     bool   `json:"two_factor_authentication"`
	Plan              struct {
		Name          string `json:"name"`
		Space         int64  `json:"space"`
		Collaborators int64  `json:"collaborators"`
		PrivateRepos  int64  `json:"private_repos"`
	} `json:"plan"`
}

type GithubOrganization struct {
	Login            string `json:"login"`
	ID               int64  `json:"id"`
	NodeID           string `json:"node_id"`
	URL              string `json:"url"`
	ReposURL         string `json:"repos_url"`
	EventsURL        string `json:"events_url"`
	HooksURL         string `json:"hooks_url"`
	IssuesURL        string `json:"issues_url"`
	MembersURL       string `json:"members_url"`
	PublicMembersURL string `json:"public_members_url"`
	AvatarURL        string `json:"avatar_url"`
	Description      string `json:"description"`
}

func GetUserFromGithub(ctx context.Context, conf *oauth2.Config, accessToken string) (*GithubUser, error) {
	var user GithubUser
	if err := getGithubResource(ctx, conf, accessToken, "/user", &user); err != nil {
		return nil, err
	}
	if !validGithubIdentity(&user) {
		return nil, errors.New("github user response has no valid identity")
	}
	user.Login = strings.TrimSpace(user.Login)
	return &user, nil
}

func GetOrganizationsFromGithub(ctx context.Context,
	conf *oauth2.Config,
	accessToken string) ([]string, error) {
	list := make([]*GithubOrganization, 0)
	if err := getGithubResource(ctx, conf, accessToken, "/user/orgs", &list); err != nil {
		return nil, err
	}
	org := make([]string, 0, len(list))
	for i := range list {
		if login := strings.TrimSpace(list[i].Login); login != "" {
			org = append(org, login)
		}
	}
	return org, nil
}

func getGithubResource(
	ctx context.Context,
	conf *oauth2.Config,
	accessToken string,
	path string,
	target any,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if conf == nil {
		return errors.New("github oauth configuration is missing")
	}
	if strings.TrimSpace(accessToken) == "" {
		return errors.New("github access token is missing")
	}
	endpoint, err := githubAPIURL(ctx, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create github API request: %w", err)
	}
	resp, err := conf.Client(ctx, &oauth2.Token{AccessToken: accessToken}).Do(req)
	if err != nil {
		return fmt.Errorf("github API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("github API request failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, githubResponseMaxSize+1))
	if err != nil {
		return errors.New("read github API response")
	}
	if len(body) > githubResponseMaxSize {
		return errors.New("github API response exceeds size limit")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errors.New("decode github API response")
	}
	return nil
}

func githubAPIURL(ctx context.Context, path string) (string, error) {
	baseURL := githubAPIBaseURL
	if configured, ok := ctx.Value(githubAPIBaseURLKey{}).(string); ok && configured != "" {
		baseURL = configured
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("github API base URL is invalid")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validGithubIdentity(user *GithubUser) bool {
	if user == nil || user.ID <= 0 {
		return false
	}
	login := strings.TrimSpace(user.Login)
	if login == "" || len(login) > 100 {
		return false
	}
	for _, r := range login {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
