package pkg

import (
	"time"

	"github.com/google/go-github/v88/github"
)

const githubClientTimeout = 30 * time.Second

func newGitHubClient(token string, options ...github.ClientOptionsFunc) (*github.Client, error) {
	clientOptions := []github.ClientOptionsFunc{
		github.WithTimeout(githubClientTimeout),
	}
	if token != "" {
		clientOptions = append(clientOptions, github.WithAuthToken(token))
	}
	clientOptions = append(clientOptions, options...)
	return github.NewClient(clientOptions...)
}
