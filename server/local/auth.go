package local

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/app/router"
	authpkg "src.sourcegraph.com/sourcegraph/auth"
	"src.sourcegraph.com/sourcegraph/auth/accesstoken"
	"src.sourcegraph.com/sourcegraph/auth/authutil"
	"src.sourcegraph.com/sourcegraph/auth/idkey"
	"src.sourcegraph.com/sourcegraph/auth/ldap"
	"src.sourcegraph.com/sourcegraph/conf"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/pkg/oauth2util"
	"src.sourcegraph.com/sourcegraph/store"
	"src.sourcegraph.com/sourcegraph/svc"
	"src.sourcegraph.com/sourcegraph/util/randstring"
)

var (
	githubClientID string
)

func init() {
	githubClientID = os.Getenv("GITHUB_CLIENT_ID")
}

var Auth sourcegraph.AuthServer = &auth{}

type auth struct{}

var _ sourcegraph.AuthServer = (*auth)(nil)

func (s *auth) GetAuthorizationCode(ctx context.Context, op *sourcegraph.AuthorizationCodeRequest) (*sourcegraph.AuthorizationCode, error) {
	authStore := store.AuthorizationsFromContextOrNil(ctx)
	if authStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no Authorizations")
	}

	if op.ResponseType != "code" {
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid response_type")
	}

	client, err := (&registeredClients{}).Get(ctx, &sourcegraph.RegisteredClientSpec{ID: op.ClientID})
	if err != nil {
		return nil, err
	}

	// RedirectURI is OPTIONAL
	// (https://tools.ietf.org/html/rfc6749#section-4.1.1) but must be
	// validated if set.
	if op.RedirectURI != "" {
		if err := oauth2util.AllowRedirectURI(client.RedirectURIs, op.RedirectURI); err != nil {
			return nil, err
		}
	}

	code, err := authStore.CreateAuthCode(ctx, op, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return &sourcegraph.AuthorizationCode{Code: code, RedirectURI: op.RedirectURI}, nil
}

func (s *auth) GetAccessToken(ctx context.Context, op *sourcegraph.AccessTokenRequest) (*sourcegraph.AccessTokenResponse, error) {
	if authCode := op.GetAuthorizationCode(); authCode != nil {
		return s.exchangeCodeForAccessToken(ctx, authCode)
	} else if resOwnerPassword := op.GetResourceOwnerPassword(); resOwnerPassword != nil {
		return s.authenticateLogin(ctx, resOwnerPassword)
	} else if bearerJWT := op.GetBearerJWT(); bearerJWT != nil {
		return s.authenticateBearerJWT(ctx, bearerJWT)
	} else {
		return nil, grpc.Errorf(codes.Unauthenticated, "no supported auth credentials provided")
	}
}

func (s *auth) exchangeCodeForAccessToken(ctx context.Context, code *sourcegraph.AuthorizationCode) (*sourcegraph.AccessTokenResponse, error) {
	authStore := store.AuthorizationsFromContextOrNil(ctx)
	if authStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no Authorizations")
	}

	usersStore := store.UsersFromContextOrNil(ctx)
	if usersStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no Users")
	}

	clientID := authpkg.ActorFromContext(ctx).ClientID
	client, err := (&registeredClients{}).Get(ctx, &sourcegraph.RegisteredClientSpec{ID: clientID})
	if err != nil {
		return nil, err
	}

	// RedirectURI is REQUIRED if one was provided when the code was
	// created (https://tools.ietf.org/html/rfc6749#section-4.1.3).
	if code.RedirectURI != "" {
		if err := oauth2util.AllowRedirectURI(client.RedirectURIs, code.RedirectURI); err != nil {
			return nil, err
		}
	}

	req, err := authStore.MarkExchanged(ctx, code, authpkg.ActorFromContext(ctx).ClientID)
	if err != nil {
		return nil, err
	}

	user, err := usersStore.Get(ctx, sourcegraph.UserSpec{UID: req.UID})
	if err != nil {
		return nil, err
	}

	tok, err := accesstoken.New(idkey.FromContext(ctx), authpkg.Actor{
		UID:      int(user.UID),
		Login:    user.Login,
		ClientID: req.ClientID,
		Scope:    authpkg.UnmarshalScope(req.Scope),
	}, map[string]string{"GrantType": "AuthorizationCode"}, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return accessTokenToTokenResponse(tok), nil
}

func (s *auth) authenticateLogin(ctx context.Context, cred *sourcegraph.LoginCredentials) (*sourcegraph.AccessTokenResponse, error) {
	usersStore := store.UsersFromContextOrNil(ctx)
	if usersStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no Users")
	}

	user, err := usersStore.Get(ctx, sourcegraph.UserSpec{Login: cred.Login})
	if err != nil {
		if !(store.IsUserNotFound(err) && authutil.ActiveFlags.IsLDAP()) {
			return nil, err
		}
	}

	if authutil.ActiveFlags.IsLDAP() {
		ldapuser, err := ldap.VerifyLogin(cred.Login, cred.Password)
		if err != nil {
			return nil, grpc.Errorf(codes.PermissionDenied, "LDAP auth failed: %v", err)
		}

		if user == nil {
			user, err = linkLDAPUserAccount(ctx, ldapuser)
			if err != nil {
				return nil, err
			}
		}
	} else {
		passwordStore := store.PasswordFromContextOrNil(ctx)
		if passwordStore == nil {
			return nil, grpc.Errorf(codes.Unimplemented, "no Passwords")
		}

		if passwordStore.CheckUIDPassword(ctx, user.UID, cred.Password) != nil {
			return nil, grpc.Errorf(codes.PermissionDenied, "bad password for user %q", cred.Login)
		}
	}

	a := authpkg.ActorFromContext(ctx)
	if a.IsUser() {
		return nil, grpc.Errorf(codes.PermissionDenied, "refusing to issue access token from resource owner password to already authenticated user %d (only client, not user, must be authenticated)", a.UID)
	}

	a.UID = int(user.UID)
	a.Login = user.Login
	a.ClientID = idkey.FromContext(ctx).ID
	a.Scope = make(map[string]bool)
	if user.Write {
		a.Scope["user:write"] = true
	}
	if user.Admin {
		a.Scope["user:admin"] = true
	}

	tok, err := accesstoken.New(
		idkey.FromContext(ctx),
		a,
		map[string]string{"GrantType": "ResourceOwnerPassword"},
		7*24*time.Hour,
	)

	if err != nil {
		return nil, err
	}

	return accessTokenToTokenResponse(tok), nil
}

func (s *auth) authenticateBearerJWT(ctx context.Context, rawTok *sourcegraph.BearerJWT) (*sourcegraph.AccessTokenResponse, error) {
	var regClient *sourcegraph.RegisteredClient
	tok, err := jwt.Parse(rawTok.Assertion, func(tok *jwt.Token) (interface{}, error) {
		// The JWT's "iss" is the client's OAuth2 client ID.
		clientID, _ := tok.Claims["iss"].(string)
		if clientID == "" {
			return nil, errors.New("bearer JWT has empty issuer, can't look up key")
		}
		var err error
		regClient, err = (&registeredClients{}).Get(ctx, &sourcegraph.RegisteredClientSpec{ID: clientID})
		if err != nil {
			return nil, err
		}

		// Get the client's registered public key.
		if regClient.JWKS == "" {
			return nil, fmt.Errorf("client ID %s (identified by bearer JWT) has no JWKS", clientID)
		}
		pubKey, err := idkey.UnmarshalJWKSPublicKey([]byte(regClient.JWKS))
		if err != nil {
			return nil, fmt.Errorf("parsing client ID %s JWKS public key: %s", clientID, err)
		}
		return pubKey, nil
	})
	if err != nil {
		return nil, err
	}

	// Validate claims; see
	// https://tools.ietf.org/html/draft-ietf-oauth-jwt-bearer-12#section-3.
	aud, _ := tok.Claims["aud"].(string)
	tokURL := conf.AppURL(ctx).ResolveReference(router.Rel.URLTo(router.OAuth2ServerToken))
	if subtle.ConstantTimeCompare([]byte(aud), []byte(tokURL.String())) != 1 {
		return nil, grpc.Errorf(codes.PermissionDenied, "bearer JWT aud claim mismatch (JWT %q, server %q)", aud, tokURL)
	}

	atok, err := accesstoken.New(
		idkey.FromContext(ctx),
		authpkg.Actor{ClientID: regClient.ID},
		map[string]string{"GrantType": "BearerJWT"},
		time.Hour,
	)
	if err != nil {
		return nil, err
	}

	return accessTokenToTokenResponse(atok), nil
}

func accessTokenToTokenResponse(t *oauth2.Token) *sourcegraph.AccessTokenResponse {
	if t.AccessToken == "" {
		panic("empty AccessToken")
	}
	if t.TokenType == "" {
		panic("empty TokenType")
	}
	r := &sourcegraph.AccessTokenResponse{
		AccessToken: t.AccessToken,
		TokenType:   t.TokenType,
	}
	if !t.Expiry.IsZero() {
		sec := t.Expiry.Sub(time.Now()) / time.Second
		if sec > math.MaxInt32 {
			sec = math.MaxInt32
		}
		r.ExpiresInSec = int32(sec)
	}
	return r
}

func (s *auth) Identify(ctx context.Context, _ *pbtypes.Void) (*sourcegraph.AuthInfo, error) {
	// TODO(sqs): cache until the expiration of the token
	// shortCache(ctx)

	a := authpkg.ActorFromContext(ctx)
	return &sourcegraph.AuthInfo{
		ClientID: a.ClientID,
		UID:      int32(a.UID),
		Login:    a.Login,
		Domain:   a.Domain,

		Write:  a.HasWriteAccess(),
		Admin:  a.HasAdminAccess(),
		Scopes: authpkg.MarshalScope(a.Scope),

		MirrorsNext:     a.MirrorsNext,
		MirrorsWaitlist: a.MirrorsWaitlist,
	}, nil
}

func (s *auth) GetExternalToken(ctx context.Context, request *sourcegraph.ExternalTokenRequest) (*sourcegraph.ExternalToken, error) {
	extTokensStore := store.ExternalAuthTokensFromContextOrNil(ctx)
	if extTokensStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no External Tokens")
	}

	if request.ClientID == "" {
		request.ClientID = githubClientID
	}

	dbToken, err := extTokensStore.GetUserToken(ctx, int(request.UID), request.Host, request.ClientID)
	if err == authpkg.ErrNoExternalAuthToken {
		return nil, grpc.Errorf(codes.NotFound, "no external auth token found")
	} else if err != nil {
		return nil, err
	}

	shortCache(ctx)

	return &sourcegraph.ExternalToken{
		UID:      int32(dbToken.User),
		Host:     dbToken.Host,
		Token:    dbToken.Token,
		Scope:    dbToken.Scope,
		ClientID: dbToken.ClientID,
		ExtUID:   int32(dbToken.ExtUID),
	}, nil
}

func (s *auth) SetExternalToken(ctx context.Context, extToken *sourcegraph.ExternalToken) (*pbtypes.Void, error) {
	extTokensStore := store.ExternalAuthTokensFromContextOrNil(ctx)
	if extTokensStore == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "no External Tokens")
	}

	if extToken.ClientID == "" {
		extToken.ClientID = githubClientID
	}

	dbToken := &authpkg.ExternalAuthToken{
		User:     int(extToken.UID),
		Host:     extToken.Host,
		Token:    extToken.Token,
		Scope:    extToken.Scope,
		ClientID: extToken.ClientID,
		ExtUID:   int(extToken.ExtUID),
	}

	err := extTokensStore.SetUserToken(ctx, dbToken)
	return &pbtypes.Void{}, err
}

// linkLDAPUserAccount links the LDAP account with an account in the local users store.
func linkLDAPUserAccount(ctx context.Context, ldapuser *ldap.LDAPUser) (*sourcegraph.User, error) {
	if len(ldapuser.Emails) == 0 {
		return nil, grpc.Errorf(codes.FailedPrecondition, "LDAP accounts must have an associated email address to access Sourcegraph")
	}

	// Link the LDAP username with a user in the local accounts store.
	userSpec, err := svc.Accounts(ctx).Create(ctx, &sourcegraph.NewAccount{
		// Use the LDAP username.
		Login: ldapuser.Username,
		// Use the common email address as the primary email.
		Email: ldapuser.Emails[0],
		// Password in local store is irrelevant since auth will be done via LDAP.
		Password: randstring.NewLen(20),
	})
	return &sourcegraph.User{
		UID:    userSpec.UID,
		Login:  userSpec.Login,
		Domain: userSpec.Domain,
	}, err
}
