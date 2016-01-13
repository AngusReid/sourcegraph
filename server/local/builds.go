package local

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/server/accesscontrol"
	"src.sourcegraph.com/sourcegraph/store"
	"src.sourcegraph.com/sourcegraph/svc"
	"src.sourcegraph.com/sourcegraph/util/eventsutil"
	"src.sourcegraph.com/sourcegraph/util/metricutil"
)

var Builds sourcegraph.BuildsServer = &builds{}

type builds struct{}

var _ sourcegraph.BuildsServer = (*builds)(nil)

func (s *builds) Get(ctx context.Context, build *sourcegraph.BuildSpec) (*sourcegraph.Build, error) {
	veryShortCache(ctx)
	return store.BuildsFromContext(ctx).Get(ctx, *build)
}

func (s *builds) List(ctx context.Context, opt *sourcegraph.BuildListOptions) (*sourcegraph.BuildList, error) {
	builds, err := store.BuildsFromContext(ctx).List(ctx, opt)
	if err != nil {
		return nil, err
	}

	// Find out if there are more pages.
	// StreamResponse.HasMore is set to true if next page has non-zero entries.
	// TODO(shurcooL): This can be optimized by structuring how pagination works a little better.
	var streamResponse sourcegraph.StreamResponse
	if opt != nil {
		moreOpt := *opt
		moreOpt.ListOptions.Page = int32(moreOpt.ListOptions.PageOrDefault()) + 1
		moreBuilds, err := store.BuildsFromContext(ctx).List(ctx, &moreOpt)
		if err != nil {
			return nil, err
		}
		streamResponse = sourcegraph.StreamResponse{HasMore: len(moreBuilds) > 0}
	}

	veryShortCache(ctx)
	return &sourcegraph.BuildList{Builds: builds, StreamResponse: streamResponse}, nil
}

func (s *builds) Create(ctx context.Context, op *sourcegraph.BuildsCreateOp) (*sourcegraph.Build, error) {
	defer noCache(ctx)

	if len(op.CommitID) != 40 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Builds.Create requires full commit ID")
	}

	repo, err := svc.Repos(ctx).Get(ctx, &op.Repo)
	if err != nil {
		return nil, err
	}

	if repo.Blocked {
		return nil, grpc.Errorf(codes.FailedPrecondition, "repo %s is blocked", repo.URI)
	}

	// Only an admin can re-enqueue a successful build
	if err = accesscontrol.VerifyUserHasAdminAccess(ctx, "Builds.Create"); err != nil {
		successful, err := s.List(ctx, &sourcegraph.BuildListOptions{
			Repo:      repo.URI,
			CommitID:  op.CommitID,
			Succeeded: true,
			ListOptions: sourcegraph.ListOptions{
				PerPage: 1,
			},
		})

		if err == nil && len(successful.Builds) > 0 {
			return successful.Builds[0], nil
		}
	}

	if op.Branch != "" && op.Tag != "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "at most one of Branch and Tag may be specified when creating a build (repo %s commit %q)", op.Repo.URI, op.CommitID)
	}

	b := &sourcegraph.Build{
		Repo:        repo.URI,
		CommitID:    op.CommitID,
		Branch:      op.Branch,
		Tag:         op.Tag,
		CreatedAt:   pbtypes.NewTimestamp(time.Now()),
		BuildConfig: op.Config,
	}

	b, err = store.BuildsFromContext(ctx).Create(ctx, b)
	if err != nil {
		return nil, err
	}

	if err := updateRepoStatusForBuild(ctx, b); err != nil {
		log.Printf("WARNING: failed to update repo status for new build #%s (repo %s): %s.", b.Spec().IDString(), b.Repo, err)
	}

	return b, nil
}

func (s *builds) Update(ctx context.Context, op *sourcegraph.BuildsUpdateOp) (*sourcegraph.Build, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Builds.Update"); err != nil {
		return nil, err
	}

	defer noCache(ctx)

	b, err := store.BuildsFromContext(ctx).Get(ctx, op.Build)
	if err != nil {
		return nil, err
	}

	info := op.Info
	var updateRepoStatus bool
	if info.StartedAt != nil {
		b.StartedAt = info.StartedAt
		updateRepoStatus = true
	}
	if info.EndedAt != nil {
		b.EndedAt = info.EndedAt
		updateRepoStatus = true
	}
	if info.HeartbeatAt != nil {
		b.HeartbeatAt = info.HeartbeatAt
	}
	if info.Host != "" {
		b.Host = info.Host
	}
	if info.Purged {
		b.Purged = info.Purged
	}
	if info.Success {
		b.Success = info.Success
		updateRepoStatus = true
	}
	if info.Failure {
		b.Failure = info.Failure
		updateRepoStatus = true
	}
	if info.Priority != 0 {
		b.Priority = info.Priority
	}
	if info.Killed {
		b.Killed = info.Killed
		updateRepoStatus = true
	}
	if info.BuilderConfig != "" {
		b.BuilderConfig = info.BuilderConfig
	}

	if err := store.BuildsFromContext(ctx).Update(ctx, b.Spec(), info); err != nil {
		return nil, err
	}

	if updateRepoStatus {
		if err := updateRepoStatusForBuild(ctx, b); err != nil {
			log.Printf("WARNING: failed to update repo status for modified build #%s (repo %s): %s.", b.Spec().IDString(), b.Repo, err)
		}
	}

	var Result string
	if b.Success {
		Result = "success"
	} else if b.Failure {
		Result = "failed"
	}
	if Result != "" {
		metricutil.LogEvent(ctx, &sourcegraph.UserEvent{
			Type:    "notif",
			Service: "Builds",
			Method:  "Update",
			Result:  Result,
		})
	}
	eventsutil.LogBuildRepo(ctx, Result)

	return b, nil
}

// updateRepoStatusForBuild updates the repo commit status for b's
// commit based on the status of b (and for the base repo of the
// cross-repo pull request that b was built for, if applicable). If b
// is not a build on a GitHub or Sourcegraph repo, no update is
// performed.
func updateRepoStatusForBuild(ctx context.Context, b *sourcegraph.Build) error {
	// TODO(nodb-deploy): implement this
	return nil
	// updateRepoStatus := func(repoRevSpec sourcegraph.RepoRevSpec, st sourcegraph.RepoStatus) error {
	// 	// Check if the repo is a GitHub-backed repo.
	// 	repo, err := svc.Repos(ctx).Get(ctx, &repoRevSpec.RepoSpec)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if !repo.IsGitHubRepo() {
	// 		return nil
	// 	}

	// 	// Check if the external statuses are enabled.
	// 	settings, err := svc.Repos(ctx).GetSettings(repoRevSpec.RepoSpec)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if settings.ExternalCommitStatuses == nil || *settings.ExternalCommitStatuses == false {
	// 		// Disabled.
	// 		return nil
	// 	}
	// 	if *st.State != "success" && (settings.UnsuccessfulExternalCommitStatuses == nil || *settings.UnsuccessfulExternalCommitStatuses == false) {
	// 		// Don't publish non-successful statuses.
	// 		return nil
	// 	}

	// 	// Assume the identity of a repo admin (with permission to
	// 	// create a repo status) because usually this func is called
	// 	// by an asynchronous build worker that is authenticated as
	// 	// superuser, not as any particular user. If we didn't do
	// 	// this, GitHub would forbid the create status request because
	// 	// it'd be coming from an anonymous user (from GitHub's POV).
	// 	if settings.LastAdminUID == nil {
	// 		log.Printf("Unable to update repo %s commit %s status for build #%d: no admin UID could be determined.", b.Repo, b.CommitID, b.BID)
	// 		return nil
	// 	}

	// 	// TODO(nodb-deploy): use a context to act as UID=*settings.LastAdminUID
	// 	if _, err := svc.Repos(ctx).CreateStatus(repoRevSpec, st); err == nil {
	// 		log.Printf("Updated repo %s commit %s status for build #%d (%s)", b.Repo, b.CommitID, b.BID, *st.State)
	// 	}
	// 	return err
	// }

	// repoRevSpec := sourcegraph.RepoRevSpec{
	// 	RepoSpec: sourcegraph.RepoSpec{URI: b.Repo},
	// 	Rev:      b.CommitID,
	// 	CommitID: b.CommitID,
	// }

	// // Reserve the "failure" state for if Sourcegraph ever runs actual
	// // tests. In general, users don't yet consider a Sourcegraph graph
	// // failure to be akin to a test failure. More like it should be
	// // pending until all open items on the code review are resolved.
	// var state, description string
	// if b.Failure {
	// 	state = "error"
	// 	description = "Sourcegraph build failed."
	// } else if b.Success {
	// 	state = "success"
	// 	description = "Sourcegraph build completed successfully."
	// } else {
	// 	if b.StartedAt.Valid {
	// 		description = "Sourcegraph build in progress..."
	// 	} else {
	// 		description = "Sourcegraph build queued..."
	// 	}
	// 	state = "pending"
	// }

	// st := sourcegraph.RepoStatus{RepoStatus: github.RepoStatus{
	// 	State:       github.String(state),
	// 	Description: github.String(description),

	// 	// The "/build" distinguishes it from a status on the merge
	// 	// commit that we will implement later. Here are all of the
	// 	// different kinds of planned statuses:
	// 	//
	// 	//  - sourcegraph/build: a build of a specific commit
	// 	//  - sourcegraph/review: the status of a code review (have all checklist items been resolved?)
	// 	Context: github.String("sourcegraph/build"),
	// }}
	// if state == "success" {
	// 	// Link directly to the repo if successful because that is
	// 	// more likely what people want. Only if it's a failure or in
	// 	// progress are they more likely to care about the build logs
	// 	// and details.
	// 	st.TargetURL = github.String(conf.AppURL(ctx).ResolveReference(router.Rel.URLToRepoCommit(b.Repo, b.CommitID)).String())
	// } else {
	// 	st.TargetURL = github.String(conf.AppURL(ctx).ResolveReference(router.Rel.URLToRepoBuild(b.Repo, b.BID)).String())
	// }

	// if err := updateRepoStatus(ctx, repoRevSpec, st); err != nil {
	// 	return err
	// }

	// return nil
}

func (s *builds) ListBuildTasks(ctx context.Context, op *sourcegraph.BuildsListBuildTasksOp) (*sourcegraph.BuildTaskList, error) {
	tasks, err := store.BuildsFromContext(ctx).ListBuildTasks(ctx, op.Build, op.Opt)
	if err != nil {
		return nil, err
	}
	return &sourcegraph.BuildTaskList{BuildTasks: tasks}, nil
}

func (s *builds) CreateTasks(ctx context.Context, op *sourcegraph.BuildsCreateTasksOp) (*sourcegraph.BuildTaskList, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Builds.CreateTasks"); err != nil {
		return nil, err
	}

	defer noCache(ctx)

	// Validate.
	buildSpec := op.Build
	tasks := op.Tasks
	for _, task := range tasks {
		if task.Build != (sourcegraph.BuildSpec{}) && task.Build != buildSpec {
			return nil, fmt.Errorf("task build (%s) does not match build (%s)", task.Build.IDString(), buildSpec.IDString())
		}
	}

	tasks2 := make([]*sourcegraph.BuildTask, len(tasks)) // copy to avoid mutating
	for i, taskPtr := range tasks {
		task := *taskPtr
		task.CreatedAt = pbtypes.NewTimestamp(time.Now())
		task.Build = buildSpec
		tasks2[i] = &task
	}

	created, err := store.BuildsFromContext(ctx).CreateTasks(ctx, tasks2)
	if err != nil {
		return nil, err
	}
	return &sourcegraph.BuildTaskList{BuildTasks: created}, nil
}

func (s *builds) UpdateTask(ctx context.Context, op *sourcegraph.BuildsUpdateTaskOp) (*sourcegraph.BuildTask, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Builds.UpdateTask"); err != nil {
		return nil, err
	}

	defer noCache(ctx)

	t, err := store.BuildsFromContext(ctx).GetTask(ctx, op.Task)
	if err != nil {
		return nil, err
	}

	info := op.Info
	if info.StartedAt != nil {
		t.StartedAt = info.StartedAt
	}
	if info.EndedAt != nil {
		t.EndedAt = info.EndedAt
	}
	if info.Success {
		t.Success = true
	}
	if info.Failure {
		t.Failure = true
	}

	if err := store.BuildsFromContext(ctx).UpdateTask(ctx, op.Task, info); err != nil {
		return nil, err
	}

	// If the task has finished, log it's result.
	if info.EndedAt != nil {
		eventsutil.LogFinishBuildTask(ctx, t.Label, t.Success, t.Failure)
	}

	return t, nil
}

// GetTaskLog gets the logs for a task.
//
// The build is fetched using the task's key (IDString) and its
// StartedAt/EndedAt fields are used to set the start/end times for
// the log entry search, which speeds up the operation significantly
// for the Papertrail backend.
func (s *builds) GetTaskLog(ctx context.Context, op *sourcegraph.BuildsGetTaskLogOp) (*sourcegraph.LogEntries, error) {
	task := op.Task
	opt := op.Opt

	if opt == nil {
		opt = &sourcegraph.BuildGetLogOptions{}
	}

	buildLogs := store.BuildLogsFromContextOrNil(ctx)
	if buildLogs == nil {
		return nil, grpc.Errorf(codes.Unimplemented, "BuildLogs")
	}

	var minID string
	var minTime, maxTime time.Time

	build, err := store.BuildsFromContext(ctx).Get(ctx, task.Build)
	if err != nil {
		return nil, err
	}

	if opt.MinID == "" {
		const timeBuffer = 120 * time.Second // in case clocks are off
		if build.StartedAt != nil {
			minTime = build.StartedAt.Time().Add(-1 * timeBuffer)
		}
		if build.EndedAt != nil {
			maxTime = build.EndedAt.Time().Add(timeBuffer)
		}
	} else {
		minID = opt.MinID
	}

	return buildLogs.Get(ctx, task, minID, minTime, maxTime)
}

func (s *builds) DequeueNext(ctx context.Context, op *sourcegraph.BuildsDequeueNextOp) (*sourcegraph.Build, error) {
	if err := accesscontrol.VerifyUserHasWriteAccess(ctx, "Builds.DequeueNext"); err != nil {
		return nil, err
	}

	nextBuild, err := store.BuildsFromContext(ctx).DequeueNext(ctx)
	if err != nil {
		return nil, err
	}
	if nextBuild == nil {
		return nil, grpc.Errorf(codes.NotFound, "build queue is empty")
	}
	return nextBuild, nil
}
