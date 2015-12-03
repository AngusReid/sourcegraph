// GENERATED CODE - DO NOT EDIT!
//
// Generated by:
//
//   go run gen_remote.go
//
// Called via:
//
//   go generate
//

package remote

import (
	"golang.org/x/net/context"
	"sourcegraph.com/sourcegraph/go-vcs/vcs"
	"sourcegraph.com/sourcegraph/srclib/unit"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/svc"
)

// Services is a full set of remote services (implemented by calling a client to invoke each method on a remote server).
var Services = svc.Services{
	Accounts:            remoteAccounts{},
	Auth:                remoteAuth{},
	Builds:              remoteBuilds{},
	Changesets:          remoteChangesets{},
	Defs:                remoteDefs{},
	Deltas:              remoteDeltas{},
	GraphUplink:         remoteGraphUplink{},
	Markdown:            remoteMarkdown{},
	Meta:                remoteMeta{},
	MirrorRepos:         remoteMirrorRepos{},
	MirroredRepoSSHKeys: remoteMirroredRepoSSHKeys{},
	Notify:              remoteNotify{},
	Orgs:                remoteOrgs{},
	People:              remotePeople{},
	RegisteredClients:   remoteRegisteredClients{},
	RepoBadges:          remoteRepoBadges{},
	RepoStatuses:        remoteRepoStatuses{},
	RepoTree:            remoteRepoTree{},
	Repos:               remoteRepos{},
	Search:              remoteSearch{},
	Storage:             remoteStorage{},
	Units:               remoteUnits{},
	UserKeys:            remoteUserKeys{},
	Users:               remoteUsers{},
}

type remoteAccounts struct{ sourcegraph.AccountsServer }

func (s remoteAccounts) Create(ctx context.Context, v1 *sourcegraph.NewAccount) (*sourcegraph.UserSpec, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.Create(ctx, v1)
}

func (s remoteAccounts) RequestPasswordReset(ctx context.Context, v1 *sourcegraph.EmailAddr) (*sourcegraph.User, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.RequestPasswordReset(ctx, v1)
}

func (s remoteAccounts) ResetPassword(ctx context.Context, v1 *sourcegraph.NewPassword) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.ResetPassword(ctx, v1)
}

func (s remoteAccounts) Update(ctx context.Context, v1 *sourcegraph.User) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.Update(ctx, v1)
}

func (s remoteAccounts) Invite(ctx context.Context, v1 *sourcegraph.AccountInvite) (*sourcegraph.PendingInvite, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.Invite(ctx, v1)
}

func (s remoteAccounts) AcceptInvite(ctx context.Context, v1 *sourcegraph.AcceptedInvite) (*sourcegraph.UserSpec, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.AcceptInvite(ctx, v1)
}

func (s remoteAccounts) ListInvites(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.AccountInviteList, error) {
	return sourcegraph.NewClientFromContext(ctx).Accounts.ListInvites(ctx, v1)
}

type remoteAuth struct{ sourcegraph.AuthServer }

func (s remoteAuth) GetAuthorizationCode(ctx context.Context, v1 *sourcegraph.AuthorizationCodeRequest) (*sourcegraph.AuthorizationCode, error) {
	return sourcegraph.NewClientFromContext(ctx).Auth.GetAuthorizationCode(ctx, v1)
}

func (s remoteAuth) GetAccessToken(ctx context.Context, v1 *sourcegraph.AccessTokenRequest) (*sourcegraph.AccessTokenResponse, error) {
	return sourcegraph.NewClientFromContext(ctx).Auth.GetAccessToken(ctx, v1)
}

func (s remoteAuth) Identify(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.AuthInfo, error) {
	return sourcegraph.NewClientFromContext(ctx).Auth.Identify(ctx, v1)
}

func (s remoteAuth) GetPermissions(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.UserPermissions, error) {
	return sourcegraph.NewClientFromContext(ctx).Auth.GetPermissions(ctx, v1)
}

type remoteBuilds struct{ sourcegraph.BuildsServer }

func (s remoteBuilds) Get(ctx context.Context, v1 *sourcegraph.BuildSpec) (*sourcegraph.Build, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.Get(ctx, v1)
}

func (s remoteBuilds) GetRepoBuildInfo(ctx context.Context, v1 *sourcegraph.BuildsGetRepoBuildInfoOp) (*sourcegraph.RepoBuildInfo, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.GetRepoBuildInfo(ctx, v1)
}

func (s remoteBuilds) List(ctx context.Context, v1 *sourcegraph.BuildListOptions) (*sourcegraph.BuildList, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.List(ctx, v1)
}

func (s remoteBuilds) Create(ctx context.Context, v1 *sourcegraph.BuildsCreateOp) (*sourcegraph.Build, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.Create(ctx, v1)
}

func (s remoteBuilds) Update(ctx context.Context, v1 *sourcegraph.BuildsUpdateOp) (*sourcegraph.Build, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.Update(ctx, v1)
}

func (s remoteBuilds) ListBuildTasks(ctx context.Context, v1 *sourcegraph.BuildsListBuildTasksOp) (*sourcegraph.BuildTaskList, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.ListBuildTasks(ctx, v1)
}

func (s remoteBuilds) CreateTasks(ctx context.Context, v1 *sourcegraph.BuildsCreateTasksOp) (*sourcegraph.BuildTaskList, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.CreateTasks(ctx, v1)
}

func (s remoteBuilds) UpdateTask(ctx context.Context, v1 *sourcegraph.BuildsUpdateTaskOp) (*sourcegraph.BuildTask, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.UpdateTask(ctx, v1)
}

func (s remoteBuilds) GetLog(ctx context.Context, v1 *sourcegraph.BuildsGetLogOp) (*sourcegraph.LogEntries, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.GetLog(ctx, v1)
}

func (s remoteBuilds) GetTaskLog(ctx context.Context, v1 *sourcegraph.BuildsGetTaskLogOp) (*sourcegraph.LogEntries, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.GetTaskLog(ctx, v1)
}

func (s remoteBuilds) DequeueNext(ctx context.Context, v1 *sourcegraph.BuildsDequeueNextOp) (*sourcegraph.Build, error) {
	return sourcegraph.NewClientFromContext(ctx).Builds.DequeueNext(ctx, v1)
}

type remoteChangesets struct{ sourcegraph.ChangesetsServer }

func (s remoteChangesets) Create(ctx context.Context, v1 *sourcegraph.ChangesetCreateOp) (*sourcegraph.Changeset, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.Create(ctx, v1)
}

func (s remoteChangesets) Get(ctx context.Context, v1 *sourcegraph.ChangesetSpec) (*sourcegraph.Changeset, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.Get(ctx, v1)
}

func (s remoteChangesets) List(ctx context.Context, v1 *sourcegraph.ChangesetListOp) (*sourcegraph.ChangesetList, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.List(ctx, v1)
}

func (s remoteChangesets) Update(ctx context.Context, v1 *sourcegraph.ChangesetUpdateOp) (*sourcegraph.ChangesetEvent, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.Update(ctx, v1)
}

func (s remoteChangesets) Merge(ctx context.Context, v1 *sourcegraph.ChangesetMergeOp) (*sourcegraph.ChangesetEvent, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.Merge(ctx, v1)
}

func (s remoteChangesets) UpdateAffected(ctx context.Context, v1 *sourcegraph.ChangesetUpdateAffectedOp) (*sourcegraph.ChangesetEventList, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.UpdateAffected(ctx, v1)
}

func (s remoteChangesets) CreateReview(ctx context.Context, v1 *sourcegraph.ChangesetCreateReviewOp) (*sourcegraph.ChangesetReview, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.CreateReview(ctx, v1)
}

func (s remoteChangesets) ListReviews(ctx context.Context, v1 *sourcegraph.ChangesetListReviewsOp) (*sourcegraph.ChangesetReviewList, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.ListReviews(ctx, v1)
}

func (s remoteChangesets) ListEvents(ctx context.Context, v1 *sourcegraph.ChangesetSpec) (*sourcegraph.ChangesetEventList, error) {
	return sourcegraph.NewClientFromContext(ctx).Changesets.ListEvents(ctx, v1)
}

type remoteDefs struct{ sourcegraph.DefsServer }

func (s remoteDefs) Get(ctx context.Context, v1 *sourcegraph.DefsGetOp) (*sourcegraph.Def, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.Get(ctx, v1)
}

func (s remoteDefs) List(ctx context.Context, v1 *sourcegraph.DefListOptions) (*sourcegraph.DefList, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.List(ctx, v1)
}

func (s remoteDefs) ListRefs(ctx context.Context, v1 *sourcegraph.DefsListRefsOp) (*sourcegraph.RefList, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.ListRefs(ctx, v1)
}

func (s remoteDefs) ListExamples(ctx context.Context, v1 *sourcegraph.DefsListExamplesOp) (*sourcegraph.ExampleList, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.ListExamples(ctx, v1)
}

func (s remoteDefs) ListAuthors(ctx context.Context, v1 *sourcegraph.DefsListAuthorsOp) (*sourcegraph.DefAuthorList, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.ListAuthors(ctx, v1)
}

func (s remoteDefs) ListClients(ctx context.Context, v1 *sourcegraph.DefsListClientsOp) (*sourcegraph.DefClientList, error) {
	return sourcegraph.NewClientFromContext(ctx).Defs.ListClients(ctx, v1)
}

type remoteDeltas struct{ sourcegraph.DeltasServer }

func (s remoteDeltas) Get(ctx context.Context, v1 *sourcegraph.DeltaSpec) (*sourcegraph.Delta, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.Get(ctx, v1)
}

func (s remoteDeltas) ListUnits(ctx context.Context, v1 *sourcegraph.DeltasListUnitsOp) (*sourcegraph.UnitDeltaList, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.ListUnits(ctx, v1)
}

func (s remoteDeltas) ListDefs(ctx context.Context, v1 *sourcegraph.DeltasListDefsOp) (*sourcegraph.DeltaDefs, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.ListDefs(ctx, v1)
}

func (s remoteDeltas) ListFiles(ctx context.Context, v1 *sourcegraph.DeltasListFilesOp) (*sourcegraph.DeltaFiles, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.ListFiles(ctx, v1)
}

func (s remoteDeltas) ListAffectedAuthors(ctx context.Context, v1 *sourcegraph.DeltasListAffectedAuthorsOp) (*sourcegraph.DeltaAffectedPersonList, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.ListAffectedAuthors(ctx, v1)
}

func (s remoteDeltas) ListAffectedClients(ctx context.Context, v1 *sourcegraph.DeltasListAffectedClientsOp) (*sourcegraph.DeltaAffectedPersonList, error) {
	return sourcegraph.NewClientFromContext(ctx).Deltas.ListAffectedClients(ctx, v1)
}

type remoteGraphUplink struct{ sourcegraph.GraphUplinkServer }

func (s remoteGraphUplink) Push(ctx context.Context, v1 *sourcegraph.MetricsSnapshot) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).GraphUplink.Push(ctx, v1)
}

func (s remoteGraphUplink) PushEvents(ctx context.Context, v1 *sourcegraph.UserEventList) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).GraphUplink.PushEvents(ctx, v1)
}

type remoteMarkdown struct{ sourcegraph.MarkdownServer }

func (s remoteMarkdown) Render(ctx context.Context, v1 *sourcegraph.MarkdownRenderOp) (*sourcegraph.MarkdownData, error) {
	return sourcegraph.NewClientFromContext(ctx).Markdown.Render(ctx, v1)
}

type remoteMeta struct{ sourcegraph.MetaServer }

func (s remoteMeta) Status(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.ServerStatus, error) {
	return sourcegraph.NewClientFromContext(ctx).Meta.Status(ctx, v1)
}

func (s remoteMeta) Config(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.ServerConfig, error) {
	return sourcegraph.NewClientFromContext(ctx).Meta.Config(ctx, v1)
}

type remoteMirrorRepos struct{ sourcegraph.MirrorReposServer }

func (s remoteMirrorRepos) RefreshVCS(ctx context.Context, v1 *sourcegraph.MirrorReposRefreshVCSOp) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).MirrorRepos.RefreshVCS(ctx, v1)
}

type remoteMirroredRepoSSHKeys struct {
	sourcegraph.MirroredRepoSSHKeysServer
}

func (s remoteMirroredRepoSSHKeys) Create(ctx context.Context, v1 *sourcegraph.MirroredRepoSSHKeysCreateOp) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).MirroredRepoSSHKeys.Create(ctx, v1)
}

func (s remoteMirroredRepoSSHKeys) Get(ctx context.Context, v1 *sourcegraph.RepoSpec) (*sourcegraph.SSHPrivateKey, error) {
	return sourcegraph.NewClientFromContext(ctx).MirroredRepoSSHKeys.Get(ctx, v1)
}

func (s remoteMirroredRepoSSHKeys) Delete(ctx context.Context, v1 *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).MirroredRepoSSHKeys.Delete(ctx, v1)
}

type remoteNotify struct{ sourcegraph.NotifyServer }

func (s remoteNotify) GenericEvent(ctx context.Context, v1 *sourcegraph.NotifyGenericEvent) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Notify.GenericEvent(ctx, v1)
}

type remoteOrgs struct{ sourcegraph.OrgsServer }

func (s remoteOrgs) Get(ctx context.Context, v1 *sourcegraph.OrgSpec) (*sourcegraph.Org, error) {
	return sourcegraph.NewClientFromContext(ctx).Orgs.Get(ctx, v1)
}

func (s remoteOrgs) List(ctx context.Context, v1 *sourcegraph.OrgsListOp) (*sourcegraph.OrgList, error) {
	return sourcegraph.NewClientFromContext(ctx).Orgs.List(ctx, v1)
}

func (s remoteOrgs) ListMembers(ctx context.Context, v1 *sourcegraph.OrgsListMembersOp) (*sourcegraph.UserList, error) {
	return sourcegraph.NewClientFromContext(ctx).Orgs.ListMembers(ctx, v1)
}

type remotePeople struct{ sourcegraph.PeopleServer }

func (s remotePeople) Get(ctx context.Context, v1 *sourcegraph.PersonSpec) (*sourcegraph.Person, error) {
	return sourcegraph.NewClientFromContext(ctx).People.Get(ctx, v1)
}

type remoteRegisteredClients struct {
	sourcegraph.RegisteredClientsServer
}

func (s remoteRegisteredClients) Get(ctx context.Context, v1 *sourcegraph.RegisteredClientSpec) (*sourcegraph.RegisteredClient, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.Get(ctx, v1)
}

func (s remoteRegisteredClients) GetCurrent(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.RegisteredClient, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.GetCurrent(ctx, v1)
}

func (s remoteRegisteredClients) Create(ctx context.Context, v1 *sourcegraph.RegisteredClient) (*sourcegraph.RegisteredClient, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.Create(ctx, v1)
}

func (s remoteRegisteredClients) Update(ctx context.Context, v1 *sourcegraph.RegisteredClient) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.Update(ctx, v1)
}

func (s remoteRegisteredClients) Delete(ctx context.Context, v1 *sourcegraph.RegisteredClientSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.Delete(ctx, v1)
}

func (s remoteRegisteredClients) List(ctx context.Context, v1 *sourcegraph.RegisteredClientListOptions) (*sourcegraph.RegisteredClientList, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.List(ctx, v1)
}

func (s remoteRegisteredClients) GetUserPermissions(ctx context.Context, v1 *sourcegraph.UserPermissionsOptions) (*sourcegraph.UserPermissions, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.GetUserPermissions(ctx, v1)
}

func (s remoteRegisteredClients) SetUserPermissions(ctx context.Context, v1 *sourcegraph.UserPermissions) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.SetUserPermissions(ctx, v1)
}

func (s remoteRegisteredClients) ListUserPermissions(ctx context.Context, v1 *sourcegraph.RegisteredClientSpec) (*sourcegraph.UserPermissionsList, error) {
	return sourcegraph.NewClientFromContext(ctx).RegisteredClients.ListUserPermissions(ctx, v1)
}

type remoteRepoBadges struct{ sourcegraph.RepoBadgesServer }

func (s remoteRepoBadges) ListBadges(ctx context.Context, v1 *sourcegraph.RepoSpec) (*sourcegraph.BadgeList, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoBadges.ListBadges(ctx, v1)
}

func (s remoteRepoBadges) ListCounters(ctx context.Context, v1 *sourcegraph.RepoSpec) (*sourcegraph.CounterList, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoBadges.ListCounters(ctx, v1)
}

func (s remoteRepoBadges) RecordHit(ctx context.Context, v1 *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoBadges.RecordHit(ctx, v1)
}

func (s remoteRepoBadges) CountHits(ctx context.Context, v1 *sourcegraph.RepoBadgesCountHitsOp) (*sourcegraph.RepoBadgesCountHitsResult, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoBadges.CountHits(ctx, v1)
}

type remoteRepoStatuses struct{ sourcegraph.RepoStatusesServer }

func (s remoteRepoStatuses) GetCombined(ctx context.Context, v1 *sourcegraph.RepoRevSpec) (*sourcegraph.CombinedStatus, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoStatuses.GetCombined(ctx, v1)
}

func (s remoteRepoStatuses) Create(ctx context.Context, v1 *sourcegraph.RepoStatusesCreateOp) (*sourcegraph.RepoStatus, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoStatuses.Create(ctx, v1)
}

type remoteRepoTree struct{ sourcegraph.RepoTreeServer }

func (s remoteRepoTree) Get(ctx context.Context, v1 *sourcegraph.RepoTreeGetOp) (*sourcegraph.TreeEntry, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoTree.Get(ctx, v1)
}

func (s remoteRepoTree) Search(ctx context.Context, v1 *sourcegraph.RepoTreeSearchOp) (*sourcegraph.VCSSearchResultList, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoTree.Search(ctx, v1)
}

func (s remoteRepoTree) List(ctx context.Context, v1 *sourcegraph.RepoTreeListOp) (*sourcegraph.RepoTreeListResult, error) {
	return sourcegraph.NewClientFromContext(ctx).RepoTree.List(ctx, v1)
}

type remoteRepos struct{ sourcegraph.ReposServer }

func (s remoteRepos) Get(ctx context.Context, v1 *sourcegraph.RepoSpec) (*sourcegraph.Repo, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Get(ctx, v1)
}

func (s remoteRepos) List(ctx context.Context, v1 *sourcegraph.RepoListOptions) (*sourcegraph.RepoList, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.List(ctx, v1)
}

func (s remoteRepos) Create(ctx context.Context, v1 *sourcegraph.ReposCreateOp) (*sourcegraph.Repo, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Create(ctx, v1)
}

func (s remoteRepos) Update(ctx context.Context, v1 *sourcegraph.ReposUpdateOp) (*sourcegraph.Repo, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Update(ctx, v1)
}

func (s remoteRepos) Delete(ctx context.Context, v1 *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Delete(ctx, v1)
}

func (s remoteRepos) GetReadme(ctx context.Context, v1 *sourcegraph.RepoRevSpec) (*sourcegraph.Readme, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.GetReadme(ctx, v1)
}

func (s remoteRepos) Enable(ctx context.Context, v1 *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Enable(ctx, v1)
}

func (s remoteRepos) Disable(ctx context.Context, v1 *sourcegraph.RepoSpec) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.Disable(ctx, v1)
}

func (s remoteRepos) GetConfig(ctx context.Context, v1 *sourcegraph.RepoSpec) (*sourcegraph.RepoConfig, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.GetConfig(ctx, v1)
}

func (s remoteRepos) GetCommit(ctx context.Context, v1 *sourcegraph.RepoRevSpec) (*vcs.Commit, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.GetCommit(ctx, v1)
}

func (s remoteRepos) ListCommits(ctx context.Context, v1 *sourcegraph.ReposListCommitsOp) (*sourcegraph.CommitList, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.ListCommits(ctx, v1)
}

func (s remoteRepos) ListBranches(ctx context.Context, v1 *sourcegraph.ReposListBranchesOp) (*sourcegraph.BranchList, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.ListBranches(ctx, v1)
}

func (s remoteRepos) ListTags(ctx context.Context, v1 *sourcegraph.ReposListTagsOp) (*sourcegraph.TagList, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.ListTags(ctx, v1)
}

func (s remoteRepos) ListCommitters(ctx context.Context, v1 *sourcegraph.ReposListCommittersOp) (*sourcegraph.CommitterList, error) {
	return sourcegraph.NewClientFromContext(ctx).Repos.ListCommitters(ctx, v1)
}

type remoteSearch struct{ sourcegraph.SearchServer }

func (s remoteSearch) SearchTokens(ctx context.Context, v1 *sourcegraph.TokenSearchOptions) (*sourcegraph.DefList, error) {
	return sourcegraph.NewClientFromContext(ctx).Search.SearchTokens(ctx, v1)
}

func (s remoteSearch) SearchText(ctx context.Context, v1 *sourcegraph.TextSearchOptions) (*sourcegraph.VCSSearchResultList, error) {
	return sourcegraph.NewClientFromContext(ctx).Search.SearchText(ctx, v1)
}

type remoteStorage struct{ sourcegraph.StorageServer }

func (s remoteStorage) Get(ctx context.Context, v1 *sourcegraph.StorageKey) (*sourcegraph.StorageValue, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.Get(ctx, v1)
}

func (s remoteStorage) Put(ctx context.Context, v1 *sourcegraph.StoragePutOp) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.Put(ctx, v1)
}

func (s remoteStorage) PutNoOverwrite(ctx context.Context, v1 *sourcegraph.StoragePutOp) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.PutNoOverwrite(ctx, v1)
}

func (s remoteStorage) Delete(ctx context.Context, v1 *sourcegraph.StorageKey) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.Delete(ctx, v1)
}

func (s remoteStorage) Exists(ctx context.Context, v1 *sourcegraph.StorageKey) (*sourcegraph.StorageExists, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.Exists(ctx, v1)
}

func (s remoteStorage) List(ctx context.Context, v1 *sourcegraph.StorageKey) (*sourcegraph.StorageList, error) {
	return sourcegraph.NewClientFromContext(ctx).Storage.List(ctx, v1)
}

type remoteUnits struct{ sourcegraph.UnitsServer }

func (s remoteUnits) Get(ctx context.Context, v1 *sourcegraph.UnitSpec) (*unit.RepoSourceUnit, error) {
	return sourcegraph.NewClientFromContext(ctx).Units.Get(ctx, v1)
}

func (s remoteUnits) List(ctx context.Context, v1 *sourcegraph.UnitListOptions) (*sourcegraph.RepoSourceUnitList, error) {
	return sourcegraph.NewClientFromContext(ctx).Units.List(ctx, v1)
}

type remoteUserKeys struct{ sourcegraph.UserKeysServer }

func (s remoteUserKeys) AddKey(ctx context.Context, v1 *sourcegraph.SSHPublicKey) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).UserKeys.AddKey(ctx, v1)
}

func (s remoteUserKeys) LookupUser(ctx context.Context, v1 *sourcegraph.SSHPublicKey) (*sourcegraph.UserSpec, error) {
	return sourcegraph.NewClientFromContext(ctx).UserKeys.LookupUser(ctx, v1)
}

func (s remoteUserKeys) DeleteKey(ctx context.Context, v1 *pbtypes.Void) (*pbtypes.Void, error) {
	return sourcegraph.NewClientFromContext(ctx).UserKeys.DeleteKey(ctx, v1)
}

type remoteUsers struct{ sourcegraph.UsersServer }

func (s remoteUsers) Get(ctx context.Context, v1 *sourcegraph.UserSpec) (*sourcegraph.User, error) {
	return sourcegraph.NewClientFromContext(ctx).Users.Get(ctx, v1)
}

func (s remoteUsers) GetWithEmail(ctx context.Context, v1 *sourcegraph.EmailAddr) (*sourcegraph.User, error) {
	return sourcegraph.NewClientFromContext(ctx).Users.GetWithEmail(ctx, v1)
}

func (s remoteUsers) ListEmails(ctx context.Context, v1 *sourcegraph.UserSpec) (*sourcegraph.EmailAddrList, error) {
	return sourcegraph.NewClientFromContext(ctx).Users.ListEmails(ctx, v1)
}

func (s remoteUsers) List(ctx context.Context, v1 *sourcegraph.UsersListOptions) (*sourcegraph.UserList, error) {
	return sourcegraph.NewClientFromContext(ctx).Users.List(ctx, v1)
}

func (s remoteUsers) Count(ctx context.Context, v1 *pbtypes.Void) (*sourcegraph.UserCount, error) {
	return sourcegraph.NewClientFromContext(ctx).Users.Count(ctx, v1)
}
