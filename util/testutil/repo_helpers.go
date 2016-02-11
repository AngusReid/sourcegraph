package testutil

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"src.sourcegraph.com/sourcegraph/auth/authutil"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/pkg/vcs"
	storecli "src.sourcegraph.com/sourcegraph/store/cli"
	"src.sourcegraph.com/sourcegraph/util/executil"
)

func EnsureRepoExists(t *testing.T, ctx context.Context, repoURI string) {
	cl, _ := sourcegraph.NewClientFromContext(ctx)

	repo, err := cl.Repos.Get(ctx, &sourcegraph.RepoSpec{URI: repoURI})
	if err != nil {
		t.Fatalf("repo %s does not exist: %s", repoURI, err)
	}

	// Make sure the repo has been cloned to vcsstore.
	repoRevSpec := sourcegraph.RepoRevSpec{RepoSpec: sourcegraph.RepoSpec{URI: repoURI}, Rev: repo.DefaultBranch}
	getCommitWithRefreshAndRetry(t, ctx, repoRevSpec)
}

// getCommitWithRefreshAndRetry tries to get a repository commit. If
// it doesn't exist, it triggers a refresh of the repo's VCS data and
// then retries (until maxGetCommitVCSRefreshWait has elapsed).
func getCommitWithRefreshAndRetry(t *testing.T, ctx context.Context, repoRevSpec sourcegraph.RepoRevSpec) *vcs.Commit {
	cl, _ := sourcegraph.NewClientFromContext(ctx)

	wait := time.Second * 9 * ciFactor

	timeout := time.After(wait)
	done := make(chan struct{})
	var commit *vcs.Commit
	var err error
	go func() {
		refreshTriggered := false
		for {
			commit, err = cl.Repos.GetCommit(ctx, &repoRevSpec)

			// Keep retrying if it's a NotFound, but stop trying if we succeeded, or if it's some other
			// error.
			if err == nil || grpc.Code(err) != codes.NotFound {
				break
			}

			if !refreshTriggered {
				if _, err = cl.MirrorRepos.RefreshVCS(ctx, &sourcegraph.MirrorReposRefreshVCSOp{Repo: repoRevSpec.RepoSpec}); err != nil {
					err = fmt.Errorf("failed to trigger VCS refresh for repo %s: %s", repoRevSpec.URI, err)
					break
				}
				t.Logf("repo %s revision %s not on remote; triggered refresh of VCS data, waiting %s", repoRevSpec.URI, repoRevSpec.Rev, wait)
				refreshTriggered = true
			}
			time.Sleep(time.Second)
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
		if err != nil {
			t.Fatal(err)
		}
		return commit
	case <-timeout:
		t.Fatalf("repo %s revision %s not found on remote, even after triggering a VCS refresh and waiting %s (vcsstore should not have taken so long)", repoRevSpec.URI, repoRevSpec.Rev, wait)
		panic("unreachable")
	}
}

// CreateRepo creates a new repo. Callers must call the returned
// done() func when done (if err is non-nil) to free up resources.
func CreateRepo(t *testing.T, ctx context.Context, repoURI string) (repo *sourcegraph.Repo, done func(), err error) {
	cl, _ := sourcegraph.NewClientFromContext(ctx)

	op := &sourcegraph.ReposCreateOp{
		URI: repoURI,
		VCS: "git",
	}

	if storecli.ActiveFlags.Store == "pgsql" {
		s := httptest.NewServer(trivialGitRepoHandler)
		op.CloneURL, done = s.URL, s.Close
		op.Mirror = true
	}
	if done == nil {
		done = func() {} // no-op
	}

	repo, err = cl.Repos.Create(ctx, op)
	if err != nil {
		done()
		return nil, done, err
	}
	t.Logf("created repo %q (VCS %q, clone URL %q)", repo.URI, repo.VCS, repo.CloneURL())

	return repo, done, nil
}

// CreateAndPushRepo is short-handed for:
//
//  CreateAndPushRepoFiles(t, ctx, repoURI, nil)
//
func CreateAndPushRepo(t *testing.T, ctx context.Context, repoURI string) (commitID string, done func(), err error) {
	return CreateAndPushRepoFiles(t, ctx, repoURI, nil)
}

// CreateAndPushRepoFiles creates and pushes sample commits to a repo. Callers
// must call the returned done() func when done (if err is non-nil) to free up
// resources.
func CreateAndPushRepoFiles(t *testing.T, ctx context.Context, repoURI string, files map[string]string) (commitID string, done func(), err error) {
	//var repo *sourcegraph.Repo
	repo, done, err := CreateRepo(t, ctx, repoURI)
	if err != nil {
		return "", nil, err
	}

	commitID, err = PushRepo(t, ctx, repo, files)
	if err != nil {
		return "", nil, err
	}
	return commitID, done, nil
}

// PushRepo pushes sample commits to a repo. If files is specified, it
// is treated as a map of filenames to file contents. If files is nil,
// a default set of some text files is used. All files are committed
// in the same commit.
func PushRepo(t *testing.T, ctx context.Context, repo *sourcegraph.Repo, files map[string]string) (commitID string, err error) {
	cl, _ := sourcegraph.NewClientFromContext(ctx)

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if repo.Mirror || repo.Origin != "" {
		t.Logf("warning: repo %q is not hosted by this server. CreateAndPushRepo should only be used when it's necessary that the server hosts the git repo and has commit data. If you just need to create a repo that can be fetched with, e.g., Repos.Get and Repos.List, call testutil.CreateRepo instead of CreateAndPushRepo.", repo.URI)
	}

	// Add auth to HTTP clone URL so that `git clone`, `git push`,
	// etc., commands are authenticated.
	u, err := url.Parse(repo.HTTPCloneURL)
	if err != nil {
		return "", err
	}
	var authedCloneURL string
	if u.User != nil {
		authedCloneURL = u.String()
	} else {
		authedCloneURL, err = authutil.AddSystemAuthToURL(ctx, "", repo.HTTPCloneURL)
		if err != nil {
			return "", err
		}
	}

	// Clone the repo locally.
	cmd := exec.Command("git", "clone", authedCloneURL)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("exec %q failed: %s\n%s", cmd.Args, err, out)
	}
	repoDir := filepath.Join(tmpDir, repo.Name)

	// Add files and make a commit.
	if files == nil {
		files = map[string]string{"myfile.txt": "a"}
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, path)), 0700); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(filepath.Join(repoDir, path), []byte(data), 0700); err != nil {
			return "", err
		}
		cmd = exec.Command("git", "add", path)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("exec %q failed: %s\n%s", cmd.Args, err, out)
		}
	}

	cmd = exec.Command("git", "commit", "-m", "hello", "--author", "a <a@a.com>", "--date", "2006-01-02T15:04:05Z")
	//cmd.Env = append(os.Environ(), "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@a.com", "GIT_COMMITTER_DATE=2006-01-02T15:04:05Z")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("exec %q failed: %s\n%s", cmd.Args, err, out)
	}

	// Push.
	cmd = exec.Command("git", "push", "origin", "master")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("exec %q failed: %s\n%s", cmd.Args, err, out)
	}

	commit, err := cl.Repos.GetCommit(ctx, &sourcegraph.RepoRevSpec{
		RepoSpec: sourcegraph.RepoSpec{URI: repo.URI},
		Rev:      "master",
	})
	if err != nil {
		return "", err
	}

	return string(commit.ID), nil
}

func CloneRepo(t *testing.T, cloneURL, dir string, args []string) error {
	return cloneRepo(t, cloneURL, dir, nil, args)
}

// CloneRepoSSH clones the repo over SSH, attempts to authenticate using the
// passed in RSA key.
func CloneRepoSSH(t *testing.T, cloneURL, dir string, key *rsa.PrivateKey, args []string) error {
	return cloneRepo(t, cloneURL, dir, key, args)
}

func cloneRepo(t *testing.T, cloneURL, dir string, key *rsa.PrivateKey, args []string) (err error) {
	if dir == "" {
		var err error
		dir, err = ioutil.TempDir("", "")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
	}
	cmd := exec.Command("git", "clone")
	cmd.Args = append(cmd.Args, args...)
	cmd.Args = append(cmd.Args, cloneURL)
	cmd.Env = append(os.Environ(), "GIT_ASKPASS=true") // disable password prompt
	cmd.Dir = dir
	if key != nil {
		// Attempting to clone over SSH.
		sshDir := filepath.Join(dir, ".ssh")
		if err := os.Mkdir(sshDir, 0700); err != nil {
			return err
		}

		idFile := filepath.Join(sshDir, "sshkey")

		// Write public key.
		sshPublicKey, err := ssh.NewPublicKey(&key.PublicKey)
		if err != nil {
			return err
		}
		publicKey := ssh.MarshalAuthorizedKey(sshPublicKey)
		t.Log("Key public data:\n", string(publicKey))
		if err := ioutil.WriteFile(idFile+".pub", publicKey, 0600); err != nil {
			return err
		}

		// Write private key.
		keyPrivatePEM := pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		t.Log("Key private PEM data:\n", string(keyPrivatePEM))
		if err := ioutil.WriteFile(idFile, keyPrivatePEM, 0600); err != nil {
			return err
		}

		// Generate the necessary SSH command.
		// NOTE: GIT_SSH_COMMAND requires git version 2.3+, so we must
		// use GIT_SSH.
		gitSSH := filepath.Join(sshDir, "gitssh")
		if err := ioutil.WriteFile(gitSSH, []byte(fmt.Sprintf("!#/bin/sh\nssh -i %s -vvvv -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \"$@\"\n", idFile)), 0500); err != nil {
			return err
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_SSH=%s", gitSSH))
	} else {
		// Attempting to clone over HTTP(s).
		ClearCachedCredentials(t, cloneURL)
		defer ClearCachedCredentials(t, cloneURL)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	defer func() {
		err = wrapExecErr(err, cmd, buf.String())
	}()

	// Bypass password prompts by sending in a password usually.
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := in.Write([]byte("\n")); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}

	if err := executil.CmdWaitWithTimeout(5*time.Second, cmd); err != nil {
		return err
	}
	return nil
}
