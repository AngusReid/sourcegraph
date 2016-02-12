package local

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"sourcegraph.com/sourcegraph/srclib/store/pb"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/server/accesscontrol"
	"src.sourcegraph.com/sourcegraph/store"
)

// NOTE: Only the importer part of the graph service is available over
// gRPC for now, because it is more necessary and has a simpler
// interface to convert to protobufs.

var Graph pb.MultiRepoImporterServer = &graph_{}

// The "_" differentiates it from package graph, which other files in
// this package import.
type graph_ struct{}

func (s *graph_) Import(ctx context.Context, op *pb.ImportOp) (*pbtypes.Void, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Graph.Import", op.Repo); err != nil {
		return nil, err
	}

	graphStore := store.GraphFromContextOrNil(ctx)
	if graphStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "graph importing")
	}
	return pb.Server(graphStore).Import(ctx, op)
}

func (s *graph_) Index(ctx context.Context, op *pb.IndexOp) (*pbtypes.Void, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Graph.Index", op.Repo); err != nil {
		return nil, err
	}

	graphStore := store.GraphFromContextOrNil(ctx)
	if graphStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "graph indexing")
	}
	return pb.Server(graphStore).Index(ctx, op)
}
