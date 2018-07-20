package backend

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/db"
	"github.com/sourcegraph/sourcegraph/pkg/actor"
	"github.com/sourcegraph/sourcegraph/pkg/conf"
	"github.com/sourcegraph/sourcegraph/pkg/randstring"
)

func MakeRandomHardToGuessPassword() string {
	return randstring.NewLen(36)
}

func MakePasswordResetURL(ctx context.Context, userID int32) (*url.URL, error) {
	resetCode, err := db.Users.RenewPasswordResetCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("userID", strconv.Itoa(int(userID)))
	query.Set("code", resetCode)
	return &url.URL{Path: "/password-reset", RawQuery: query.Encode()}, nil
}

// CheckActorHasTag reports whether the context actor has the given tag. If not, or if an error
// occurs, a non-nil error is returned.
func CheckActorHasTag(ctx context.Context, tag string) error {
	actor := actor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return ErrNotAuthenticated
	}
	user, err := db.Users.GetByID(ctx, actor.UID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotAuthenticated
	}
	for _, t := range user.Tags {
		if t == tag {
			return nil
		}
	}
	return fmt.Errorf("actor lacks required tag %q", tag)
}

// PlatformTag is the user tag indicating that the user has the platform experiment enabled.
const PlatformTag = "platform"

// CheckActorHasPlatformEnabled reports whether the platform experiment is enabled for the context's actor.
func CheckActorHasPlatformEnabled(ctx context.Context) error {
	if conf.Platform() == nil {
		return errors.New("platform is disabled")
	}
	if !envvar.SourcegraphDotComMode() {
		return nil // enabled for all non-Sourcegraph.com users
	}
	return CheckActorHasTag(ctx, PlatformTag) // enabled for "platform"-tagged Sourcegraph.com users
}