package local

import (
	"fmt"

	"golang.org/x/net/context"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/store"
)

var RepoStatuses sourcegraph.RepoStatusesServer = &repoStatuses{}

type repoStatuses struct{}

var _ sourcegraph.RepoStatusesServer = (*repoStatuses)(nil)

func (s *repoStatuses) GetCombined(ctx context.Context, repoRev *sourcegraph.RepoRevSpec) (*sourcegraph.CombinedStatus, error) {
	if repoRev == nil {
		return nil, fmt.Errorf("nil repo rev")
	}
	return store.RepoStatusesFromContext(ctx).GetCombined(ctx, *repoRev)
}

func (s *repoStatuses) Create(ctx context.Context, op *sourcegraph.RepoStatusesCreateOp) (*sourcegraph.RepoStatus, error) {
	repoRev := op.Repo
	status := &op.Status
	err := store.RepoStatusesFromContext(ctx).Create(ctx, repoRev, status)
	if err != nil {
		return nil, err
	}
	return status, nil
}
