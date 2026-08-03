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
	"os"
	"path/filepath"

	"github.com/google/go-github/v88/github"
)

// GistClone clone gist repo
func GistClone(id, dir, accessToken string) error {
	client, err := newGitHubClient(accessToken)
	if err != nil {
		return err
	}
	return gistClone(context.Background(), client, id, dir)
}

func gistClone(ctx context.Context, client *github.Client, id, dir string) error {
	gist, _, err := client.Gists.Get(ctx, id)
	if err != nil {
		return err
	}

	if !PathExist(dir) {
		_ = PathCreate(dir)
	}

	// copy file to directory
	for _, f := range gist.Files {
		err = FileOpen(*bytes.NewBufferString(f.GetContent()), filepath.Join(dir, f.GetFilename()), os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
