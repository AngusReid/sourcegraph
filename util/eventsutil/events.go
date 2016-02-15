package eventsutil

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/mux"

	"golang.org/x/net/context"

	"gopkg.in/inconshreveable/log15.v2"
	"src.sourcegraph.com/sourcegraph/auth"
	"src.sourcegraph.com/sourcegraph/conf"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
	"src.sourcegraph.com/sourcegraph/util/httputil/httpctx"
)

// LogStartServer records a server startup event.
func LogStartServer() {
	clientID := sourcegraphClientID
	Log(&sourcegraph.Event{
		Type:     "StartServer",
		DeviceID: clientID,
		ClientID: clientID,
		EventProperties: map[string]string{
			"OS-Arch":  fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
			"ClientID": clientID,
		},
	})
}

// LogRegisterServer records that this client registered with the mothership.
func LogRegisterServer(clientName string) {
	clientID := sourcegraphClientID

	Log(&sourcegraph.Event{
		Type:     "RegisterServer",
		DeviceID: clientID,
		ClientID: clientID,
	})
}

// LogCreateAccount records that an account got created, possibly with
// an invite code.
func LogCreateAccount(ctx context.Context, newAcct *sourcegraph.NewAccount, admin, write, firstUser bool, inviteCode string) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, newAcct.Login)

	userProperties := map[string]string{
		"UID":         strconv.Itoa(int(newAcct.UID)),
		"UserID":      userID,
		"Email":       newAcct.Email,
		"ClientID":    clientID,
		"AccessLevel": getAccessLevel(admin, write),
	}

	if strings.Contains(newAcct.Email, "@") {
		userProperties["Domain"] = strings.SplitN(newAcct.Email, "@", 2)[1]
	}

	appURL := conf.AppURL(ctx)
	if appURL != nil {
		userProperties["AppURL"] = appURL.String()
	}

	firstUserStr := "False"
	if firstUser {
		firstUserStr = "True"
	}
	eventProperties := map[string]string{
		"FirstUser":  firstUserStr,
		"InviteCode": inviteCode,
	}

	Log(&sourcegraph.Event{
		Type:            "CreateAccount",
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		UserProperties:  userProperties,
		EventProperties: eventProperties,
	})
}

// LogSendInvite records that an invite link was created.
func LogSendInvite(ctx context.Context, email, inviteCode string, admin, write bool) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	eventProperties := map[string]string{
		"Invitee":     email,
		"InviteCode":  inviteCode,
		"AccessLevel": getAccessLevel(admin, write),
	}

	Log(&sourcegraph.Event{
		Type:            "SendInvite",
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func LogAddRepo(ctx context.Context, cloneURL, language string, mirror, private bool) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	source := "local"
	if mirror {
		if u, err := url.Parse(cloneURL); err != nil {
			source = "unknown"
		} else {
			source = u.Host
		}
	}

	visibility := "public"
	if private {
		visibility = "private"
	}

	eventProperties := map[string]string{
		"Source":     source,
		"Visibility": visibility,
		"Language":   language,
	}

	organization := returnOrganization(cloneURL)
	if organization != "" {
		eventProperties["Org"] = organization
	}

	Log(&sourcegraph.Event{
		Type:            "AddRepo",
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func LogBuildRepo(ctx context.Context, result string, build *sourcegraph.Build) {
	if result == "" {
		return
	}

	repoRevSpec := &sourcegraph.RepoRevSpec{
		RepoSpec: sourcegraph.RepoSpec{build.Repo},
		CommitID: build.CommitID,
	}
	cl, err := sourcegraph.NewClientFromContext(ctx)
	if err != nil {
		log15.Debug(err.Error())
		return
	}

	inventory, err := cl.Repos.GetInventory(ctx, repoRevSpec)
	if err != nil {
		log15.Debug(err.Error())
		return
	}

	var languages []string
	for _, v := range inventory.Languages {
		languages = append(languages, v.Name)
	}
	langs := strings.Join(languages, ",")

	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	eventProperties := map[string]string{
		"CodeIntelligence": result,
		"ProgramLanguages": langs,
	}

	organization := returnOrganization(build.Repo)
	if organization != "" {
		eventProperties["Org"] = organization
	}

	Log(&sourcegraph.Event{
		Type:            "BuildRepo",
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func LogFinishBuildTask(ctx context.Context, label string, success bool, failure bool) {
	var eventType string
	if strings.Contains(strings.ToLower(label), "(indexing)") {
		// Log srclib code intelligence task result.
		eventType = "FinishSrclibBuild"
	} else if strings.ToLower(label) == "build" {
		// Log CI (continuous integration) build task result.
		eventType = "FinishCIBuild"
	} else {
		// Don't log other task types.
		return
	}

	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	result := "N/A"
	if success {
		result = "success"
	} else if failure {
		result = "failed"
	}

	Log(&sourcegraph.Event{
		Type:     eventType,
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
		EventProperties: map[string]string{
			"Label":  label,
			"Result": result,
		},
	})

}

func LogBrowseCode(ctx context.Context, entryType string, tc *handlerutil.TreeEntryCommon, rc *handlerutil.RepoCommon) {
	clientID := sourcegraphClientID
	user := handlerutil.UserFromContext(ctx)
	userID, deviceID := getUserOrDeviceID(clientID, getUserLogin(user))
	userAgent := UserAgentFromContext(ctx)

	codeIntelligenceAvailable := "false"
	if tc != nil && tc.SrclibDataVersion != nil {
		codeIntelligenceAvailable = "true"
	}

	source := "local"
	if rc != nil && rc.Repo != nil && rc.Repo.Mirror {
		if u, err := url.Parse(rc.Repo.HTTPCloneURL); err != nil {
			source = "unknown"
		} else {
			source = u.Host
		}
	}

	eventProperties := map[string]string{
		"EntryType":        entryType,
		"CodeIntelligence": codeIntelligenceAvailable,
		"Source":           source,
	}

	if userAgent != "" {
		eventProperties["UserAgent"] = userAgent
	}

	Log(&sourcegraph.Event{
		Type:            "ViewRepoTree",
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func LogHTTPGitPush(ctx context.Context) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	Log(&sourcegraph.Event{
		Type:     "GitPush",
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
		EventProperties: map[string]string{
			"Protocol": "HTTP",
		},
	})
}

func LogSSHGitPush(clientID, login string) {
	userID, deviceID := getUserOrDeviceID(clientID, login)

	Log(&sourcegraph.Event{
		Type:     "GitPush",
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
		EventProperties: map[string]string{
			"Protocol": "SSH",
		},
	})
}

func LogSearchQuery(ctx context.Context, searchType string, numResults int32) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	Log(&sourcegraph.Event{
		Type:     searchType,
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
		EventProperties: map[string]string{
			"NumResults": strconv.Itoa(int(numResults)),
		},
	})
}

func LogViewDef(ctx context.Context, eventType string) {
	clientID := sourcegraphClientID
	user := handlerutil.UserFromContext(ctx)
	userID, deviceID := getUserOrDeviceID(clientID, getUserLogin(user))

	Log(&sourcegraph.Event{
		Type:     eventType,
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
	})
}

func LogCreateChangeset(ctx context.Context) {
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, auth.ActorFromContext(ctx).Login)

	Log(&sourcegraph.Event{
		Type:     "CreateChangeset",
		ClientID: clientID,
		UserID:   userID,
		DeviceID: deviceID,
	})
}

func LogPageView(ctx context.Context, user *sourcegraph.UserSpec, req *http.Request) {
	route := httpctx.RouteName(req)
	eventType := getPageViewEventType(route)
	if eventType == "" {
		return
	}

	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, getUserLogin(user))
	repoSpec, err := sourcegraph.UnmarshalRepoSpec(mux.Vars(req))
	var organization string
	if err != nil {
		organization = ""
	} else {
		organization = returnOrganization(repoSpec.URI)
	}
	userAgent := UserAgentFromContext(ctx)
	referer := req.Referer()

	var eventProperties map[string]string
	if organization != "" {
		eventProperties = make(map[string]string)
		eventProperties["Org"] = organization
	}

	if userAgent != "" {
		if organization == "" {
			eventProperties = make(map[string]string)
		}
		eventProperties["UserAgent"] = userAgent
	}

	if strings.Contains(referer, ".search") {
		var searchClick string

		if strings.Contains(referer, "type=token") {
			searchClick = "token"
		} else if strings.Contains(referer, "type=text") {
			searchClick = "text"
		}
		if eventProperties == nil && searchClick != "" {
			eventProperties = make(map[string]string)
			eventProperties["SearchClick"] = searchClick
		} else if searchClick != "" {
			eventProperties["SearchClick"] = searchClick
		}
	}

	Log(&sourcegraph.Event{
		Type:            eventType,
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func LogSignIn(ctx context.Context) {
	LogEvent(ctx, "UserSignIn")
}

func LogSignOut(ctx context.Context) {
	LogEvent(ctx, "UserSignOut")
}

func LogEvent(ctx context.Context, event string) {
	login := auth.ActorFromContext(ctx).Login
	clientID := sourcegraphClientID
	userID, deviceID := getUserOrDeviceID(clientID, login)
	userAgent := UserAgentFromContext(ctx)

	var eventProperties map[string]string
	if userAgent != "" {
		eventProperties = make(map[string]string)
		eventProperties["UserAgent"] = userAgent
	}

	Log(&sourcegraph.Event{
		Type:            event,
		ClientID:        clientID,
		UserID:          userID,
		DeviceID:        deviceID,
		EventProperties: eventProperties,
	})
}

func getAccessLevel(admin, write bool) string {
	if admin {
		return "Admin"
	} else if write {
		return "Write"
	}
	return "Read"
}

func getShortClientID(clientID string) string {
	shortLen := 6
	if len(clientID) < shortLen {
		shortLen = len(clientID)
	}
	return clientID[:shortLen]
}

func getUserOrDeviceID(clientID, login string) (string, string) {
	if login == "" {
		return "", clientID
	}
	shortClientID := getShortClientID(clientID)
	return fmt.Sprintf("%s@%s", login, shortClientID), ""
}

func getUserLogin(user *sourcegraph.UserSpec) string {
	if user != nil {
		return user.Login
	}
	return ""
}

func getPageViewEventType(route string) string {
	if route == "" {
		return ""
	}

	// Filter out routes that have their own top-level event
	// to avoid double logging the same user event.
	switch route {
	case "repo.tree":
		return ""
	}

	eventType := "View"
	chunks := strings.Split(route, ".")
	for i := range chunks {
		if len(chunks[i]) > 0 {
			token := []rune(chunks[i])
			token[0] = unicode.ToUpper(token[0])
			eventType = eventType + string(token)
		}
	}

	return eventType
}
