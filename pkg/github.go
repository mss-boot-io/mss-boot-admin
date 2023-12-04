package pkg

import (
	"context"
	"encoding/json"
	"golang.org/x/oauth2"
	"log/slog"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/2 23:12:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/2 23:12:03
 */

/*
{
    "login": "lwnmengjing",
    "id": 12806223,
    "node_id": "MDQ6VXNlcjEyODA2MjIz",
    "avatar_url": "https://avatars.githubusercontent.com/u/12806223?v=4",
    "gravatar_id": "",
    "url": "https://api.github.com/users/lwnmengjing",
    "html_url": "https://github.com/lwnmengjing",
    "followers_url": "https://api.github.com/users/lwnmengjing/followers",
    "following_url": "https://api.github.com/users/lwnmengjing/following{/other_user}",
    "gists_url": "https://api.github.com/users/lwnmengjing/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/lwnmengjing/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/lwnmengjing/subscriptions",
    "organizations_url": "https://api.github.com/users/lwnmengjing/orgs",
    "repos_url": "https://api.github.com/users/lwnmengjing/repos",
    "events_url": "https://api.github.com/users/lwnmengjing/events{/privacy}",
    "received_events_url": "https://api.github.com/users/lwnmengjing/received_events",
    "type": "User",
    "site_admin": false,
    "name": null,
    "company": "@mss-boot-io @go-admin-team @MatrixLabsTech @WhiteMatrixTech",
    "blog": "https://docs.mss-boot-io.top/",
    "location": "huaian",
    "email": "lwnmengjing@qq.com",
    "hireable": null,
    "bio": null,
    "twitter_username": null,
    "public_repos": 88,
    "public_gists": 0,
    "followers": 25,
    "following": 10,
    "created_at": "2015-06-09T01:38:59Z",
    "updated_at": "2023-12-03T10:44:53Z",
    "private_gists": 0,
    "total_private_repos": 41,
    "owned_private_repos": 41,
    "disk_usage": 758437,
    "collaborators": 1,
    "two_factor_authentication": true,
    "plan": {
        "name": "free",
        "space": 976562499,
        "collaborators": 0,
        "private_repos": 10000
    }
}
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
