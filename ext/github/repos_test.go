package github

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/sourcegraph/go-github/github"
	"src.sourcegraph.com/sourcegraph/conf"
	"src.sourcegraph.com/sourcegraph/ext/github/githubcli"
	"src.sourcegraph.com/sourcegraph/store"
)

func isRepoNotFound(err error) bool {
	_, ok := err.(*store.RepoNotFoundError)
	return ok
}

// TestRepos_Get_existing tests the behavior of Repos.Get when called on a
// repo that exists (i.e., the successful outcome).
func TestRepos_Get_existing(t *testing.T) {
	githubcli.Config.GitHubHost = "github.com"
	ctx := testContext(&minimalClient{
		repos: mockGitHubRepos{
			Get_: func(owner, repo string) (*github.Repository, *github.Response, error) {
				return &github.Repository{
					ID:       github.Int(1),
					Name:     github.String("repo"),
					FullName: github.String("owner/repo"),
					Owner:    &github.User{ID: github.Int(1)},
					CloneURL: github.String("https://github.com/owner/repo.git"),
				}, nil, nil
			},
		},
	})

	s := &Repos{}
	existingRepo := "github.com/owner/repo"
	ctx = conf.WithURL(ctx, &url.URL{Scheme: "http", Host: "example.com"}, nil)

	repo, err := s.Get(ctx, existingRepo)
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Error("repo == nil")
	}
	if repo.URI != existingRepo {
		t.Errorf("got URI %q, want %q", repo.URI, existingRepo)
	}
}

// TestRepos_Get_nonexistent tests the behavior of Repos.Get when called
// on a repo that does not exist.
func TestRepos_Get_nonexistent(t *testing.T) {
	githubcli.Config.GitHubHost = "github.com"
	ctx := testContext(&minimalClient{
		repos: mockGitHubRepos{
			Get_: func(owner, repo string) (*github.Repository, *github.Response, error) {
				resp := &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(bytes.NewReader(nil))}
				return nil, &github.Response{Response: resp}, github.CheckResponse(resp)
			},
		},
	})

	s := &Repos{}
	nonexistentRepo := "github.com/owner/repo"
	repo, err := s.Get(ctx, nonexistentRepo)
	if !isRepoNotFound(err) {
		t.Fatal(err)
	}
	if repo != nil {
		t.Error("repo != nil")
	}
}
