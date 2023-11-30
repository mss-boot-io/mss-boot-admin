/*
 * @Author: lwnmengjing
 * @Date: 2021/12/16 9:07 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/12/16 9:07 下午
 */

package pkg

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
)

type GithubConfig struct {
	Name         string            `yaml:"name"`
	Organization string            `yaml:"organization"`
	Description  string            `yaml:"description"`
	Secrets      map[string]string `yaml:"secrets"`
	Token        string            `yaml:"token"`
}

// GitRemote from remote git
func GitRemote(url, directory string) error {
	r, err := git.PlainInit(directory, false)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	if err != nil {
		log.Println(err)
		return err
	}
	err = r.CreateBranch(&config.Branch{
		Name: "main",
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// GitClone clone git repo
func GitClone(url, branch, directory string, noCheckout bool, accessToken string) (*git.Repository, error) {
	auth := &http.BasicAuth{}
	if accessToken != "" {
		//fixme username not valid
		auth.Username = "lwnmengjing"
		auth.Password = accessToken
	}
	if PathExist(directory) {
		r, err := git.PlainOpen(directory)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		w, err := r.Worktree()
		if err != nil {
			log.Println(err)
			return nil, err
		}
		if branch != "" {
			err = w.Pull(&git.PullOptions{
				RemoteName:    "origin",
				Auth:          auth,
				ReferenceName: plumbing.NewBranchReferenceName(branch),
				Force:         true,
			})
		} else {
			err = w.Pull(&git.PullOptions{
				RemoteName: "origin",
				Auth:       auth,
				Force:      true,
			})
		}

		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return r, nil
		}
		log.Println(err)
		return r, err
	}
	if branch != "" {
		return git.PlainClone(directory, false, &git.CloneOptions{
			URL:               url,
			NoCheckout:        noCheckout,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			Auth:              auth,
			ReferenceName:     plumbing.NewBranchReferenceName(branch),
		})
	}
	return git.PlainClone(directory, false, &git.CloneOptions{
		URL:               url,
		NoCheckout:        noCheckout,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Auth:              auth,
	})
}

// GitCloneSSH clone git repo from ssh
func GitCloneSSH(url, directory, reference, privateKeyFile, password string) error {
	_, err := os.Stat(privateKeyFile)
	if err != nil {
		return fmt.Errorf("read file %s failed %s\n", privateKeyFile, err.Error())
	}
	publicKey, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, password)
	if err != nil {
		return fmt.Errorf("generate publickeys failed: %s\n", err.Error())
	}
	_, err = git.PlainClone(directory, false, &git.CloneOptions{
		Auth:          publicKey,
		URL:           url,
		Progress:      os.Stdout,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(reference),
	})
	if err != nil {
		return fmt.Errorf("clone repo error: %s", err.Error())
	}
	return nil
}

// CreateGithubRepo create github repo
func CreateGithubRepo(organization, name, description, token string, private bool) (*github.Repository, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	r := &github.Repository{Name: &name, Private: &private, Description: &description}
	repo, _, err := client.Repositories.Create(ctx, organization, r)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Printf("Successfully created new repo: %s\n", repo.GetName())
	return repo, nil
}

// GetGithubRepoAllBranches get all branches of github repo
func GetGithubRepoAllBranches(ctx context.Context, organization, name, token string) ([]*github.Branch, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	branches, _, err := client.Repositories.ListBranches(ctx, organization, name, &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	})
	return branches, err
}

// AddActionSecretsGithubRepo add action secret
//func AddActionSecretsGithubRepo(organization, name, token string, data map[string]string) error {
//	ctx := context.Background()
//	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
//	tc := oauth2.NewClient(ctx, ts)
//	client := github.NewClient(tc)
//	var err error
//	for k, v := range data {
//		input := github.EncryptedSecret{
//			Name: k,
//			EncryptedValue: v,
//		}
//		_, err = client.Actions.CreateOrUpdateRepoSecret(ctx, organization, name, &input)
//		if err != nil {
//			return err
//		}
//	}
//	return nil
//}

// CommitAndPushGithubRepo commit and push github repo
func CommitAndPushGithubRepo(directory, branch, path, accessToken string, auth *http.BasicAuth) error {
	r, err := git.PlainOpen(directory)
	if err != nil {
		log.Println(err)
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		log.Println(err)
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Create: true,
		Keep:   true,
	})
	if err != nil {
		log.Println(err)
		return err
	}
	if path == "" {
		path = "."
	}
	fmt.Println("path:", path)
	_, err = w.Add(path)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = w.Commit(":tada: generate "+path, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Email: auth.Username,
		},
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return r.Push(&git.PushOptions{
		Auth: auth,
	})
}
