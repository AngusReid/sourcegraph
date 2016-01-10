package mockstore

import (
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/store"
)

func (s *Builds) MockGet(t *testing.T, wantBuild sourcegraph.BuildSpec) (called *bool) {
	called = new(bool)
	s.Get_ = func(ctx context.Context, build sourcegraph.BuildSpec) (*sourcegraph.Build, error) {
		*called = true
		if build != wantBuild {
			t.Errorf("got build %q, want %q", build, wantBuild)
			return nil, grpc.Errorf(codes.NotFound, "build %s not found", build.IDString())
		}
		return &sourcegraph.Build{ID: build.ID, Repo: build.Repo.URI}, nil
	}
	return
}

func (s *Builds) MockGet_Return(t *testing.T, returns *sourcegraph.Build) (called *bool) {
	called = new(bool)
	s.Get_ = func(ctx context.Context, build sourcegraph.BuildSpec) (*sourcegraph.Build, error) {
		*called = true
		if build != returns.Spec() {
			t.Errorf("got build %q, want %q", build, returns.Spec())
			return nil, grpc.Errorf(codes.NotFound, "build %s not found", build.IDString())
		}
		return returns, nil
	}
	return
}

func (s *Builds) MockList(t *testing.T, wantBuilds ...sourcegraph.BuildSpec) (called *bool) {
	called = new(bool)
	s.List_ = func(ctx context.Context, opt *sourcegraph.BuildListOptions) ([]*sourcegraph.Build, error) {
		*called = true
		builds := make([]*sourcegraph.Build, len(wantBuilds))
		for i, build := range wantBuilds {
			builds[i] = &sourcegraph.Build{ID: build.ID, Repo: build.Repo.URI, CreatedAt: pbtypes.NewTimestamp(time.Unix(int64(len(wantBuilds)-1-i), 0))}
		}
		builds = store.SortAndPaginateBuilds(builds, opt)
		return builds, nil
	}
	return
}
