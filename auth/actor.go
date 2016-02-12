package auth

import (
	"fmt"

	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
)

// Actor represents an agent that accesses resources. It can represent
// an anonymous user or a logged-in user.
type Actor struct {
	// TODO: Make UID an int32.
	UID int `json:",omitempty"`

	// Domain is the Sourcegraph server hostname that owns the user
	// account of the user that this actor represents (if any). A
	// blank Domain means that the user lives on the current server.
	Domain string `json:",omitempty"`

	// Login is the login of the currently authenticated user, if
	// any. It is provided as a convenience and is not guaranteed to
	// be correct (e.g., the user's login can change during the course
	// of a request if the user renames their account). It is also not
	// guaranteed to be populated (many request paths do not populate
	// it, as an optimization to avoid incurring the Users.Get call).
	Login string `json:",omitempty"`

	// ClientID is the client ID of the authenticated OAuth2 client
	// that initiated the original operation. It is NOT the client ID
	// of the server that owns the account for UID, or the current
	// server executing the operation.
	ClientID string `json:",omitempty"`

	// Scope is a set of authorized scopes that the actor has
	// access to on the given server.
	Scope map[string]bool `json:",omitempty"`

	// MirrorsNext is true if the actor corresponds to a user that has
	// access to the private mirrors feature.
	MirrorsNext bool `json:",omitempty"`

	// MirrorsWaitlist is true if the actor corresponds to a user that
	// is on the waitlist for access to the private mirrors feature.
	MirrorsWaitlist bool `json:",omitempty"`

	// RepoPerms holds the private repo permissions for this user on this
	// server. This field is set in the `server/accesscontrol` package.
	// The field type is private to that package to ensure that any other
	// part of the code base cannot accidentally modify the permissions
	// information for an actor.
	RepoPerms interface{} `json:",omitempty"`
}

func (a Actor) String() string {
	return fmt.Sprintf("Actor UID %d (domain=%v clientID=%v scope=%v)", a.UID, a.Domain, a.ClientID, a.Scope)
}

// IsAuthenticated returns true if the Actor is derived from an authenticated user.
func (a Actor) IsAuthenticated() bool {
	return a.UID != 0
}

// IsUser returns a boolean indicating whether this actor represents a
// user. When does an actor not represent a user? In two cases: (1) an
// unauthenticated actor; and (2) an actor that just has a ClientID
// (and UID 0) represents an authenticated client but not an
// authenticated user.
func (a Actor) IsUser() bool {
	return a.UID != 0
}

// HasScope returns a boolean indicating whether this actor has the
// given scope.
func (a Actor) HasScope(s string) bool {
	hasScope, ok := a.Scope[s]
	return ok && hasScope
}

// HasWriteAccess checks if the actor has "user:write" or "user:admin" scopes.
func (a Actor) HasWriteAccess() bool {
	return a.IsAuthenticated() && (a.HasScope("user:write") || a.HasScope("user:admin"))
}

// HasAdminAccess checks if the actor has "user:admin" scope.
func (a Actor) HasAdminAccess() bool {
	return a.IsAuthenticated() && (a.HasScope("user:admin"))
}

func UnmarshalScope(scope []string) map[string]bool {
	scopeMap := make(map[string]bool)
	for _, s := range scope {
		scopeMap[s] = true
	}
	return scopeMap
}

func MarshalScope(scopeMap map[string]bool) []string {
	scope := make([]string, 0)
	for s := range scopeMap {
		scope = append(scope, s)
	}
	return scope
}

func GetActorFromUser(user sourcegraph.User) Actor {
	scope := make(map[string]bool)
	if user.Write {
		scope["user:write"] = true
	}
	if user.Admin {
		scope["user:admin"] = true
	}
	return Actor{
		UID:    int(user.UID),
		Login:  user.Login,
		Domain: user.Domain,
		Scope:  scope,
	}
}
