package testing

import (
	"os"

	"src.sourcegraph.com/sourcegraph/pkg/vcs"
)

type MockRepository struct {
	GitRootDir_ func() string

	ResolveRevision_ func(spec string) (vcs.CommitID, error)

	Branches_ func(vcs.BranchesOptions) ([]*vcs.Branch, error)
	Tags_     func() ([]*vcs.Tag, error)

	GetCommit_ func(vcs.CommitID) (*vcs.Commit, error)
	Commits_   func(vcs.CommitsOptions) ([]*vcs.Commit, uint, error)

	BlameFile_ func(path string, opt *vcs.BlameOptions) ([]*vcs.Hunk, error)

	Lstat_    func(commit vcs.CommitID, name string) (os.FileInfo, error)
	Stat_     func(commit vcs.CommitID, name string) (os.FileInfo, error)
	ReadFile_ func(commit vcs.CommitID, name string) ([]byte, error)
	ReadDir_  func(commit vcs.CommitID, name string, recurse bool) ([]os.FileInfo, error)

	Diff_          func(base, head vcs.CommitID, opt *vcs.DiffOptions) (*vcs.Diff, error)
	CrossRepoDiff_ func(base vcs.CommitID, headRepo vcs.Repository, head vcs.CommitID, opt *vcs.DiffOptions) (*vcs.Diff, error)

	MergeBase_          func(a, b vcs.CommitID) (vcs.CommitID, error)
	CrossRepoMergeBase_ func(a vcs.CommitID, repoB vcs.Repository, b vcs.CommitID) (vcs.CommitID, error)

	Committers_ func(opt vcs.CommittersOptions) ([]*vcs.Committer, error)

	UpdateEverything_ func(vcs.RemoteOpts) (*vcs.UpdateResult, error)

	Search_ func(vcs.CommitID, vcs.SearchOptions) ([]*vcs.SearchResult, error)
}

var _ vcs.Repository = MockRepository{}

func (r MockRepository) GitRootDir() string {
	return r.GitRootDir_()
}

func (r MockRepository) ResolveRevision(spec string) (vcs.CommitID, error) {
	return r.ResolveRevision_(spec)
}

func (r MockRepository) Branches(opt vcs.BranchesOptions) ([]*vcs.Branch, error) {
	return r.Branches_(opt)
}

func (r MockRepository) Tags() ([]*vcs.Tag, error) {
	return r.Tags_()
}

func (r MockRepository) GetCommit(id vcs.CommitID) (*vcs.Commit, error) {
	return r.GetCommit_(id)
}

func (r MockRepository) Commits(opt vcs.CommitsOptions) ([]*vcs.Commit, uint, error) {
	return r.Commits_(opt)
}

func (r MockRepository) BlameFile(path string, opt *vcs.BlameOptions) ([]*vcs.Hunk, error) {
	return r.BlameFile_(path, opt)
}

func (r MockRepository) Lstat(commit vcs.CommitID, name string) (os.FileInfo, error) {
	return r.Lstat_(commit, name)
}

func (r MockRepository) Stat(commit vcs.CommitID, name string) (os.FileInfo, error) {
	return r.Stat_(commit, name)
}

func (r MockRepository) ReadFile(commit vcs.CommitID, name string) ([]byte, error) {
	return r.ReadFile_(commit, name)
}

func (r MockRepository) ReadDir(commit vcs.CommitID, name string, recurse bool) ([]os.FileInfo, error) {
	return r.ReadDir_(commit, name, recurse)
}

func (r MockRepository) Diff(base, head vcs.CommitID, opt *vcs.DiffOptions) (*vcs.Diff, error) {
	return r.Diff_(base, head, opt)
}

func (r MockRepository) CrossRepoDiff(base vcs.CommitID, headRepo vcs.Repository, head vcs.CommitID, opt *vcs.DiffOptions) (*vcs.Diff, error) {
	return r.CrossRepoDiff_(base, headRepo, head, opt)
}

func (r MockRepository) MergeBase(a, b vcs.CommitID) (vcs.CommitID, error) {
	return r.MergeBase_(a, b)
}

func (r MockRepository) CrossRepoMergeBase(a vcs.CommitID, repoB vcs.Repository, b vcs.CommitID) (vcs.CommitID, error) {
	return r.CrossRepoMergeBase_(a, repoB, b)
}

func (r MockRepository) Committers(opt vcs.CommittersOptions) ([]*vcs.Committer, error) {
	return r.Committers_(opt)
}

func (r MockRepository) UpdateEverything(opt vcs.RemoteOpts) (*vcs.UpdateResult, error) {
	return r.UpdateEverything_(opt)
}

func (r MockRepository) Search(commit vcs.CommitID, opt vcs.SearchOptions) ([]*vcs.SearchResult, error) {
	return r.Search_(commit, opt)
}
