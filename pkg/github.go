package pkg

import (
	"context"
	"encoding/json"
	"log/slog"

	"golang.org/x/oauth2"
)

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

func GetUserFromGithub(ctx context.Context, conf *oauth2.Config, accessToken string) (*GithubUser, error) {
	client := conf.Client(ctx, &oauth2.Token{AccessToken: accessToken})
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		slog.Error("get user from github error", slog.Any("error", err))
		return nil, err
	}
	defer resp.Body.Close()
	var user GithubUser
	// unmarshal body contents as a type Candidate
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		slog.Error("decode user from github error", slog.Any("error", err))
		return nil, err
	}
	return &user, nil
}
