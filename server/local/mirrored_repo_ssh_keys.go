package local

import (
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/store"
)

var MirroredRepoSSHKeys sourcegraph.MirroredRepoSSHKeysServer = &mirroredRepoSSHKeys{}

type mirroredRepoSSHKeys struct{}

var _ sourcegraph.MirroredRepoSSHKeysServer = (*mirroredRepoSSHKeys)(nil)

func (s *mirroredRepoSSHKeys) Create(ctx context.Context, op *sourcegraph.MirroredRepoSSHKeysCreateOp) (*pbtypes.Void, error) {
	repo := op.Repo
	keyPEM := op.Key.PEM

	block, _ := pem.Decode(keyPEM)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	store := store.MirroredRepoSSHKeysFromContextOrNil(ctx)
	if store == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "MirroredRepoSSHKeys")
	}

	if err := store.Create(ctx, repo.URI, key); err != nil {
		return nil, err
	}
	return &pbtypes.Void{}, nil
}

func (s *mirroredRepoSSHKeys) Get(ctx context.Context, repo *sourcegraph.RepoSpec) (*sourcegraph.SSHPrivateKey, error) {
	store := store.MirroredRepoSSHKeysFromContextOrNil(ctx)
	if store == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "MirroredRepoSSHKeys")
	}

	keyPEM, err := store.GetPEM(ctx, repo.URI)
	if err != nil {
		return nil, err
	}
	if keyPEM == nil {
		return nil, grpc.Errorf(codes.NotFound, "no SSH key for repo %s", repo)
	}
	return &sourcegraph.SSHPrivateKey{PEM: keyPEM}, nil
}

func (s *mirroredRepoSSHKeys) Delete(ctx context.Context, repo *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	store := store.MirroredRepoSSHKeysFromContextOrNil(ctx)
	if store == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "MirroredRepoSSHKeys")
	}

	if err := store.Delete(ctx, repo.URI); err != nil {
		return nil, err
	}
	return &pbtypes.Void{}, nil
}
