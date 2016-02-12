package app

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sourcegraph/mux"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"gopkg.in/inconshreveable/log15.v2"
	"sourcegraph.com/sourcegraph/grpccache"
	"src.sourcegraph.com/sourcegraph/app/internal/authutil"
	"src.sourcegraph.com/sourcegraph/app/internal/tmpl"
	"src.sourcegraph.com/sourcegraph/app/router"
	"src.sourcegraph.com/sourcegraph/errcode"
	"src.sourcegraph.com/sourcegraph/ext"
	"src.sourcegraph.com/sourcegraph/ext/github"
	"src.sourcegraph.com/sourcegraph/ext/github/githubcli"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/repoupdater"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
	"src.sourcegraph.com/sourcegraph/util/httputil/httpctx"
	"src.sourcegraph.com/sourcegraph/util/router_util"
)

var (
	githubClientID string
)

func init() {
	githubClientID = os.Getenv("GITHUB_CLIENT_ID")
}

type userSettingsCommonData struct {
	User        *sourcegraph.User
	OrgsAndSelf []*sourcegraph.User
}

type privateRemoteRepo struct {
	ExistsLocally bool
	*sourcegraph.Repo
}

type gitHubIntegrationData struct {
	URL                string
	Host               string
	PrivateRemoteRepos []*privateRemoteRepo
	TokenIsPresent     bool
	TokenIsValid       bool
}

var errUserSettingsCommonWroteResponse = errors.New("userSettingsCommon already wrote an HTTP response")

// userSettingsCommon should be called at the beginning of each HTTP
// handler that generates or saves settings data. It checks auth and
// fetches common data.
//
// If this function returns the error
// errUserSettingsCommonWroteResponse, callers should return nil and
// stop handling the HTTP request. That means that this function
// already sent an HTTP response (such as a redirect). For example:
//
// 	userSpec, cd, err := userSettingsCommon(w, r)
// 	if err == errUserSettingsCommonWroteResponse {
// 		return nil
// 	} else if err != nil {
// 		return err
// 	}
func userSettingsCommon(w http.ResponseWriter, r *http.Request) (sourcegraph.UserSpec, *userSettingsCommonData, error) {
	apiclient := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)

	currentUser := handlerutil.UserFromRequest(r)
	if currentUser == nil {
		if err := authutil.RedirectToLogIn(w, r); err != nil {
			return sourcegraph.UserSpec{}, nil, err
		}
		return sourcegraph.UserSpec{}, nil, errUserSettingsCommonWroteResponse // tell caller to not continue handling this req
	}

	if err := userSettingsMeRedirect(w, r, currentUser); err == errUserSettingsCommonWroteResponse {
		return sourcegraph.UserSpec{}, nil, err
	}

	p, userSpec, err := getUser(grpccache.NoCache(ctx), r)
	if err != nil {
		return sourcegraph.UserSpec{}, nil, err
	}

	if currentUser.UID != userSpec.UID {
		return sourcegraph.UserSpec{}, nil, &errcode.HTTPErr{Status: http.StatusUnauthorized, Err: fmt.Errorf("must be logged in as the requested user")}
	}

	// The settings panel should have sections for the user AND for
	// each of the orgs that the user can admin. This list is
	// orgsAndSelf.
	orgs, err := apiclient.Orgs.List(ctx, &sourcegraph.OrgsListOp{Member: sourcegraph.UserSpec{UID: int32(currentUser.UID)}, ListOptions: sourcegraph.ListOptions{PerPage: 100}})
	if errcode.GRPC(err) == codes.Unimplemented {
		orgs = &sourcegraph.OrgList{} // ignore error
	} else if err != nil {
		return *userSpec, nil, err
	}

	orgsAndSelf := []*sourcegraph.User{p}
	for _, org := range orgs.Orgs {
		orgsAndSelf = append(orgsAndSelf, &org.User)
	}

	// The current user can only view their own profile, as well as
	// the profiles of orgs they are an admin for.
	currentUserCanAdminOrg := false
	for _, adminable := range orgsAndSelf {
		if p.UID == adminable.UID {
			currentUserCanAdminOrg = true
			break
		}
	}
	if !currentUserCanAdminOrg {
		return *userSpec, nil, &errcode.HTTPErr{
			Status: http.StatusForbidden,
			Err:    errors.New("only a user or an org admin can view/edit profile"),
		}
	}

	return *userSpec, &userSettingsCommonData{
		User:        p,
		OrgsAndSelf: orgsAndSelf,
	}, nil
}

// Redirects "/.me/.settings/*" to "/<u>/.settings/*". Returns errUserSettingsCommonWroteResponse if redirect should
// happen. Otherwise returns nil.
func userSettingsMeRedirect(w http.ResponseWriter, r *http.Request, u *sourcegraph.UserSpec) error {
	if u == nil {
		return nil
	}

	vars := mux.Vars(r)
	if userSpec_, err := sourcegraph.ParseUserSpec(vars["User"]); err == nil && userSpec_.Login == ".me" {
		varsCopy := make(map[string]string)
		for k, v := range vars {
			varsCopy[k] = vars[v]
		}
		varsCopy["User"] = u.Login

		redirectURL := router.Rel.URLTo(httpctx.RouteName(r), router_util.MapToArray(varsCopy)...)
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return errUserSettingsCommonWroteResponse
	}
	return nil
}

func userGitHubIntegrationData(ctx context.Context, apiclient *sourcegraph.Client) (*gitHubIntegrationData, error) {
	gd := &gitHubIntegrationData{
		URL:  githubcli.Config.URL(),
		Host: githubcli.Config.Host() + "/",
	}

	// Fetch the currently authenticated user's stored access token (if any).
	extToken, err := apiclient.Auth.GetExternalToken(ctx, &sourcegraph.ExternalTokenRequest{
		Host:     githubcli.Config.Host(),
		ClientID: githubClientID,
	})
	if err != nil {
		return nil, err
	}
	if extToken.Token == "" {
		return nil, errors.New("no valid token found for fetching GitHub repos")
	}

	ghRepos := &github.Repos{}
	// TODO(perf) Cache this response or perform the fetch after page load to avoid
	// having to wait for an http round trip to github.com.
	privateGitHubRepos, err := ghRepos.ListPrivate(ctx, extToken.Token)
	if err != nil {
		// If the error is caused by something other than the token not existing,
		// ensure the user knows there is a value set for the token but that
		// it is invalid.
		if _, ok := err.(ext.TokenNotFoundError); !ok {
			gd.TokenIsPresent = true
		}
		return gd, nil
	}
	gd.TokenIsPresent, gd.TokenIsValid = true, true

	existingRepos := make(map[string]struct{})
	privateRemoteRepos := make([]*privateRemoteRepo, len(privateGitHubRepos))

	repoOpts := &sourcegraph.RepoListOptions{
		ListOptions: sourcegraph.ListOptions{
			PerPage: 1000,
			Page:    1,
		},
	}
	for {
		repoList, err := apiclient.Repos.List(ctx, repoOpts)
		if err != nil {
			return nil, err
		}
		if len(repoList.Repos) == 0 {
			break
		}

		for _, repo := range repoList.Repos {
			existingRepos[repo.URI] = struct{}{}
		}

		repoOpts.ListOptions.Page += 1
	}

	// Check if a user's remote GitHub repo already exists locally under the
	// same URI. If so, mark it so it's clear that it can't be enabled.
	for i, repo := range privateGitHubRepos {
		if _, ok := existingRepos[repo.URI]; ok {
			privateRemoteRepos[i] = &privateRemoteRepo{ExistsLocally: true, Repo: repo}
		} else {
			privateRemoteRepos[i] = &privateRemoteRepo{ExistsLocally: false, Repo: repo}
		}
	}
	gd.PrivateRemoteRepos = privateRemoteRepos

	return gd, nil
}

func serveUserSettingsProfile(w http.ResponseWriter, r *http.Request) error {
	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	if r.Method == "POST" {
		user := cd.User
		user.Name = r.PostFormValue("Name")
		user.HomepageURL = r.PostFormValue("HomepageURL")
		user.Company = r.PostFormValue("Company")
		user.Location = r.PostFormValue("Location")
		if _, err := handlerutil.APIClient(r).Accounts.Update(httpctx.FromRequest(r), user); err != nil {
			return err
		}

		http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsProfile, "User", cd.User.Login).String(), http.StatusSeeOther)
		return nil
	}

	return tmpl.Exec(r, w, "user/settings/profile.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		tmpl.Common
	}{
		userSettingsCommonData: *cd,
	})
}

func serveUserSettingsProfileAvatar(w http.ResponseWriter, r *http.Request) error {
	apiclient := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)

	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	user := cd.User
	email := r.PostFormValue("GravatarEmail")
	user.AvatarURL = gravatarURL(email)

	_, err = apiclient.Accounts.Update(ctx, user)
	if err != nil {
		return err
	}

	http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsProfile, "User", user.Login).String(), http.StatusSeeOther)
	return nil
}

// gravatarURL returns the URL to the Gravatar avatar image for email.
// The generated URL can have a "&s=128"-like suffix appended to set the size.
// That allows it to be compatible with User.AvatarURLOfSize.
func gravatarURL(email string) string {
	email = strings.TrimSpace(email) // Trim leading and trailing whitespace from an email address.
	email = strings.ToLower(email)   // Force all characters to lower-case.
	h := md5.New()
	io.WriteString(h, email) // md5 hash the final string.
	return fmt.Sprintf("https://secure.gravatar.com/avatar/%x?d=mm", h.Sum(nil))
}

func serveUserSettingsEmails(w http.ResponseWriter, r *http.Request) error {
	apiclient := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)

	userSpec, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	if cd.User.IsOrganization {
		return &errcode.HTTPErr{
			Status: http.StatusNotFound,
			Err:    errors.New("only users have emails"),
		}
	}

	emails, err := apiclient.Users.ListEmails(ctx, &userSpec)
	if err != nil {
		if grpc.Code(err) == codes.PermissionDenied {
			// We are not allowed to view the emails, so just show
			// an empty list
			emails = &sourcegraph.EmailAddrList{EmailAddrs: []*sourcegraph.EmailAddr{}}
		} else {
			return err
		}
	}

	return tmpl.Exec(r, w, "user/settings/emails.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		EmailAddrs []*sourcegraph.EmailAddr
		tmpl.Common
	}{
		userSettingsCommonData: *cd,
		EmailAddrs:             emails.EmailAddrs,
	})
}

func serveUserSettingsIntegrations(w http.ResponseWriter, r *http.Request) error {
	apiclient := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)

	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	var gd *gitHubIntegrationData

	gd, err = userGitHubIntegrationData(ctx, apiclient)
	if err != nil {
		return err
	}

	return tmpl.Exec(r, w, "user/settings/integrations.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		GitHub *gitHubIntegrationData
		tmpl.Common
	}{
		userSettingsCommonData: *cd,
		GitHub:                 gd,
	})
}

func serveUserSettingsIntegrationsUpdate(w http.ResponseWriter, r *http.Request) error {
	apiclient := handlerutil.APIClient(r)
	ctx := httpctx.FromRequest(r)
	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	switch mux.Vars(r)["Integration"] {
	case "enable":
		r.ParseForm() // required if you don't call r.FormValue()
		repoURIs := r.Form["RepoURI[]"]

		for _, repoURI := range repoURIs {
			// Check repo doesn't already exist, skip if so.
			_, err = apiclient.Repos.Get(ctx, &sourcegraph.RepoSpec{URI: repoURI})
			if grpc.Code(err) != codes.NotFound {
				switch err {
				case nil:
					log15.Warn("repo", repoURI, "already exists")
					http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsIntegrations, "User", cd.User.Login).String(), http.StatusSeeOther)
					return nil
				default:
					return fmt.Errorf("problem getting repo %q: %v", repoURI, err)
				}
			}

			// Perform the following operations locally (non-federated) because it's a private repo.
			_, err = apiclient.Repos.Create(ctx, &sourcegraph.ReposCreateOp{
				URI:      repoURI,
				VCS:      "git",
				CloneURL: "https://" + repoURI + ".git",
				Mirror:   true,
				Private:  true,
			})
			if err != nil {
				return err
			}

			repoupdater.Enqueue(&sourcegraph.Repo{URI: repoURI})
		}
	}

	http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsIntegrations, "User", cd.User.Login).String(), http.StatusSeeOther)
	return nil
}

func serveUserSettingsKeys(w http.ResponseWriter, r *http.Request) error {
	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	return tmpl.Exec(r, w, "user/settings/keys.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		tmpl.Common
	}{
		userSettingsCommonData: *cd,
	})
}
