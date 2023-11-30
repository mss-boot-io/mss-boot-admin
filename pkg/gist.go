/*
 * @Author: snakelu
 * @Date: 2023/02/24 9:55 上午
 * @Last Modified by: snakelu
 * @Last Modified time: 2023/02/24 9:55 上午
 */

package pkg

import (
	"bytes"
	"context"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/go-github/v41/github"
)

// GistClone clone gist repo
func GistClone(id, dir, accessToken string) error {
	ctx := context.Background()
	var tc *http.Client
	if accessToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	client := github.NewClient(tc)
	gist, _, err := client.Gists.Get(ctx, id)
	if err != nil {
		log.Fatal(err)
		return err
	}

	if !PathExist(dir) {
		PathCreate(dir)
	}

	// copy file to directory
	for _, f := range gist.Files {
		FileCreate(*bytes.NewBufferString(f.GetContent()), filepath.Join(dir, f.GetFilename()), os.ModePerm)
	}

	return nil
}
