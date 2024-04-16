package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/spf13/cast"
	"golang.org/x/oauth2"
	"os"
)

func main() {
	// Authenticate with GitHub using a personal access token
	token := os.Getenv("GITHUB_TOKEN")
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	output, err := os.ReadFile(os.Getenv("COVERAGE_FILE"))
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(-1)
	}
	content := string(output)

	content = `
| File | Coverage |
| ---- | -------- |
` + content

	// Print or save the Markdown table.
	fmt.Println(content)

	// Create a comment with the coverage table and submit it to the PR
	owner := os.Getenv("REPO_OWNER")   // Set by GitHub Actions
	repo := os.Getenv("REPO_NAME")     // Set by GitHub Actions
	prNumber := os.Getenv("PR_NUMBER") // Set by GitHub Actions

	comment := &github.IssueComment{
		Body: &content,
	}
	_, _, err = client.Issues.CreateComment(ctx, owner, repo, cast.ToInt(prNumber), comment)
	if err != nil {
		fmt.Println("Error creating comment:", err)
		return
	}

	fmt.Println("Comment submitted successfully!")
}
