package errcode

import (
	"fmt"
	"net/http"
	"os"

	"strings"

	"src.sourcegraph.com/sourcegraph/auth"
	"src.sourcegraph.com/sourcegraph/fed/discover"
	"src.sourcegraph.com/sourcegraph/pkg/vcs"
	"src.sourcegraph.com/sourcegraph/store"

	"github.com/gorilla/schema"
	"github.com/sourcegraph/go-github/github"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// HTTP returns the most appropriate HTTP status code that describes
// err. It contains a hard-coded list of error types and error values
// (such as mapping store.RepoNotFoundError to NotFound) and
// heuristics (such as mapping os.IsNotExist-satisfying errors to
// NotFound). All other errors are mapped to HTTP 500 Internal Server
// Error.
func HTTP(err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch err {
	case vcs.ErrRevisionNotFound, vcs.ErrCommitNotFound:
		return http.StatusNotFound
	case auth.ErrNoExternalAuthToken:
		return http.StatusUnauthorized
	case store.ErrRepoNeedsCloneURL, store.ErrRepoNoCloneURL:
		return http.StatusPreconditionFailed
	case store.ErrRepoMirrorOnly:
		return http.StatusNotImplemented
	case store.ErrRegisteredClientIDExists:
		return http.StatusConflict
	}

	if strings.Contains(err.Error(), "git repository not found") {
		return http.StatusNotFound
	}

	switch e := err.(type) {
	case interface {
		HTTPStatusCode() int
	}:
		return e.HTTPStatusCode()
	case *github.ErrorResponse:
		return e.Response.StatusCode
	case schema.ConversionError:
		return http.StatusBadRequest
	case schema.MultiError:
		return http.StatusBadRequest
	case *store.RepoNotFoundError:
		return http.StatusNotFound
	case *store.UserNotFoundError:
		return http.StatusNotFound
	case *store.RegisteredClientNotFoundError:
		return http.StatusNotFound
	case *store.AccountAlreadyExistsError:
		return http.StatusConflict
	}

	if os.IsNotExist(err) {
		return http.StatusNotFound
	} else if os.IsNotExist(err) {
		return http.StatusNotFound
	} else if os.IsPermission(err) {
		return http.StatusForbidden
	} else if discover.IsNotFound(err) {
		return http.StatusNotFound
	}

	if code := grpc.Code(err); code != codes.Unknown {
		return grpcToHTTP(code)
	}

	return http.StatusInternalServerError
}

// GRPC returns the most appropriate gRPC error code that describes
// err.
func GRPC(err error) codes.Code {
	// Piggyback on the HTTP func to reduce code duplication.
	return httpToGRPC(HTTP(err))
}

type HTTPErr struct {
	Status int   // HTTP status code.
	Err    error // Optional reason for the HTTP error.
}

func (err *HTTPErr) Error() string {
	if err.Err != nil {
		return fmt.Sprintf("status %d, reason %s", err.Status, err.Err)
	}
	return fmt.Sprintf("Status %d", err.Status)
}

func (err *HTTPErr) HTTPStatusCode() int { return err.Status }

func IsHTTPErrorCode(err error, statusCode int) bool {
	return HTTP(err) == statusCode
}
