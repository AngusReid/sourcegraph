package local

import (
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"
	"sourcegraph.com/sqs/pbtypes"
	authpkg "src.sourcegraph.com/sourcegraph/auth"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/server/accesscontrol"
	"src.sourcegraph.com/sourcegraph/store"
)

var Users sourcegraph.UsersServer = &users{}

type users struct{}

var _ sourcegraph.UsersServer = (*users)(nil)

func (s *users) Get(ctx context.Context, user *sourcegraph.UserSpec) (*sourcegraph.User, error) {
	store := store.UsersFromContextOrNil(ctx)
	if store == nil {
		return nil, &sourcegraph.NotImplementedError{What: "Users"}
	}

	shortCache(ctx)
	return store.Get(ctx, *user)
}

func (s *users) GetWithEmail(ctx context.Context, emailAddr *sourcegraph.EmailAddr) (*sourcegraph.User, error) {
	store := store.UsersFromContextOrNil(ctx)
	if store == nil {
		return nil, &sourcegraph.NotImplementedError{What: "Users"}
	}

	shortCache(ctx)
	return store.GetWithEmail(ctx, *emailAddr)
}

func (s *users) ListEmails(ctx context.Context, user *sourcegraph.UserSpec) (*sourcegraph.EmailAddrList, error) {
	store := store.UsersFromContextOrNil(ctx)
	if store == nil {
		log.Printf("Warning: users not implemented, returning empty list")
		return &sourcegraph.EmailAddrList{}, nil
	}
	if err := s.verifyCanReadEmail(ctx, *user); err != nil {
		return nil, err
	}

	emails, err := store.ListEmails(ctx, *user)
	if err != nil {
		return nil, err
	}
	shortCache(ctx)
	return &sourcegraph.EmailAddrList{EmailAddrs: emails}, nil
}

func (s *users) List(ctx context.Context, opt *sourcegraph.UsersListOptions) (*sourcegraph.UserList, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Users.List"); err != nil {
		return nil, err
	}

	store := store.UsersFromContextOrNil(ctx)
	if store == nil {
		log.Printf("Warning: users not implemented, returning empty list")
		return &sourcegraph.UserList{}, nil
	}

	users, err := store.List(ctx, opt)
	if err != nil {
		return nil, err
	}
	shortCache(ctx)
	return &sourcegraph.UserList{Users: users}, nil
}

func (s *users) Count(ctx context.Context, _ *pbtypes.Void) (*sourcegraph.UserCount, error) {
	store := store.UsersFromContextOrNil(ctx)
	if store == nil {
		log.Printf("Warning: users not implemented, returning zero")
		return &sourcegraph.UserCount{}, nil
	}

	count, err := store.Count(ctx)
	if err != nil {
		return nil, err
	}
	noCache(ctx)
	return &sourcegraph.UserCount{Count: count}, nil
}

func (s *users) verifyCanReadEmail(ctx context.Context, user sourcegraph.UserSpec) error {
	if authpkg.UserSpecFromContext(ctx).UID == user.UID {
		return nil
	}
	return grpc.Errorf(codes.PermissionDenied, "Can not view user email")
}
